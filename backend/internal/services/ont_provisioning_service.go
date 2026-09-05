package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// OntProvisioningService orchestrates single ONT provisioning with state tracking,
// snapshot capture, command execution, rollback on failure, and audit logging.
type OntProvisioningService struct {
	db               *gorm.DB
	jobService       *JobService
	snapshotSvc      *SnapshotService
	commanderFactory CommanderFactory
	rollback         *RollbackEngine
	audit            *AuditService
	logger           *zap.Logger
	templates        configTemplateReader
}

// CommanderFactory abstracts connectivity.CommanderFactory for testability.
type CommanderFactory interface {
	ForOLT(model models.OLTModel, host string, port int, username, password string) (connectivity.OLTCommander, error)
}

type protocolCommanderFactory interface {
	ForOLTWithProtocol(model models.OLTModel, host string, protocol models.OLTProtocol, port int, username, password string) (connectivity.OLTCommander, error)
}

type oltRollbacker interface {
	RollbackToSnapshotForOLT(context.Context, models.OLT, models.ONT, *ConfigSnapshot) error
}

type configTemplateReader interface {
	GetByID(uuid.UUID) (*models.ConfigTemplate, error)
}

// NewOntProvisioningService constructs an OntProvisioningService instance.
func NewOntProvisioningService(
	db *gorm.DB,
	jobService *JobService,
	snapshotSvc *SnapshotService,
	commanderFactory CommanderFactory,
	rollback *RollbackEngine,
	audit *AuditService,
	logger *zap.Logger,
) *OntProvisioningService {
	return &OntProvisioningService{
		db:               db,
		jobService:       jobService,
		snapshotSvc:      snapshotSvc,
		commanderFactory: commanderFactory,
		rollback:         rollback,
		audit:            audit,
		logger:           logger,
	}
}

// NewOntProvisioningServiceWithTemplates constructs a provisioning service with template lookup enabled.
func NewOntProvisioningServiceWithTemplates(
	db *gorm.DB,
	jobService *JobService,
	snapshotSvc *SnapshotService,
	commanderFactory CommanderFactory,
	rollback *RollbackEngine,
	audit *AuditService,
	logger *zap.Logger,
	templates configTemplateReader,
) *OntProvisioningService {
	service := NewOntProvisioningService(db, jobService, snapshotSvc, commanderFactory, rollback, audit, logger)
	service.templates = templates
	return service
}

// ProvisionConfig represents the configuration payload for provisioning.
type ProvisionConfig struct {
	TemplateID   *uuid.UUID             `json:"template_id"`   // Optional template ID
	ManualConfig map[string]interface{} `json:"manual_config"` // Manual override fields
	Confirm      bool                   `json:"confirm"`       // User confirmation flag
}

// ProvisionResult contains the outcome of provisioning operation.
type ProvisionResult struct {
	Job        *models.ProvisioningJob
	Config     map[string]interface{}
	DiffBefore string
}

