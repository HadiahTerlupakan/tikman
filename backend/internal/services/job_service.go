package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// OntJobResult is the per-ONT outcome stored in a batch job's ont_results map.
type OntJobResult struct {
	Status   string            `json:"status"`
	Error    string            `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// JobService manages provisioning and batch jobs with state machine enforcement.
type JobService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewJobService constructs a new JobService instance.
func NewJobService(db *gorm.DB, audit *AuditService) *JobService {
	return &JobService{db: db, audit: audit}
}

// CreateProvisioningJob creates a new provisioning job in pending state,
// refusing when another job is already running for the same ONT: two
// concurrent writers would corrupt the ONT configuration.
func (s *JobService) CreateProvisioningJob(
	ontID uuid.UUID,
	templateID *uuid.UUID,
	configSnapshot datatypes.JSON,
	beforeSnapshot datatypes.JSON,
	userID uuid.UUID,
) (*models.ProvisioningJob, error) {
	if err := s.ensureNoRunningJob(ontID); err != nil {
		return nil, err
	}

	job := &models.ProvisioningJob{
		ONTID:          ontID,
		TemplateID:     templateID,
		Status:         models.ProvisioningStatusPending,
		ConfigSnapshot: configSnapshot,
		BeforeSnapshot: beforeSnapshot,
		CreatedBy:      &userID,
	}

	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create provisioning job: %w", err)
	}

	s.logAudit(userID, "create", "provisioning_job", job.ID, nil, map[string]interface{}{
		"ont_id":      ontID,
		"template_id": templateID,
	})

	return job, nil
}

// ensureNoRunningJob returns an error when a job is already running for ontID.
func (s *JobService) ensureNoRunningJob(ontID uuid.UUID) error {
	var runningCount int64
	err := s.db.Model(&models.ProvisioningJob{}).
		Where("ont_id = ? AND status = ?", ontID, models.ProvisioningStatusRunning).
		Count(&runningCount).Error
	if err != nil {
		return fmt.Errorf("failed to check for running job: %w", err)
	}
	if runningCount > 0 {
		return fmt.Errorf("another provisioning job is already running for ont %s", ontID)
	}
	return nil
}

// GetProvisioningJob retrieves a provisioning job by ID.
func (s *JobService) GetProvisioningJob(jobID uuid.UUID) (*models.ProvisioningJob, error) {
	var job models.ProvisioningJob
	if err := s.db.First(&job, "id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("provisioning job not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &job, nil
}

// UpdateStatusProvisioning transitions a provisioning job to newStatus after
// validating it against the state machine:
// pending -> running -> success | failed, and failed -> rolled_back.
// Terminal statuses set completed_at; a transition to running is refused while
// another job is running for the same ONT.
func (s *JobService) UpdateStatusProvisioning(
	jobID uuid.UUID,
	newStatus string,
	errorMessage *string,
) error {
	if !isValidProvisioningStatus(newStatus) {
		return fmt.Errorf("invalid status '%s': must be one of pending, running, success, failed, rolled_back", newStatus)
	}

	current, err := s.GetProvisioningJob(jobID)
	if err != nil {
		return err
	}

	if !isValidProvisioningTransition(current.Status, newStatus) {
		return fmt.Errorf("invalid status transition from '%s' to '%s'", current.Status, newStatus)
	}

	if newStatus == models.ProvisioningStatusRunning {
		if err := s.ensureNoRunningJob(current.ONTID); err != nil {
			return err
		}
	}

	updates := map[string]interface{}{
		"status": newStatus,
	}
	if isProvisioningTerminal(newStatus) {
		updates["completed_at"] = time.Now()
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}

	if err := s.db.Model(&models.ProvisioningJob{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update provisioning job: %w", err)
	}

	s.logAudit(uuid.Nil, "update", "provisioning_job", jobID,
		map[string]interface{}{"status": current.Status},
		map[string]interface{}{"status": newStatus},
	)

	return nil
}

// ListProvisioningJobsByONT returns jobs for an ONT, newest first, with total count.
func (s *JobService) ListProvisioningJobsByONT(ontID uuid.UUID, limit, offset int) ([]models.ProvisioningJob, int64, error) {
	var total int64
	if err := s.db.Model(&models.ProvisioningJob{}).Where("ont_id = ?", ontID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	var jobs []models.ProvisioningJob
	if err := s.db.Where("ont_id = ?", ontID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&jobs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	return jobs, total, nil
}

// CreateBatchJob creates a new batch job in pending state.
func (s *JobService) CreateBatchJob(
	templateID uuid.UUID,
	ontIDs []uuid.UUID,
	userID uuid.UUID,
) (*models.BatchJob, error) {
	if len(ontIDs) == 0 {
		return nil, errors.New("batch job must contain at least one ONT")
	}

	job := &models.BatchJob{
		TemplateID: templateID,
		ONTIDs:     models.UUIDSlice(ontIDs),
		Status:     models.BatchStatusPending,
		CreatedBy:  &userID,
	}

	if err := s.db.Create(job).Error; err != nil {
		return nil, fmt.Errorf("failed to create batch job: %w", err)
	}

	s.logAudit(userID, "create", "batch_job", job.ID, nil, map[string]interface{}{
		"template_id": templateID,
		"ont_count":   len(ontIDs),
	})

	return job, nil
}

// GetBatchJob retrieves a batch job by ID.
func (s *JobService) GetBatchJob(jobID uuid.UUID) (*models.BatchJob, error) {
	var job models.BatchJob
	if err := s.db.First(&job, "id = ?", jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("batch job not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &job, nil
}

// UpdateStatusBatch transitions a batch job to newStatus after validating it
// against the state machine: pending -> running -> success | failed |
// partial_rollback. ontResults is persisted to ont_results when provided.
func (s *JobService) UpdateStatusBatch(
	jobID uuid.UUID,
	newStatus string,
	ontResults map[string]OntJobResult,
) error {
	if !isValidBatchStatus(newStatus) {
		return fmt.Errorf("invalid status '%s': must be one of pending, running, success, failed, partial_rollback", newStatus)
	}

	current, err := s.GetBatchJob(jobID)
	if err != nil {
		return err
	}

	if !isValidBatchTransition(current.Status, newStatus) {
		return fmt.Errorf("invalid status transition from '%s' to '%s'", current.Status, newStatus)
	}

	updates := map[string]interface{}{
		"status": newStatus,
	}
	if isBatchTerminal(newStatus) {
		updates["completed_at"] = time.Now()
	}
	if len(ontResults) > 0 {
		resultsJSON, err := json.Marshal(ontResults)
		if err != nil {
			return fmt.Errorf("failed to marshal ONT results: %w", err)
		}
		updates["ont_results"] = datatypes.JSON(resultsJSON)
	}

	if err := s.db.Model(&models.BatchJob{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update batch job: %w", err)
	}

	newValue := map[string]interface{}{"status": newStatus}
	if len(ontResults) > 0 {
		newValue["ont_results_count"] = len(ontResults)
	}
	s.logAudit(uuid.Nil, "update", "batch_job", jobID,
		map[string]interface{}{"status": current.Status},
		newValue,
	)

	return nil
}

// DeleteBatchJob removes a batch job permanently.
func (s *JobService) DeleteBatchJob(jobID uuid.UUID) error {
	job, err := s.GetBatchJob(jobID)
	if err != nil {
		return err
	}

	if err := s.db.Delete(&models.BatchJob{}, "id = ?", jobID).Error; err != nil {
		return fmt.Errorf("failed to delete batch job: %w", err)
	}

	s.logAudit(uuid.Nil, "delete", "batch_job", jobID,
		map[string]interface{}{"status": job.Status},
		nil,
	)
	return nil
}

// logAudit sends an audit log entry if an audit service is configured.
func (s *JobService) logAudit(
	userID uuid.UUID,
	action, resourceType string,
	resourceID uuid.UUID,
	oldValue, newValue map[string]interface{},
) {
	if s.audit == nil {
		return
	}

	_ = s.audit.Log(
		userID,
		action,
		resourceType,
		resourceID,
		oldValue,
		newValue,
		"",
		"",
	)
}
