package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ZTESnapshotter captures state used to verify or roll back a ZTE change.
type ZTESnapshotter interface {
	CaptureBeforeSnapshot(models.ONT) (*ConfigSnapshot, error)
	CaptureAfterSnapshot(models.ONT) (*ConfigSnapshot, error)
}

// ZTERollbacker restores a failed ZTE change.
type ZTERollbacker interface {
	RollbackToSnapshot(context.Context, models.ONT, *ConfigSnapshot) error
}

type zteOLTRollbacker interface {
	RollbackToSnapshotForOLT(context.Context, models.OLT, models.ONT, *ConfigSnapshot) error
}

// ZTEGPONRegisterService coordinates ZTE registration and Internet service jobs.
type ZTEGPONRegisterService struct {
	db               *gorm.DB
	jobs             *JobService
	snapshot         ZTESnapshotter
	commanderFactory CommanderFactory
	rollback         ZTERollbacker
	logger           *zap.Logger
	encryptionKey    []byte
}

// WithEncryptionKey lets the service seal a PPPoE password when it records the
// service a job applied. Without one the service is still stored, minus the
// password, which the next running-config read supplies.
func (s *ZTEGPONRegisterService) WithEncryptionKey(key string) *ZTEGPONRegisterService {
	s.encryptionKey = []byte(key)
	return s
}

// NewZTEGPONRegisterService constructs the ZTE registration service.
func NewZTEGPONRegisterService(db *gorm.DB, jobs *JobService, snapshot ZTESnapshotter, factory CommanderFactory, rollback ZTERollbacker, logger *zap.Logger) *ZTEGPONRegisterService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ZTEGPONRegisterService{db: db, jobs: jobs, snapshot: snapshot, commanderFactory: factory, rollback: rollback, logger: logger}
}

// RegisterAndConfigure registers a new ONU and applies its Internet service.
func canonicalZTESerial(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s *ZTEGPONRegisterService) RegisterAndConfigure(ctx context.Context, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error) {
	olt, err := s.loadOLT(req.OLTID)
	if err != nil {
		return nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return nil, err
	}

	// The position is reserved in the same transaction as the ONT row. The
	// composite unique index is the final arbiter when two registrations race.
	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	var ont models.ONT
	var onuID int
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		onuID, err = resolveZTEONUIDLocked(ctx, tx, req.OLTID, req.Card, req.PON, req.ONUID)
		if err != nil {
			return err
		}
		ont = models.ONT{OLTID: req.OLTID, PortID: req.PON, ONTID: onuID, Slot: &req.Card, SerialNumber: req.SerialNumber, Name: req.Name, Description: req.Description, DeviceType: req.ONUType, Status: models.ONTStatusUnknown}
		if err := tx.Create(&ont).Error; err != nil {
			return reserveONUError(tx, req.SerialNumber, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	snap := &ConfigSnapshot{OntID: ont.ID, ZTE: &ZTESnapshot{SerialNumber: ont.SerialNumber, Name: ont.Name, DeviceType: ont.DeviceType, ServiceMode: "gpon-register"}}
	return s.executeJob(ctx, req, olt, ont, snap, userID, true)
}

// ConfigureExisting applies the Internet service to an already registered ONU.
func (s *ZTEGPONRegisterService) ConfigureExisting(ctx context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest, userID uuid.UUID) (*models.ProvisioningJob, error) {
	var ont models.ONT
	if err := s.db.WithContext(ctx).First(&ont, "id = ?", ontID).Error; err != nil {
		return nil, fmt.Errorf("load ONT: %w", err)
	}
	if req.OLTID != ont.OLTID {
		return nil, fmt.Errorf("request OLT does not own ONT")
	}
	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return nil, err
	}
	if ont.Slot == nil || *ont.Slot != req.Card || ont.PortID != req.PON {
		return nil, fmt.Errorf("request position does not match ONT")
	}
	if req.ONUIDMode == models.ZTEONUIDCustom && req.ONUID != ont.ONTID {
		return nil, fmt.Errorf("request ONU ID does not match ONT")
	}
	if s.snapshot == nil {
		return nil, fmt.Errorf("snapshot service not configured")
	}
	before, err := s.snapshot.CaptureBeforeSnapshot(ont)
	if err != nil {
		return nil, fmt.Errorf("capture before snapshot: %w", err)
	}
	return s.executeJob(ctx, req, olt, ont, before, userID, false)
}

// PreviewRegister returns the ONU ID the allocator would assign and the
// commands the OLT would receive, without writing anything. The preview an
// operator approves has to come from the same builder that runs, or it is not
// a preview of what happens.
func (s *ZTEGPONRegisterService) PreviewRegister(ctx context.Context, req models.ZTEGPONRegisterRequest) (int, []string, error) {
	olt, err := s.loadOLT(req.OLTID)
	if err != nil {
		return 0, nil, err
	}
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return 0, nil, err
	}

	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	onuID, err := ResolveZTEONUID(ctx, s.db, req.OLTID, req.Card, req.PON, req.ONUID)
	if err != nil {
		return 0, nil, err
	}

	commands, err := BuildZTEGPONRegisterCommands(req, onuID)
	if err != nil {
		return 0, nil, err
	}
	return onuID, commands, nil
}

