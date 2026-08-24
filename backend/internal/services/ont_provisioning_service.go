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
	db          *gorm.DB
	jobService  *JobService
	snapshotSvc *SnapshotService
	commander   connectivity.OLTCommander
	rollback    *RollbackEngine
	audit       *AuditService
	logger      *zap.Logger
}

// NewOntProvisioningService constructs an OntProvisioningService instance.
func NewOntProvisioningService(
	db *gorm.DB,
	jobService *JobService,
	snapshotSvc *SnapshotService,
	commander connectivity.OLTCommander,
	rollback *RollbackEngine,
	audit *AuditService,
	logger *zap.Logger,
) *OntProvisioningService {
	return &OntProvisioningService{
		db:          db,
		jobService:  jobService,
		snapshotSvc: snapshotSvc,
		commander:   commander,
		rollback:    rollback,
		audit:       audit,
		logger:      logger,
	}
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
	// Step 1: Load ONT and validate it exists
	ont, err := s.loadONT(ontID)
	if err != nil {
		return nil, fmt.Errorf("failed to load ONT %s: %w", ontID, err)
	}

	// Step 2: Load OLT for driver resolution
	olt, err := s.loadOLT(ont.OLTID)
	if err != nil {
		return nil, fmt.Errorf("failed to load OLT %s: %w", ont.OLTID, err)
	}

	// Step 3: Build config from template + manual overrides
	provisionConfig, err := s.buildProvisionConfig(config, ont, olt)
	if err != nil {
		return nil, fmt.Errorf("failed to build provision config: %w", err)
	}

	// Step 4: Capture BeforeSnapshot (baseline for comparison)
	beforeSnap, err := s.snapshotSvc.CaptureBeforeSnapshot(*ont)
	if err != nil {
		return nil, fmt.Errorf("failed to capture before snapshot: %w", err)
	}

	// Step 5: Create provisioning job in PENDING state
	beforeJSON, _ := json.Marshal(beforeSnap)
	configJSON, err := json.Marshal(provisionConfig)
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

	// Step 6: Update status to RUNNING
	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusRunning, nil); err != nil {
		return nil, fmt.Errorf("failed to update job status to running: %w", err)
	}

	// Step 7: Execute provision (send config commands to OLT)
	_, executeErr := s.executeProvision(ctx, ont, olt, provisionConfig)
	if executeErr != nil {
		// Step 8a: Failure → mark FAILED, trigger rollback
		errMsg := executeErr.Error()
		if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &errMsg); err != nil {
			s.logger.Error("failed to mark job as failed", zap.String("job_id", job.ID.String()), zap.Error(err))
		}
		// Attempt rollback (will return without side effects if RollbackTo is not implemented)
		if rbErr := s.rollbackOnt(job, *ont, beforeSnap); rbErr != nil {
			s.logger.Error("rollback attempt failed after provision failure", zap.String("job_id", job.ID.String()), zap.Error(rbErr))
		}
		return nil, fmt.Errorf("provision execution failed: %w", executeErr)
	}

	// Step 8b: Success → verify config match via Compare
	afterSnap, err := s.snapshotSvc.CaptureAfterSnapshot(*ont)
	if err != nil {
		s.logger.Warn("failed to capture after snapshot during success verification", zap.String("job_id", job.ID.String()))
	}

	if afterSnap != nil && beforeSnap != nil {
		diffs := s.snapshotSvc.Compare(beforeSnap, afterSnap)
		if len(diffs) > 0 {
			diffJSON, _ := json.Marshal(diffs)
			errMsg := fmt.Sprintf("config drift detected: %s", string(diffJSON))
			if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusFailed, &errMsg); err != nil {
				s.logger.Error("failed to mark job as failed due to drift", zap.String("job_id", job.ID.String()), zap.Error(err))
			}
			if rbErr := s.rollbackOnt(job, *ont, beforeSnap); rbErr != nil {
				s.logger.Error("rollback attempt failed after drift detection", zap.String("job_id", job.ID.String()), zap.Error(rbErr))
			}
			return nil, fmt.Errorf("config drift detected: %v", diffs)
		}
	}

	// Step 9: Mark SUCCESS
	if err := s.jobService.UpdateStatusProvisioning(job.ID, models.ProvisioningStatusSuccess, nil); err != nil {
		return nil, fmt.Errorf("failed to update job status to success: %w", err)
	}

	s.logAudit(userID, "provision", "ont", ontID, nil, map[string]interface{}{
		"job_id":   job.ID,
		"status":   models.ProvisioningStatusSuccess,
		"ont_id":   ont.ID,
		"olt_ip":   olt.IPAddress,
		"template": config.TemplateID,
	})

	return &ProvisionResult{
		Job:    job,
		Config: provisionConfig,
	}, nil
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

func (s *OntProvisioningService) buildProvisionConfig(config ProvisionConfig, ont *models.ONT, olt *models.OLT) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// TODO: Fetch template from ConfigTemplateService and merge fields when config.TemplateID is set

	// Add manual overrides
	for k, v := range config.ManualConfig {
		result[k] = v
	}

	// Add implicit ONT/Olt IDs
	result["ont_id"] = ont.ID
	result["olt_id"] = olt.ID
	result["slot"] = ont.Slot
	result["port"] = ont.PortID
	result["ontid"] = ont.ONTID

	return result, nil
}

func (s *OntProvisioningService) executeProvision(ctx context.Context, ont *models.ONT, olt *models.OLT, config map[string]interface{}) (*connectivity.CommandResult, error) {
	// Prepare vendor-specific CLI commands based on OLT model
	var cmds []string

	switch olt.Model {
	case models.OLTModelZTEC300, models.OLTModelZTEC320:
		cmds = s.buildZTECommands(*ont, config)
	default:
		cmds = s.buildHSGQCommands(*ont, config)
	}

	results, err := s.commander.BatchExecute(ctx, cmds)
	if err != nil {
		return nil, fmt.Errorf("command batch execution failed: %w", err)
	}

	// Check results
	for i, r := range results {
		if !r.Success {
			return nil, fmt.Errorf("command %d failed: %s", i, r.Output)
		}
	}

	return results[len(results)-1], nil
}

func (s *OntProvisioningService) buildZTECommands(ont models.ONT, config map[string]interface{}) []string {
	cmds := []string{
		"config",
		fmt.Sprintf("interface gpon 0/%d", ont.PortID),
		fmt.Sprintf("ont ontid %d profile 1", ont.ONTID),
		"commit",
	}

	// Add bandwidth/vlan config if present
	if bw, ok := config["bandwidth"]; ok {
		cmds = append(cmds, fmt.Sprintf("ont traffic band-width %s %d", bw, ont.ONTID))
	}
	if vlan, ok := config["vlan"]; ok {
		cmds = append(cmds, fmt.Sprintf("ont service vlan add %s %d", vlan, ont.ONTID))
	}

	return cmds
}

func (s *OntProvisioningService) buildHSGQCommands(ont models.ONT, config map[string]interface{}) []string {
	return []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-oltport 0/%d", ont.PortID),
		fmt.Sprintf("ont create %d serial XXXX service-profile 1", ont.ONTID),
		"commit",
	}
}

func (s *OntProvisioningService) rollbackOnt(job *models.ProvisioningJob, ont models.ONT, snap *ConfigSnapshot) error {
	if s.rollback == nil {
		return fmt.Errorf("rollback engine not configured")
	}
	return s.rollback.RollbackToSnapshot(context.Background(), ont, snap)
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
