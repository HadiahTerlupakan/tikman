-- Migration: Create provisioning tables (config_templates, provisioning_jobs, batch_jobs)
-- These tables power the Phase 1 provisioning system: reusable config templates
-- plus state machines for single-ONT and batch-ONT provisioning runs.

-- config_templates: reusable per-vendor config patterns.
-- config_fields stays schemaless jsonb because ZTE and HSGQ CLIs take different
-- shapes; per-vendor validation happens at apply time, not in the schema.
CREATE TABLE IF NOT EXISTS config_templates (
    id            UUID PRIMARY KEY,
    name          VARCHAR(255) NOT NULL UNIQUE,
    description   TEXT,
    vendor        VARCHAR(50) NOT NULL CHECK (vendor IN ('zte', 'hsgq')),
    config_fields JSONB,
    is_default    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Only one template may be the default for a given vendor. A partial index is
-- used (rather than a plain unique on vendor) so non-default templates don't
-- collide with each other.
CREATE UNIQUE INDEX IF NOT EXISTS uq_config_templates_one_default_per_vendor
    ON config_templates (vendor) WHERE is_default;

CREATE INDEX IF NOT EXISTS idx_config_templates_vendor ON config_templates(vendor);

COMMENT ON TABLE config_templates IS 'Reusable ONT configuration patterns, keyed by vendor';
COMMENT ON COLUMN config_templates.config_fields IS 'Vendor-specific config schema (ZTE vs HSGQ CLI shapes differ)';
COMMENT ON COLUMN config_templates.is_default IS 'At most one default template per vendor (enforced by partial unique index)';

-- provisioning_jobs: state machine for a single ONT provisioning run.
-- before_snapshot is captured before any change so a failed run can be rolled
-- back to the exact prior config; config_snapshot records what was sent.
CREATE TABLE IF NOT EXISTS provisioning_jobs (
    id              UUID PRIMARY KEY,
    ont_id          UUID NOT NULL REFERENCES onts(id) ON DELETE CASCADE,
    template_id     UUID REFERENCES config_templates(id) ON DELETE SET NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'running', 'success', 'failed', 'rolled_back')),
    config_snapshot JSONB,
    before_snapshot JSONB,
    error_message   TEXT,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at    TIMESTAMP
);

-- Two writers on the same ONT would interleave config lines and corrupt the
-- device. The partial unique index makes the second concurrent 'running' job
-- fail at insert instead of on the wire. GORM cannot express partial indexes,
-- which is why it lives here and not in the model.
CREATE UNIQUE INDEX IF NOT EXISTS uq_provisioning_jobs_running_ont
    ON provisioning_jobs (ont_id) WHERE status = 'running';

-- Hot lookup: all jobs for an ONT filtered by status (status transitions and
-- per-ONT history both hit this shape).
CREATE INDEX IF NOT EXISTS idx_provisioning_jobs_ont_status
    ON provisioning_jobs(ont_id, status);

COMMENT ON TABLE provisioning_jobs IS 'State machine for a single ONT provisioning run (pending/running/success/failed/rolled_back)';
COMMENT ON COLUMN provisioning_jobs.before_snapshot IS 'ONT config before the run, used to roll back on failure';
COMMENT ON COLUMN provisioning_jobs.config_snapshot IS 'Exact config payload sent to the ONT, kept for audit';

-- batch_jobs: state machine for a multi-ONT provisioning run.
-- ont_results is a jsonb map of ont_id -> per-ONT result, updated as the batch
-- progresses so a long run stays observable.
CREATE TABLE IF NOT EXISTS batch_jobs (
    id           UUID PRIMARY KEY,
    template_id  UUID NOT NULL REFERENCES config_templates(id) ON DELETE RESTRICT,
    ont_ids      UUID[] NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'running', 'success', 'failed', 'partial_rollback')),
    ont_results  JSONB,
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_batch_jobs_status ON batch_jobs(status);

COMMENT ON TABLE batch_jobs IS 'State machine for a multi-ONT provisioning run with rollback tracking';
COMMENT ON COLUMN batch_jobs.ont_ids IS 'ONTs targeted by this batch, in provisioning order';
COMMENT ON COLUMN batch_jobs.ont_results IS 'Map of ont_id to per-ONT result (status, error, provisioning_job_id), written as the batch runs';