// PreviewConfigure is the same for an ONT that is already registered, whose
// position and ONU ID are fixed.
func (s *ZTEGPONRegisterService) PreviewConfigure(ctx context.Context, ontID uuid.UUID, req models.ZTEGPONRegisterRequest) (int, []string, error) {
	var ont models.ONT
	if err := s.db.WithContext(ctx).First(&ont, "id = ?", ontID).Error; err != nil {
		return 0, nil, fmt.Errorf("load ONT: %w", err)
	}

	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return 0, nil, err
	}
	req.OLTID = ont.OLTID
	if err := ValidateZTEGPONRegister(req, olt); err != nil {
		return 0, nil, err
	}

	req.SerialNumber = canonicalZTESerial(req.SerialNumber)
	commands, err := BuildZTEGPONServiceCommands(req, ont.ONTID)
	if err != nil {
		return 0, nil, err
	}
	return ont.ONTID, commands, nil
}

func (s *ZTEGPONRegisterService) executeJob(ctx context.Context, req models.ZTEGPONRegisterRequest, olt *models.OLT, ont models.ONT, before *ConfigSnapshot, userID uuid.UUID, register bool) (*models.ProvisioningJob, error) {
	normalized := req
	normalized.PPPoEPassword = "<redacted>"
	config, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized request: %w", err)
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return nil, fmt.Errorf("marshal before snapshot: %w", err)
	}
	job, err := s.jobs.CreateProvisioningJob(ont.ID, nil, datatypes.JSON(config), datatypes.JSON(beforeJSON), userID)
	if err != nil {
		return nil, err
	}
	job.ONUID = ont.ONTID
	if err := s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil); err != nil {
		_ = s.db.Delete(&ont).Error
		return nil, err
	}

	commander, err := createCommanderForOLT(s.commanderFactory, *olt)
	if err != nil {
		return s.failJob(ctx, job, ont, before, false, fmt.Errorf("create commander: %w", err))
	}
	defer closeCommander(commander)
	var commands []string
	if register {
		commands, err = BuildZTEGPONRegisterExecutionCommands(req, ont.ONTID)
	} else {
		commands, err = BuildZTEGPONServiceExecutionCommands(req, ont.ONTID)
	}
	if err != nil {
		return s.failJob(ctx, job, ont, before, false, err)
	}
	results, err := commander.BatchExecute(ctx, commands)
	// Whether this job put the ONU on the OLT decides whether undoing it may
	// delete one. A batch stops at its first failure, so anything at or before
	// the registration line means the ONU is not ours to remove.
	createdONU := !register
	if err == nil {
		createdONU = true
		for i, result := range results {
			if result == nil || !result.Success {
				err = failedZTECommand(commands, i, result)
				createdONU = register && i > zteRegistrationIndex(commands)
				break
			}
		}
	}
	if err != nil {
		return s.failJob(ctx, job, ont, before, createdONU, fmt.Errorf("execute ZTE registration: %w", err))
	}
	if err := s.verify(req, ont); err != nil {
		return s.failJob(ctx, job, ont, before, createdONU, err)
	}
	// Recorded from the request rather than waited for: the discovery poll that
	// reads services back off the OLT is gated at thirty minutes, so the form
	// reopened empty for most of that window.
	if err := recordZTEService(s.db, s.encryptionKey, ont, req); err != nil {
		s.logger.Error("could not record the applied ZTE service",
			zap.String("ont_id", ont.ID.String()), zap.Error(err))
	}
	// Read straight off the OLT rather than left at "unknown" for the discovery
	// poll to notice. Failing here costs the row nothing: the poll still runs.
	s.settleONUStatus(*olt, ont)
	if err := s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusSuccess, nil); err != nil {
		return nil, err
	}
	job.Status = models.ProvisioningStatusSuccess
	return job, nil
}

