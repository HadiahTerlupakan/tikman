package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
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
			return fmt.Errorf("reserve ONU position: %w", err)
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
		return s.failJob(ctx, job, ont, before, fmt.Errorf("create commander: %w", err))
	}
	defer closeCommander(commander)
	var commands []string
	if register {
		commands, err = BuildZTEGPONRegisterExecutionCommands(req, ont.ONTID)
	} else {
		commands, err = BuildZTEGPONServiceExecutionCommands(req, ont.ONTID)
	}
	if err != nil {
		return s.failJob(ctx, job, ont, before, err)
	}
	results, err := commander.BatchExecute(ctx, commands)
	if err == nil {
		for i, result := range results {
			if result == nil || !result.Success {
				err = fmt.Errorf("command %d failed", i)
				break
			}
		}
	}
	if err != nil {
		return s.failJob(ctx, job, ont, before, fmt.Errorf("execute ZTE registration: %w", err))
	}
	if err := s.verify(req, ont); err != nil {
		return s.failJob(ctx, job, ont, before, err)
	}
	if err := s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusSuccess, nil); err != nil {
		return nil, err
	}
	job.Status = models.ProvisioningStatusSuccess
	return job, nil
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

func (s *ZTEGPONRegisterService) failJob(ctx context.Context, job *models.ProvisioningJob, ont models.ONT, before *ConfigSnapshot, cause error) (*models.ProvisioningJob, error) {
	message := cause.Error()
	_ = s.jobs.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &message)
	if before != nil && before.ZTE != nil && before.ZTE.ServiceMode == "gpon-register" {
		_ = s.db.Delete(&models.ONT{}, "id = ?", ont.ID).Error
	}
	if s.rollback != nil {
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
