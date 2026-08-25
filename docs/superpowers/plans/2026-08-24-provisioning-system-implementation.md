# Provisioning System Implementation Plan

**Goal:** Implement ONT configuration provisioning system with hybrid template/manual input, single/batch modes, strict failure handling with automatic rollback, and persistent state tracking.

## Phase 1: Core Infrastructure (Week 1)

### Task 1.1: Database Schema Changes

**Files:**
- Create: `backend/internal/migrations/09_create_provisioning_tables.sql`
- Modify: `backend/internal/models/models.go` (add AutoMigrate calls)
- Create: `internal/models/config_template.go`
- Create: `internal/models/provisioning_job.go`
- Create: `internal/models/batch_job.go`

**Steps:**
```sql
CREATE TABLE config_templates (...);
CREATE TABLE provisioning_jobs (...);
CREATE INDEX idx_provisioning_jobs_ont_status ON provisioning_jobs(ont_id, status);
CREATE INDEX idx_batch_jobs_status ON batch_jobs(status);
CREATE UNIQUE INDEX unique_active_job_per_ont ON provisioning_jobs(ont_id) WHERE status = 'running';
```

Verify migration runs correctly:
```bash
cd backend && go run cmd/api/main.go --migrate-up  # or use existing migration runner
```

---

### Task 1.2: ConfigTemplate Model + Repository

**Files:**
- Create: `internal/services/config_template_service.go`
- Create: `internal/services/config_template_repository.go`
- Test: `internal/services/config_template_test.go`

**Implementation:**
```go
type ConfigTemplateRepository struct {
    db *gorm.DB
}

func (r *ConfigTemplateRepository) Create(ctx context.Context, template *models.ConfigTemplate) error
func (r *ConfigTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ConfigTemplate, error)
func (r *ConfigTemplateRepository) GetAll(ctx context.Context) ([]models.ConfigTemplate, error)
func (r *ConfigTemplateRepository) Update(ctx context.Context, template *models.ConfigTemplate) error
func (r *ConfigTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error
func (r *ConfigTemplateRepository) GetDefaultByVendor(ctx context.Context, vendor string) (*models.ConfigTemplate, error)
```

**Tests:** CRUD operations, default per vendor constraint, JSON validation.

---

### Task 1.3: ProvisioningJob + BatchJob Models + Services

**Files:**
- Create: `internal/services/provisioning_job_service.go`
- Create: `internal/services/batch_job_service.go`
- Test: `internal/services/provisioning_job_test.go`, `batch_job_test.go`

**Key methods:**
```go
type ProvisioningJobService struct {
    jobRepo   *ProvisioningJobRepository
    ontRepo   *ONTRepository
    auditLog  *AuditService
}

func (s *ProvisioningJobService) CreateForSingleOnT(...) *ProvisioningJob
func (s *ProvisioningJobService) CreateForBatch(...) *BatchJob
func (s *ProvisioningJobService) UpdateStatus(jobID uuid.UUID, newStatus string) error
func (s *ProvisioningJobService) MarkAsSuccess(jobID uuid.UUID, configSnapshot jsonb) error
func (s *ProvisioningJobService) MarkAsFailed(jobID uuid.UUID, errorMessage string) error
func (s *ProvisioningJobService) Rollback(jobID uuid.UUID) error  // Restore BeforeSnapshot
```

**State machine enforcement:**
- Status transitions validated in code
- "running" job per ONT enforced via DB unique index
- On failure → trigger Rollback method

---

### Task 1.4: Snapshot Capture Service

**Files:**
- Create: `internal/services/snapshot_service.go`
- Test: `internal/services/snapshot_service_test.go`

**Key methods:**
```go
func (s *SnapshotService) CaptureBeforeSnapshot(ontID uuid.UUID) (*models.ConfigSnapshot, error)
func (s *SnapshotService) CaptureAfterSnapshot(ontID uuid.UUID) (*models.ConfigSnapshot, error)
func (s *SnapshotService) Compare(before, after *models.ConfigSnapshot) []string  // List of changes detected
```

This is critical for rollback capability. Uses SNMP reads to capture current config state before pushing.

---

## Phase 2: CLI Integration (Week 2)

### Task 2.1: Extend TelnetExecutor with Write Commands

**Files:**
- Modify: `internal/connectivity/telnet.go` → add `ExecuteCommand()` function
- Create: `internal/connectivity/command_executor.go`