// settleONUStatus fills in the new ONU's status without waiting for the
// discovery poll. The operator's request waits only briefly for it; an OLT too
// busy to answer in that time - three and seven seconds for a GET that costs
// seven milliseconds when idle - is left to a backstop that outlives the
// request rather than holding the dialog open for it.
func (s *ZTEGPONRegisterService) settleONUStatus(olt models.OLT, ont models.ONT) {
	resolved, err := resolveONUStatusAfterProvision(s.db, olt, ont, onuSettleWindow)
	if err != nil {
		s.logger.Warn("could not read the ONU status after provisioning",
			zap.String("ont_id", ont.ID.String()), zap.Error(err))
		return
	}
	if resolved {
		return
	}

	go func() {
		if _, err := resolveONUStatusAfterProvision(s.db, olt, ont, onuSettleBackstop); err != nil {
			s.logger.Warn("could not read the ONU status after provisioning",
				zap.String("ont_id", ont.ID.String()), zap.Error(err))
		}
	}()
}

// failedZTECommand names the command the OLT refused and quotes what it said
// back. "command 2 failed" alone left an operator, and the log, with no way to
// tell a rejected serial from a mistyped profile.
//
// Both the command and the reply are redacted: a failing wan-ip line carries
// the subscriber's PPPoE password.
func failedZTECommand(commands []string, index int, result *connectivity.CommandResult) error {
	command := ""
	if index < len(commands) {
		command = RedactZTECommands([]string{commands[index]})[0]
	}

	reply := "no reply"
	if result != nil && strings.TrimSpace(result.Error) != "" {
		reply = RedactZTECommands([]string{strings.TrimSpace(result.Error)})[0]
	}

	return fmt.Errorf("command %d %q failed: %s%s", index, command, reply, zteFailureHint(command, reply))
}

// zteDeviceRefusal marks a reply the OLT actually sent to refuse a command, as
// opposed to no reply at all.
var zteDeviceRefusal = regexp.MustCompile(`(?i)\[Failed\]|%Error`)

// zteRegistrationLine matches the command that binds a serial to an ONU ID.
var zteRegistrationLine = regexp.MustCompile(`^onu \d+ type (\S+) sn \S+`)

// zteFailureHint adds what the OLT's own answer leaves out.
//
// A C300 answers the registration line with a bare ".[Failed]" when it is given
// a specific ONU type for an ONU it cannot currently see: it has no way to
// check the hardware against the type. Observed directly — "type HG8245H5" was
// refused for an ONU that was offline, and "type ALL" for the same serial on
// the same port was accepted seconds later.
func zteFailureHint(command, reply string) string {
	// Only when the OLT actually answered and refused. A timeout means the
	// reply never came, and explaining it as a rejected ONU type gave a
	// confident diagnosis of something that had not happened.
	if !zteDeviceRefusal.MatchString(reply) {
		return ""
	}

	match := zteRegistrationLine.FindStringSubmatch(command)
	if match == nil || strings.EqualFold(match[1], "ALL") {
		return ""
	}
	return fmt.Sprintf(
		". The OLT rejects a specific ONU type for an ONU it cannot see;"+
			" register %s as type ALL, or bring the ONU online first", match[1])
}

func (s *ZTEGPONRegisterService) verify(req models.ZTEGPONRegisterRequest, ont models.ONT) error {
	if s.snapshot == nil {
		return fmt.Errorf("snapshot service not configured")
	}
	after, err := s.snapshot.CaptureAfterSnapshot(ont)
	if err != nil {
		return fmt.Errorf("capture after snapshot: %w", err)
	}
	wantSerial := strings.ToUpper(strings.TrimSpace(req.SerialNumber))
	if after == nil || after.ZTE == nil || after.ZTE.SerialNumber != wantSerial {
		return fmt.Errorf("verification failed: serial or position mismatch")
	}
	if position, ok := zteSnapshotPosition(after); ok && (position.slot != intSlotOrDefault(ont.Slot) || position.port != ont.PortID || position.ontID != ont.ONTID) {
		return fmt.Errorf("verification failed: serial or position mismatch")
	}
	return nil
}

func zteSnapshotPosition(snapshot *ConfigSnapshot) (position struct{ slot, port, ontID int }, ok bool) {
	if snapshot == nil || snapshot.RawReadings == nil {
		return position, false
	}
	readInt := func(key string) (int, bool) {
		value, exists := snapshot.RawReadings[key]
		if !exists {
			return 0, false
		}
		switch value := value.(type) {
		case int:
			return value, true
		case float64:
			return int(value), value == float64(int(value))
		default:
			return 0, false
		}
	}
	var found bool
	if position.slot, found = readInt("slot"); !found {
		return position, false
	}
	if position.port, found = readInt("port"); !found {
		return position, false
	}
	if position.ontID, found = readInt("ont_id"); !found {
		return position, false
	}
	return position, true
}

