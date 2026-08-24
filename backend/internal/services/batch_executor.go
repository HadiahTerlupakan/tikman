package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ontBatchOutcome pairs an ONT ID with its job result so results collected from
// concurrent goroutines can be keyed correctly regardless of completion order.
type ontBatchOutcome struct {
	ontID  uuid.UUID
	result OntJobResult
}

// BatchExecutor orchestrates parallel provisioning for multiple ONTs with strict
// all-or-nothing atomicity: any failure cancels remaining work and rolls back
// all previously succeeded ONTs.
type BatchExecutor struct {
	db              *gorm.DB
	provisioner     *OntProvisioningService
	jobService      *JobService
	snapshotService *SnapshotService
	logger          *zap.Logger
}

// NewBatchExecutor constructs a BatchExecutor instance.
func NewBatchExecutor(
	db *gorm.DB,
	provisioner *OntProvisioningService,
	jobService *JobService,
	snapshotSvc *SnapshotService,
	logger *zap.Logger,
) *BatchExecutor {
	return &BatchExecutor{
		db:              db,
		provisioner:     provisioner,
		jobService:      jobService,
		snapshotService: snapshotSvc,
		logger:          logger,
	}
}

// BatchConfig carries the parameters of one batch provisioning run.
type BatchConfig struct {
	TemplateID   *uuid.UUID             `json:"template_id"`
	ManualConfig map[string]interface{} `json:"manual_config"`
	UserID       uuid.UUID              `json:"user_id"`
	ONTIDs       []uuid.UUID            `json:"ont_ids"`
}

// BatchResult contains the final outcome of batch execution.
type BatchResult struct {
	Job        *models.BatchJob
	Succeeded  []string
	Failed     []string
	RolledBack []string
	Details    map[string]OntJobResult
}

// Execute launches parallel provisioning for all ONTs in the config with strict
// atomicity. On any failure it cancels remaining jobs and rolls back all
// succeeded ONTs.
func (e *BatchExecutor) Execute(ctx context.Context, config BatchConfig) (*BatchResult, error) {
	if len(config.ONTIDs) == 0 {
		return nil, fmt.Errorf("batch must contain at least one ONT")
	}
	if config.TemplateID == nil {
		return nil, fmt.Errorf("batch requires a template id")
	}

	// Step 1: Create batch job in PENDING state
	batchJob, err := e.jobService.CreateBatchJob(*config.TemplateID, config.ONTIDs, config.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch job: %w", err)
	}

	// Step 2: Transition to RUNNING
	if err := e.jobService.UpdateStatusBatch(batchJob.ID, models.BatchStatusRunning, nil); err != nil {
		return nil, fmt.Errorf("failed to update batch status to running: %w", err)
	}

	outcomes := e.runAll(ctx, config.ONTIDs, config)

	// Step 3: Enforce strict atomicity on any failure
	for _, oc := range outcomes {
		if oc.result.Status != "success" {
			e.logger.Warn("batch provisioning failed, triggering rollback",
				zap.String("batch_job_id", batchJob.ID.String()),
				zap.String("failed_ont_id", oc.ontID.String()))
			return e.handleBatchFailure(batchJob, outcomes)
		}
	}

	// Step 4: All succeeded — mark batch SUCCESS and persist per-ONT results
	details := outcomeMap(outcomes)
	if err := e.jobService.UpdateStatusBatch(batchJob.ID, models.BatchStatusSuccess, details); err != nil {
		return nil, fmt.Errorf("failed to update batch status to success: %w", err)
	}

	// Reload to get the persisted status
	updatedJob, err := e.jobService.GetBatchJob(batchJob.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload batch job: %w", err)
	}

	return &BatchResult{
		Job:       updatedJob,
		Succeeded: idStrings(config.ONTIDs),
		Details:   details,
	}, nil
}

