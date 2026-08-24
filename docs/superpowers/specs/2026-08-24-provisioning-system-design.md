# Provisioning System — Design Spec

**Date:** 2026-08-24  
**Goal:** Add ONT configuration provisioning to TikMan with hybrid template/manual input, single/batch modes, strict failure handling, automatic rollback, and persistent state tracking.

## Problem Statement

Current system only monitors ONT status. Administrators cannot push configuration changes to ONTs from the application UI. This is operationally painful: every OLT config change requires CLI access or vendor-specific tools. We need a provisioning system that is:

1. **User-friendly**: Template-based with manual override option
2. **Scalable**: Single ONT or batch provisioning modes  
3. **Safe**: Strict all-or-nothing failure handling with automatic rollback
4. **Persistent**: Config state survives restarts and network blips
5. **Auditable**: Every provision attempt logged with before/after state

## System Architecture

### Three-Layer Design

```
┌─────────────────────────────────────────────────────────────┐
│                    PRESENTATION LAYER                       │
│  ┌──────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ Config Templates │  │ Provision ONT   │  │ Batch       │ │
│  │ (List/CRUD)      │  │ (Single)        │  │ Provision   │ │
│  └──────────────────┘  └─────────────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    APPLICATION LAYER                        │
│  ┌──────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ ConfigService    │  │ ProvisionService│  │ BatchService│ │
│  │ - Templates CRUD │  │ - Single ONT    │  │ - Bulk ONTs │ │
│  │ - Validation     │  │ - Async polling │  │ - Rollback  │ │
│  └──────────────────┘  └─────────────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                    INFRASTRUCTURE LAYER                     │
│  ┌──────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │ ConfigRepository │  │ Provisioning    │  │ AuditLog    │ │
│  │ (DB ops)         │  │ Service (CLI)   │  │ (tracking)  │ │
│  └──────────────────┘  └─────────────────┘  └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Key Components

#### 1. Config Templates (Database Model)

Stores reusable configuration patterns:

```go
type ConfigTemplate struct {
    ID              uuid.UUID `gorm:"type:uuid;primary_key"`
    Name            string    `gorm:"type:varchar(100);not null;uniqueIndex"`
    Description     string    `gorm:"type:text"`
    Vendor          string    `gorm:"type:varchar(50);not null"` // zte, hsgq
    ConfigFields    datatypes.JSON `gorm:"type:jsonb"` // Flexible schema per vendor
    IsDefault       bool      `gorm:"default:false"` // At most one per vendor
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

#### 2. Provisioning Job (State Machine)

Tracks each provisioning operation:

```go
type ProvisioningJob struct {
    ID              uuid.UUID `gorm:"type:uuid;primary_key"`
    OntID           uuid.UUID `gorm:"type:uuid;not null;index"`
    TemplateID      *uuid.UUID `gorm:"type:uuid;index"` // NULL for manual config
    Status          string    `gorm:"type:varchar(20);not null"` // pending, running, success, failed, rolled_back
    ConfigSnapshot  datatypes.JSON `gorm:"type:jsonb"` // What was sent
    BeforeSnapshot  datatypes.JSON `gorm:"type:jsonb"` // What it was before (for rollback)
    ErrorMessage    string    `gorm:"type:text"`
    CreatedBy       uuid.UUID `gorm:"type:uuid"` // User who triggered
    CreatedAt       time.Time
    CompletedAt     *time.Time
}
```

#### 3. Batch Job (For Multiple ONTs)

```go
type BatchJob struct {
    ID             uuid.UUID `gorm:"type:uuid;primary_key"`
    TemplateID     uuid.UUID `gorm:"type:uuid;not null"`
    OntIDs         []uuid.UUID `gorm:"type:uuid[];not null"`
    Status         string    `gorm:"type:varchar(20);not null"` // pending, running, success, failed, partial_rollback
    OntResults     map[uuid.UUID]OntJobResult `gorm:"type:jsonb"` // Individual outcomes
    CreatedBy      uuid.UUID `gorm:"type:uuid"`
    CreatedAt      time.Time
    CompletedAt    *time.Time
}

type OntJobResult struct {
    OntID    uuid.UUID
    Status   string // success, failed, rolled_back
    Error    string
    ConfigSnapshot datatypes.JSON
    BeforeSnapshot datatypes.JSON
}
```

#### 4. Configuration Snapshot (Before/After)

Before every provisioning, capture current state:

```go
type ConfigSnapshot struct {
    OntID       uuid.UUID `json:"ont_id"`
    Timestamp   time.Time `json:"timestamp"`
    // Vendor-specific fields
    ZTE         *ZTEConfigSnapshot `json:"zte,omitempty"`
    HSGQ        *HSGQConfigSnapshot `json:"hsgq,omitempty"`
}

type ZTEConfigSnapshot struct {
    SerialNumber string
    Name         string
    Bandwidth    string
    VLAN         string
    ServiceMode  string
    // ... other fields
}

type HSGQConfigSnapshot struct {
    SerialNumber string
    PortConfig   string
    VLANConfig   string
    // ... other fields
}
```

### Core Logic

#### Provisioning Flow (Single ONT)

1. **Initiate**: User selects template or manually enters config
2. **Capture**: Store current ONT state as BeforeSnapshot
3. **Validate**: Check config against OLT capabilities + template schema
4. **Execute**: Push config via Telnet/SSH (vendor-specific)
5. **Verify**: Read back config, compare to what was pushed
6. **Persist**: If verified, mark job as success; if failed, trigger rollback
7. **Rollback**: If any step fails, restore BeforeSnapshot

#### Batch Flow

1. **Initiate**: User selects N ONTs + template
2. **Parallelize**: Start provisioning jobs for each ONT (goroutine)
3. **Monitor**: Watch all jobs, collect results
4. **Rollback on failure**: If ANY ONT fails, rollback ALL others
5. **Mark**: BatchJob status = "partial_rollback" if some succeeded before failure

#### Rollback Strategy

When a provisioning fails:

1. **Immediate**: Cancel remaining in-flight operations
2. **Partially complete ONTs**: Restore BeforeSnapshot for each
3. **Batch**: All succeeded ONTs get rolled back to previous state
4. **State tracking**: Mark job as "rolled_back" with error message

### CLI Integration (Vendor-Specific)

#### ZTE OLTs (SNMP SET + CLI)

- SNMP SET for simple parameters (bandwidth, VLAN)
- CLI for complex config (service profiles, ACLs)
- Via existing `telnet.go` with proper command sequencing

#### HSGQ OLTs (CLI Only)

- All configuration via CLI (vendor doesn't expose SNMP SET for all params)
- Command sequences via existing `TelnetTest` pattern extended for writes

#### New Command Interface

```go
type CommandResult struct {
    Success bool
    Output  string
    Error   string
    Duration time.Duration
}

type OLTCommander interface {
    ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error)
    BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error)
}

// Existing implementations:
type ZTECommander struct { /* SNMP-based */ }
type HSGQCommander struct { /* CLI-based */ }
```

### Database Schema Changes

```sql
-- New tables
CREATE TABLE config_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(100) NOT NULL UNIQUE,
    description text,
    vendor varchar(50) NOT NULL,
    config_fields jsonb,
    is_default boolean DEFAULT false,
    created_at timestamp DEFAULT now(),
    updated_at timestamp DEFAULT now()
);

CREATE TABLE provisioning_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    ont_id uuid NOT NULL REFERENCES onts(id),
    template_id uuid REFERENCES config_templates(id),
    status varchar(20) NOT NULL,
    config_snapshot jsonb,
    before_snapshot jsonb,
    error_message text,
    created_by uuid,
    created_at timestamp DEFAULT now(),
    completed_at timestamp,
    CONSTRAINT unique_active_job_per_ont UNIQUE (ont_id) WHERE status = 'running'
);

CREATE TABLE batch_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id uuid NOT NULL REFERENCES config_templates(id),
    ont_ids uuid[] NOT NULL,
    status varchar(20) NOT NULL,
    ont_results jsonb,
    created_by uuid,
    created_at timestamp DEFAULT now(),
    completed_at timestamp
);

CREATE INDEX idx_provisioning_jobs_ont_status ON provisioning_jobs(ont_id, status);
CREATE INDEX idx_batch_jobs_status ON batch_jobs(status);
```

### API Endpoints

```
GET    /api/v1/config-templates           # List all templates
POST   /api/v1/config-templates           # Create template
GET    /api/v1/config-templates/:id       # Get template detail
PUT    /api/v1/config-templates/:id       # Update template
DELETE /api/v1/config-templates/:id       # Delete template

POST   /api/v1/onts/:id/provision         # Single ONT provisioning
GET    /api/v1/onts/:id/provision         # Check job status
GET    /api/v1/provision-jobs             # List provisioning history

POST   /api/v1/batch-provision            # Batch provisioning
GET    /api/v1/batch-jobs/:id             # Batch job status/results
```

### Frontend Changes

#### New Components

1. **ConfigTemplatesPage**: CRUD for templates
2. **ProvisionOntModal**: Single ONT provisioning with hybrid input
3. **BatchProvisionModal**: Select ONTs + template + execute
4. **ProvisionHistoryPage**: View all jobs (with status, errors, rollback info)

#### Hybrid Input Design

- Template dropdown: pre-selected template (loads config fields)
- Manual override: all fields editable if user wants to tweak
- Preview: show what will be pushed before confirmation
- Validation: client-side + server-side

### State Machine

```
PENDING → RUNNING → SUCCESS
   ↓         ↓
   FAILED ← ROLLBACK_IN_PROGRESS → ROLLED_BACK
```

### Rollback on Failure

1. **ONT-level failure**:
   - Mark OntJobResult as "failed"
   - Rollback to BeforeSnapshot
   - Continue other ONTs in batch

2. **Batch-level failure** (strict mode):
   - One ONT fails → rollback ALL succeeded ONTs
   - Mark BatchJob as "failed"
   - Keep error details for debugging

### Audit Trail

Every provisioning action logged:

```go
type AuditLog {
    Action: "provision_ont" | "provision_batch" | "rollback"
    ResourceType: "ont" | "batch"
    ResourceID: uuid
    OldValue: beforeSnapshot
    NewValue: configSnapshot
    IPAddress: requester IP
    UserAgent: requester UA
}
```

### Testing Strategy

- **Unit tests**: Each service method (config validation, snapshot capture, rollback)
- **Integration tests**: Full flow with mock OLTs (or real OLT in test env)
- **Rollback tests**: Verify rollback actually restores state
- **Batch tests**: Mixed success/failure scenarios

## Implementation Phases

### Phase 1: Core Infrastructure (1-2 weeks)
1. ConfigTemplate CRUD + DB schema
2. ProvisioningJob state machine
3. Snapshot capture/restore logic
4. Single ONT provisioning (single vendor first: ZTE)

### Phase 2: CLI Integration (1 week)
1. Extend TelnetTest → command executor
2. Vendor-specific command builders
3. Config validation (before push)

### Phase 3: Batch + Rollback (1 week)
1. BatchJob model + parallel execution
2. Automatic rollback on failure
3. Batch UI (progress tracking)

### Phase 4: UI Polish + Docs (3-5 days)
1. Template management UI
2. Provisioning modal (hybrid input)
3. History/audit views

## Success Metrics

- ✅ Single ONT provisioning: <5s completion time
- ✅ Batch (100 ONTs): <30s completion time
- ✅ Rollback success rate: 100% (never leave ONT in bad state)
- ✅ Zero data loss: all config snapshots stored in DB
- ✅ Auditability: every change logged with user attribution

## Risk Mitigation

1. **Network failures**: Retry with exponential backoff, then rollback
2. **Concurrent provisioning**: Lock per ONT (only one running job per ONT)
3. **Partial batch success**: Track individual ONT results, rollback all-or-none
4. **OLT disconnection**: Check connectivity before provisioning, fail fast
5. **Config conflicts**: Validate template against OLT capabilities before push

## Future Enhancements

- Multi-vendor support (add more OLT brands)
- Scheduled provisioning (provision at specific time)
- Config drift detection (alert when OLT config differs from template)
- API for programmatic provisioning (for external tools)