// failJob records the failure and undoes what the job did. createdONU says
// whether this job actually registered the ONU on the OLT: when it did not,
// the OLT is left alone.
//
// Rolling back regardless was dangerous. A registration refused because the
// serial was already on the OLT would answer by sending "no onu 15" — deleting
// a working subscriber that an earlier job had put there. It did no damage the
// one time it happened only because the OLT refused that command too.
func (s *ZTEGPONRegisterService) failJob(ctx context.Context, job *models.ProvisioningJob, ont models.ONT, before *ConfigSnapshot, createdONU bool, cause error) (*models.ProvisioningJob, error) {
	message := cause.Error()
	_ = s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &message)
	if before != nil && before.ZTE != nil && before.ZTE.ServiceMode == "gpon-register" {
		_ = s.db.Delete(&models.ONT{}, "id = ?", ont.ID).Error
	}
	if s.rollback != nil && createdONU {
		var rollbackErr error
		if rollbacker, ok := s.rollback.(zteOLTRollbacker); ok {
			olt, loadErr := s.loadOLT(ont.OLTID)
			if loadErr != nil {
				rollbackErr = loadErr
			} else {
				rollbackErr = rollbacker.RollbackToSnapshotForOLT(ctx, *olt, ont, before)
			}
		} else {
			rollbackErr = s.rollback.RollbackToSnapshot(ctx, ont, before)
		}
		if rollbackErr == nil {
			rollbackMessage := "rollback completed after provisioning failure"
			_ = s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRolledBack, &rollbackMessage)
		} else {
			s.logger.Error("ZTE rollback failed", zap.String("job_id", job.ID.String()), zap.Error(rollbackErr))
		}
	}
	return nil, cause
}

func (s *ZTEGPONRegisterService) loadOLT(id uuid.UUID) (*models.OLT, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("load OLT: %w", err)
	}
	return &olt, nil
}

func resolveZTEONUIDLocked(ctx context.Context, db *gorm.DB, oltID uuid.UUID, slotID, portID, requested int) (int, error) {
	if db.Name() == "postgres" {
		if err := db.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("zte-onu:%s:%d:%d", oltID, slotID, portID)).Error; err != nil {
			return 0, fmt.Errorf("lock ONU position: %w", err)
		}
	}
	var onts []models.ONT
	if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("olt_id = ? AND slot = ? AND port_id = ?", oltID, slotID, portID).Find(&onts).Error; err != nil {
		return 0, fmt.Errorf("resolve ONU ID: %w", err)
	}
	used := make(map[int]struct{}, len(onts))
	for _, ont := range onts {
		used[ont.ONTID] = struct{}{}
	}
	if requested > 0 {
		if _, ok := used[requested]; ok {
			return 0, fmt.Errorf("ONU ID %d is already used on this port", requested)
		}
		return requested, nil
	}
	for id := minZTEONUID; id <= maxZTEONUID; id++ {
		if _, ok := used[id]; !ok {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free ONU IDs remain on this port")
}

// reserveONUError turns the serial index's rejection into something an operator
// can act on. A registration against a busy OLT can take minutes, so an
// operator who reloads and submits again lands here; the raw constraint
// violation read as a fault in TikMan rather than as the first attempt still
// running.
func reserveONUError(tx *gorm.DB, serial string, cause error) error {
	if !errors.Is(cause, gorm.ErrDuplicatedKey) && !strings.Contains(cause.Error(), "idx_onts_serial_number") {
		return fmt.Errorf("reserve ONU position: %w", cause)
	}

	var existing models.ONT
	if err := tx.Where("serial_number = ?", serial).First(&existing).Error; err != nil {
		return fmt.Errorf("serial %s is already registered", serial)
	}

	var running int64
	tx.Model(&models.ProvisioningJob{}).
		Where("ont_id = ? AND status = ?", existing.ID, "running").
		Count(&running)
	if running > 0 {
		return fmt.Errorf("a registration for %s is already running; wait for it to finish rather than starting another", serial)
	}

	position := "an unknown position"
	if existing.Slot != nil {
		position = fmt.Sprintf("1/%d/%d:%d", *existing.Slot, existing.PortID, existing.ONTID)
	}
	return fmt.Errorf("serial %s is already registered at %s", serial, position)
}

// zteRegistrationIndex reports where the "onu N type X sn Y" line sits in a
// command list, or -1 when the list has none.
func zteRegistrationIndex(commands []string) int {
	for i, command := range commands {
		if zteRegistrationLine.MatchString(strings.TrimSpace(command)) {
			return i
		}
	}
	return -1
}