// ProvisionOnt initiates provisioning for a single ONT with state machine enforcement,
// snapshot capture, command execution, and automatic rollback on failure.
func (s *OntProvisioningService) ProvisionOnt(
	ctx context.Context,
	ontID uuid.UUID,
	userID uuid.UUID,
	config ProvisionConfig,
) (*ProvisionResult, error) {
	ont, err := s.loadONT(ontID)
	if err != nil {
		return nil, fmt.Errorf("failed to load ONT %s: %w", ontID, err)
	}
	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return nil, fmt.Errorf("failed to load OLT %s: %w", ont.OLTID, err)
	}

	provisionConfig, err := s.buildProvisionConfig(config, ont, olt)
	if err != nil {
		return nil, fmt.Errorf("failed to build provision config: %w", err)
	}
	if err := validateManualConfig(config.ManualConfig); err != nil {
		return nil, fmt.Errorf("invalid manual config: %w", err)
	}

	// The baseline is captured before anything is sent, because it is what a
	// rollback restores to.
	beforeSnap, err := s.snapshotSvc.CaptureBeforeSnapshot(*ont)
	if err != nil {
		return nil, fmt.Errorf("failed to capture before snapshot: %w", err)
	}

	job, err := s.startJob(ontID, userID, config, provisionConfig, beforeSnap)
	if err != nil {
		return nil, err
	}

	if _, err := s.executeProvision(ctx, ont, olt, provisionConfig); err != nil {
		errMsg := redactProvisioningError(err.Error(), provisionConfig)
		s.abandonJob(job, ont, olt, beforeSnap, errMsg)
		return nil, fmt.Errorf("provision execution failed: %s", errMsg)
	}

	if err := s.verifyAgainstSnapshot(job, ont, olt, beforeSnap); err != nil {
		return nil, err
	}

	return s.completeJob(job, ont, olt, userID, config.TemplateID, provisionConfig)
}

// completeJob marks the run successful and returns it as stored, so the caller
// reports the row the rest of the system will see rather than the one it
// started with.
func (s *OntProvisioningService) completeJob(job *models.ProvisioningJob, ont *models.ONT, olt *models.OLT,
	userID uuid.UUID, templateID *uuid.UUID, provisionConfig map[string]interface{}) (*ProvisionResult, error) {

	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusSuccess, nil); err != nil {
		return nil, fmt.Errorf("failed to update job status to success: %w", err)
	}

	s.logAudit(userID, "provision", "ont", ont.ID, nil, map[string]interface{}{
		"job_id":   job.ID,
		"status":   models.ProvisioningStatusSuccess,
		"ont_id":   ont.ID,
		"olt_ip":   olt.IPAddress,
		"template": templateID,
	})

	latest, err := s.jobService.GetProvisioningJob(job.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload completed job: %w", err)
	}
	return &ProvisionResult{Job: latest, Config: provisionConfig}, nil
}

// startJob records the run before a single command is sent, so a provision
// that dies mid-flight is still visible as a job rather than as nothing.
//
// The stored config is redacted; the runtime one the caller keeps is not.
func (s *OntProvisioningService) startJob(ontID, userID uuid.UUID, config ProvisionConfig,
	provisionConfig map[string]interface{}, beforeSnap *ConfigSnapshot) (*models.ProvisioningJob, error) {

	beforeJSON, _ := json.Marshal(beforeSnap)
	configJSON, err := json.Marshal(redactManualConfig(provisionConfig))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal provision config: %w", err)
	}

	job, err := s.jobService.CreateProvisioningJob(
		ontID,
		config.TemplateID,
		datatypes.JSON(configJSON),
		datatypes.JSON(beforeJSON),
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create provisioning job: %w", err)
	}

	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil); err != nil {
		return nil, fmt.Errorf("failed to update job status to running: %w", err)
	}
	return job, nil
}

// abandonJob marks a run failed and puts the ONT back as it was. Neither step
// can be allowed to hide the other's failure, so both are logged rather than
// returned: the caller already has the reason the provision failed.
func (s *OntProvisioningService) abandonJob(job *models.ProvisioningJob, ont *models.ONT, olt *models.OLT,
	beforeSnap *ConfigSnapshot, reason string) {

	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &reason); err != nil {
		s.logger.Error("failed to mark job as failed", zap.String("job_id", job.ID.String()), zap.Error(err))
	}
	// Returns without side effects when the driver implements no RollbackTo.
	if err := s.rollbackOnt(job, *ont, *olt, beforeSnap); err != nil {
		s.logger.Error("rollback attempt failed", zap.String("job_id", job.ID.String()), zap.Error(err))
	}
}

