package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
)

// Carrying out one registration against the chassis: the command run itself,
// what a device refusal looks like in the reply, and the rollback when it
// fails partway.

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

	createdONU, err := s.runRegistration(ctx, req, olt, ont, register)
	if err != nil {
		return s.failJob(ctx, job, ont, before, createdONU, err)
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
// runRegistration opens a CLI session and applies the commands, reporting
// whether this job is the one that put the ONU on the chassis.
//
// That answer decides whether undoing the job may delete an ONU. A batch stops
// at its first failure, so anything failing at or before the registration line
// means the ONU is not ours to remove.
func (s *ZTEGPONRegisterService) runRegistration(ctx context.Context, req models.ZTEGPONRegisterRequest,
	olt *models.OLT, ont models.ONT, register bool) (bool, error) {

	commander, err := createCommanderForOLT(s.commanderFactory, *olt)
	if err != nil {
		return false, fmt.Errorf("create commander: %w", err)
	}
	defer closeCommander(commander)

	var commands []string
	if register {
		commands, err = BuildZTEGPONRegisterExecutionCommands(req, ont.ONTID)
	} else {
		commands, err = BuildZTEGPONServiceExecutionCommands(req, ont.ONTID)
	}
	if err != nil {
		return false, err
	}

	results, err := commander.BatchExecute(ctx, commands)
	if err != nil {
		return !register, fmt.Errorf("execute ZTE registration: %w", err)
	}

	for i, result := range results {
		if result == nil || !result.Success {
			createdONU := register && i > zteRegistrationIndex(commands)
			return createdONU, fmt.Errorf("execute ZTE registration: %w", failedZTECommand(commands, i, result))
		}
	}
	return true, nil
}

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
