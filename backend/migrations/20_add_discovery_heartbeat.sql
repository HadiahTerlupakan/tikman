-- A discovery claim outlives the process holding it. Without a heartbeat the
-- only evidence of liveness is when the run started, so a run killed by a
-- restart held the OLT for the whole stale-claim window.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS discovery_heartbeat_at TIMESTAMPTZ;