// verifyAgainstSnapshot reads the ONT back and refuses a run whose result does
// not match what was asked for. A provision that reports success while the
// chassis holds something else is worse than one that reports failure.
func (s *OntProvisioningService) verifyAgainstSnapshot(job *models.ProvisioningJob, ont *models.ONT,
	olt *models.OLT, beforeSnap *ConfigSnapshot) error {

	afterSnap, err := s.snapshotSvc.CaptureAfterSnapshot(*ont)
	if err != nil {
		s.abandonJob(job, ont, olt, beforeSnap, fmt.Sprintf("after snapshot failed: %v", err))
		return fmt.Errorf("after snapshot failed: %w", err)
	}
	if afterSnap == nil {
		s.abandonJob(job, ont, olt, beforeSnap, "after snapshot is empty")
		return fmt.Errorf("after snapshot is empty")
	}
	if beforeSnap == nil {
		return nil
	}

	diffs := s.snapshotSvc.Compare(beforeSnap, afterSnap)
	if len(diffs) == 0 {
		return nil
	}

	diffJSON, _ := json.Marshal(diffs)
	s.abandonJob(job, ont, olt, beforeSnap, fmt.Sprintf("config drift detected: %s", string(diffJSON)))
	return fmt.Errorf("config drift detected: %v", diffs)
}

// RollbackOnt restores ONT configuration from stored snapshot.
// This is called automatically when provisioning fails or config verification detects drift.
func (s *OntProvisioningService) RollbackOnt(ctx context.Context, ontID uuid.UUID, jobID uuid.UUID) error {
	job, err := s.jobService.GetProvisioningJob(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job %s: %w", jobID, err)
	}

	if job.BeforeSnapshot == nil {
		return fmt.Errorf("no before snapshot available for rollback")
	}

	var snap ConfigSnapshot
	if err := json.Unmarshal([]byte(job.BeforeSnapshot), &snap); err != nil {
		return fmt.Errorf("failed to unmarshal before snapshot: %w", err)
	}

	ont, err := s.loadONT(ontID)
	if err != nil {
		return fmt.Errorf("failed to load ONT %s: %w", ontID, err)
	}

	if err := s.snapshotSvc.RollbackTo(ctx, *ont, &snap); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	errMsg := "rollback completed after provisioning failure"
	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRolledBack, &errMsg); err != nil {
		return fmt.Errorf("failed to update job status to rolled_back: %w", err)
	}

	s.logAudit(uuid.Nil, "rollback", "ont", ontID, nil, map[string]interface{}{
		"job_id": jobID,
		"ont_id": ontID,
	})

	return nil
}

// GetProvisioningJob retrieves a provisioning job by ID.
func (s *OntProvisioningService) GetProvisioningJob(jobID uuid.UUID) (*models.ProvisioningJob, error) {
	return s.jobService.GetProvisioningJob(jobID)
}

// ListProvisioningJobsByONT returns jobs for an ONT, newest first.
func (s *OntProvisioningService) ListProvisioningJobsByONT(ontID uuid.UUID, limit, offset int) ([]models.ProvisioningJob, int64, error) {
	return s.jobService.ListProvisioningJobsByONT(ontID, limit, offset)
}

// Helper methods

func (s *OntProvisioningService) loadONT(ontID uuid.UUID) (*models.ONT, error) {
	var ont models.ONT
	if err := s.db.First(&ont, "id = ?", ontID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("ONT not found: %w", err)
		}
		return nil, fmt.Errorf("database error loading ONT: %w", err)
	}
	return &ont, nil
}

func (s *OntProvisioningService) loadOLT(oltID uuid.UUID) (*models.OLT, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("OLT not found: %w", err)
		}
		return nil, fmt.Errorf("database error loading OLT: %w", err)
	}
	return &olt, nil
}
func (s *OntProvisioningService) logAudit(
	userID uuid.UUID,
	action, resourceType string,
	resourceID uuid.UUID,
	oldValue, newValue map[string]interface{},
) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(userID, action, resourceType, resourceID, oldValue, newValue, "", "")
}