// runAll provisions every ONT concurrently and waits for all goroutines.
// A failure cancels the shared context so in-flight provisioning aborts early.
func (e *BatchExecutor) runAll(ctx context.Context, ontIDs []uuid.UUID, config BatchConfig) []ontBatchOutcome {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	outcomes := make([]ontBatchOutcome, len(ontIDs))
	var wg sync.WaitGroup

	for i, ontID := range ontIDs {
		wg.Add(1)
		go func(idx int, id uuid.UUID) {
			defer wg.Done()
			result := e.provisionOneONT(ctx, id, config)
			outcomes[idx] = ontBatchOutcome{ontID: id, result: result}
			if result.Status != "success" {
				cancel()
			}
		}(i, ontID)
	}
	wg.Wait()

	return outcomes
}

// provisionOneONT provisions a single ONT and maps the outcome to a job result.
func (e *BatchExecutor) provisionOneONT(ctx context.Context, ontID uuid.UUID, config BatchConfig) OntJobResult {
	provisionConfig := ProvisionConfig{
		TemplateID:   config.TemplateID,
		ManualConfig: config.ManualConfig,
		Confirm:      true,
	}

	_, err := e.provisioner.ProvisionOnt(ctx, ontID, config.UserID, provisionConfig)
	if err != nil {
		return OntJobResult{Status: "failed", Error: err.Error()}
	}
	return OntJobResult{Status: "success"}
}

// handleBatchFailure processes a failed batch:
// 1. Rolls back every ONT whose provisioning succeeded
// 2. Marks the batch failed or partial_rollback depending on outcome
// 3. Returns the aggregated result for UI display
func (e *BatchExecutor) handleBatchFailure(batchJob *models.BatchJob, outcomes []ontBatchOutcome) (*BatchResult, error) {
	var succeeded, failed, rolledBack []string
	anyRollbackErr := false

	for _, oc := range outcomes {
		if oc.result.Status == "success" {
			if err := e.rollbackOntJob(oc.ontID); err != nil {
				anyRollbackErr = true
				e.logger.Error("rollback failed for succeeded ONT",
					zap.String("ont_id", oc.ontID.String()), zap.Error(err))
				failed = append(failed, oc.ontID.String())
				continue
			}
			rolledBack = append(rolledBack, oc.ontID.String())
		} else {
			failed = append(failed, oc.ontID.String())
		}
	}

	finalStatus := models.BatchStatusFailed
	if len(rolledBack) > 0 {
		finalStatus = models.BatchStatusPartialRollback
	}
	if anyRollbackErr {
		finalStatus = models.BatchStatusFailed
	}

	details := make(map[string]OntJobResult, len(outcomes))
	for _, oc := range outcomes {
		details[oc.ontID.String()] = oc.result
	}
	for _, ontIDStr := range rolledBack {
		details[ontIDStr] = OntJobResult{Status: "rolled_back"}
	}

	if err := e.jobService.UpdateStatusBatch(batchJob.ID, finalStatus, details); err != nil {
		e.logger.Error("failed to update batch status after failure",
			zap.String("batch_job_id", batchJob.ID.String()), zap.Error(err))
	}

	return &BatchResult{
		Job:        batchJob,
		Succeeded:  succeeded,
		Failed:     failed,
		RolledBack: rolledBack,
		Details:    details,
	}, nil
}

// rollbackOntJob restores an ONT using the latest provisioning job's before
// snapshot via the provisioning service's rollback path.
func (e *BatchExecutor) rollbackOntJob(ontID uuid.UUID) error {
	jobs, _, err := e.jobService.ListProvisioningJobsByONT(ontID, 1, 0)
	if err != nil {
		return fmt.Errorf("failed to list jobs for ONT %s: %w", ontID, err)
	}
	if len(jobs) == 0 {
		return fmt.Errorf("no provisioning job found for ONT %s rollback", ontID)
	}
	return e.provisioner.RollbackOnt(context.Background(), ontID, jobs[0].ID)
}

// GetBatchResult retrieves the current status and details of a batch job.
func (e *BatchExecutor) GetBatchResult(jobID uuid.UUID) (*models.BatchJob, error) {
	return e.jobService.GetBatchJob(jobID)
}

func outcomeMap(outcomes []ontBatchOutcome) map[string]OntJobResult {
	m := make(map[string]OntJobResult, len(outcomes))
	for _, oc := range outcomes {
		m[oc.ontID.String()] = oc.result
	}
	return m
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}