**New interface:**
```go
type CommandExecutor interface {
    ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error)
    BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error)
}

// Implementations:
type ZTECommandExecutor struct { /* SNMP-based SET */ }
type HSGQCommandExecutor struct { /* Telnet CLI */ }
```

**ZTE commands** (example):
```
config
opti-gpon
interface gpon 0/1
ont ontid X profile Y
commit
```

**HSGQ commands** (example):
```
configure terminal
interface gpon-oltport 0/1
ont create 1 serial XXXX service-profile Y VLAN Z
commit
```

Need to discover actual command syntax from each vendor's docs.

---

### Task 2.2: Vendor-Specific Provisioning Logic

**Files:**
- Create: `internal/services/zte_provisioner.go`
- Create: `internal/services/hsgq_provisioner.go`

**Each implements:**
```go
type OLTProvisioner interface {
    ApplyConfiguration(ont models.ONT, config map[string]string) error
    ValidateConfig(ont models.ONT, config map[string]string) bool
    ReadCurrentConfig(ont models.ONT) (map[string]string, error)
}
```

**Testing:** Mock OLT or test against real OLT in staging environment. Start with ZTE C300/C320 first (largest install base).

---

## Phase 3: Single ONT Provisioning Flow (Week 2)

### Task 3.1: Main Provisioning Service

**Files:**
- Create: `internal/services/ont_provisioning_service.go`

**Main flow:**
```go
func (s *ONTPROvisioningService) ProvisionOnt(ontID, userID uuid.UUID, templateID *uuid.UUID, manualConfig map[string]string) (*ProvisioningJob, error) {
    // 1. Capture BeforeSnapshot
    before, err := s.snapshot.CaptureBeforeSnapshot(ontID)
    
    // 2. Build config (template + manual overrides)
    config := s.buildConfig(templateID, manualConfig)
    
    // 3. Create ProvisioningJob (status=PENDING)
    job := s.job.CreateForSingleOnt(ontID, templateID, config, userID, before)
    
    // 4. Update status=RUNNING
    job.UpdateStatus("RUNNING")
    
    // 5. Execute provision (Telnet/SSH/SNMP)
    result, err := s.provisioner.ApplyConfiguration(ont, config)
    if err != nil {
        // 6a. Failure → mark FAILED, trigger rollback
        job.MarkAsFailed(err.Error())
        s.rollback.Rollback(job.ID)
        return job, err
    }
    
    // 6b. Success → update SUCCESS, verify config match
    after, _ := s.snapshot.CaptureAfterSnapshot(ontID)
    if !s.validate.Equal(config, after) {
        job.MarkAsFailed("config mismatch after push")
        s.rollback.Rollback(job.ID)
        return job, fmt.Errorf("config verification failed")
    }
    
    job.MarkAsSuccess(after)
    return job, nil
}
```

---

### Task 3.2: API Handler Layer

**Files:**
- Create: `internal/api/provision_handler.go`
- Modify: `internal/api/router.go` (register routes)

**Endpoints:**
```go
POST /api/v1/onts/:id/provision
```

Request body:
```json
{
  "template_id": "uuid-or-null",
  "manual_config": {"field": "value", ...},
  "confirm": true  // User confirmation flag
}
```

Response:
```json
{
  "job_id": "uuid",
  "status": "pending|running|success|failed|rolled_back",
  "message": "..."
}
```

---

## Phase 4: Batch Provisioning with Rollback (Week 3)

### Task 4.1: Parallel Job Executor

**Files:**
- Create: `internal/services/batch_executor.go`

**Parallel execution with atomicity:**
```go
func (e *BatchExecutor) Execute(batchID uuid.UUID, ontIDs []uuid.UUID, config map[string]string) {
    // Launch goroutines for each ONT
    results := make(chan OntJobResult, len(ontIDs))
    ctx, cancel := context.WithCancel(context.Background())
    
    for _, ontID := range ontIDs {
        go func(id uuid.UUID) {
            result := e.provisionOntSingle(id, config)
            results <- result
        }(ontID)
    }
    
    // Collect results
    for i := 0; i < len(ontIDs); i++ {
        result := <-results
        if result.Success {
            e.trackSuccess(result)
        } else {
            // STRICt MODE: any failure triggers rollback of all succeeded
            cancel()  // Cancel remaining jobs
            e.rollbackAllSucceeded(batchID)  // Rollback everything
            break
        }
    }
    
    // Finalize batch status
    e.finalizeBatch(batchID, allSuccess || partialSuccessWithRollback)
}
```

