-- One row per OLT per polling tier, and the queue workers claim from.
--
-- The cycle used to walk every OLT on one timer, doing a full discovery walk of
-- each chassis every time. Measured on this installation that second walk cost
-- as much as the poll it accompanied: Depok spent 38s reading and another 36s
-- rediscovering, every minute, to learn what changes a few times a day.
--
-- Splitting the work into tiers with their own schedules is what makes minute
-- level status possible, and a claimable row is what lets more than one worker
-- share the chassis without two of them reading the same agent at once.
CREATE TABLE IF NOT EXISTS olt_poll_jobs (
    olt_id UUID NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    kind VARCHAR(16) NOT NULL,
    due_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by VARCHAR(64),
    locked_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_duration_ms BIGINT,
    last_error TEXT,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (olt_id, kind)
);

-- The claim reads due and unlocked rows in due order, which is the only query
-- that runs against this table on the hot path.
CREATE INDEX IF NOT EXISTS idx_olt_poll_jobs_due ON olt_poll_jobs (due_at);

-- Every OLT that already exists gets its three jobs, due immediately, so the
-- first cycle after this migration behaves like the one before it.
INSERT INTO olt_poll_jobs (olt_id, kind)
SELECT olts.id, tier.kind
FROM olts
CROSS JOIN (VALUES ('status'), ('metrics'), ('discovery')) AS tier(kind)
ON CONFLICT DO NOTHING;