---

### Task 4.2: Rollback Engine

**Files:**
- Create: `internal/services/rollback_engine.go`

**Restore BeforeSnapshot:**
```go
func (e *RollbackEngine) RollbackToSnapshot(ontID uuid.UUID, snapshot *models.ConfigSnapshot) error {
    // Use same executor as apply config, just inverted parameters
    return e.executor.RestoreConfig(ontID, snapshot)
}
```

**Ensure idempotency:** Rollback should work even if called multiple times.

---

### Task 4.3: Batch Provisioning Handler & UI

**API endpoints:**
```go
POST /api/v1/batch-provision
GET /api/v1/batch-jobs/:id
```

Request body:
```json
{
  "template_id": "uuid",
  "ont_ids": ["uuid1", "uuid2", ...],
  "confirm": true
}
```

---

## Phase 5: Frontend Implementation (Week 3)

### Task 5.1: Config Templates CRUD UI

**Component:** `/frontend/src/presentation/pages/ConfigTemplatesPage.tsx`

Features:
- Table listing templates
- New/Edit modal with field builder
- Default template toggle
- Clone template button
- Delete with confirmation

---

### Task 5.2: Hybrid Provision Modal

**Component:** `/frontend/src/presentation/components/ProvisionOntModal.tsx`

Features:
- Template dropdown (loads form fields)
- All fields editable for manual override
- Preview section showing what will be pushed
- Server-side validation before submit
- Loading spinner + progress indicator
- Real-time job status polling every 2 seconds

---

### Task 5.3: Batch Provision Modal

**Component:** `/frontend/src/presentation/components/BatchProvisionModal.tsx`

Features:
- ONT list selection (search/filter/pagination)
- Template selection
- Count display ("Will provision N ONTs")
- Confirm action with warning banner
- Progress bar showing % complete
- Individual ONT status badges (✓ success, ✗ failed, ↺ rolling back)

---

### Task 5.4: Provision History Page

**Component:** `/frontend/src/presentation/pages/ProvisionHistoryPage.tsx`

Features:
- Filter by date range, status, operator
- Search by ONT serial/name
- Expand row to show Before/After snapshots
- Error message display
- Export CSV of history

---

## Phase 6: Testing & Documentation (Week 3)

### Task 6.1: Integration Tests

**Set up:**
1. Test OLT simulator (mock SNMP/Telnet responses) OR
2. Use staging environment with test ONTs

**Test scenarios:**
1. Successful single provision
2. Failed provision → verify rollback
3. Batch with all successes
4. Batch with one failure → verify all rollback
5. Concurrent provisioning on same ONT → verify only one runs at a time
6. Config drift detection (read-back fails)

---

### Task 6.2: API Documentation

Update OpenAPI spec (`backend/internal/api/swagger.yaml`) with new endpoints. Generate docs using Swagger UI or similar.

---

### Task 6.3: Operator Guide

Create documentation for users:
1. How to create config templates
2. How to provision single ONT
3. How to do batch provisioning
4. Understanding job status
5. Troubleshooting common errors

---

## Verification Checklist

Before marking tasks complete:

- [ ] All unit tests pass (`go test ./... -race`)
- [ ] Backend build succeeds (`go build ./...`)
- [ ] Frontend build succeeds (`npm run build`)
- [ ] No linting errors (`golangci-lint run`, `npm run lint`)
- [ ] Migration tested on fresh DB
- [ ] Rollback verified to restore state
- [ ] Concurrent provision lock works (2 users → only 1 runs)
- [ ] Audit logs recorded for all provisioning actions

## Rollout Strategy

1. **Phase 1 deployment**: Only infrastructure, no UI yet (enable/disable via feature flag)
2. **Phase 2+ rollout**: Enable for internal testing team first
3. **Production launch**: Gradual rollout, monitor error rates closely

## Success Criteria

- ✅ Single ONT provisioning completes in <5s average
- ✅ Batch (100 ONTs) completes in <30s average  
- ✅ 100% rollback success rate (no ONT left in inconsistent state)
- ✅ Zero data loss (all snapshots persisted)
- ✅ Audit trail for every change
- ✅ No breaking changes to existing functionality
