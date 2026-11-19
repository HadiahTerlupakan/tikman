-- A BRAS's latest snapshot (cores, pools, RADIUS servers, top users), kept
-- like health / health_at, and one sample a minute of what the page charts.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS bras jsonb;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS bras_at timestamptz;

CREATE TABLE IF NOT EXISTS router_bras_samples (
    router_id uuid NOT NULL,
    time timestamptz NOT NULL,
    sessions bigint NOT NULL,
    cpu_max smallint NOT NULL,
    cpu_avg smallint NOT NULL,
    radius_accepts bigint NOT NULL,
    radius_rejects bigint NOT NULL,
    radius_timeouts bigint NOT NULL,
    pool_min_free bigint,
    PRIMARY KEY (router_id, time)
);

ALTER TABLE router_bras_samples DROP CONSTRAINT IF EXISTS router_bras_samples_router_fk;
ALTER TABLE router_bras_samples ADD CONSTRAINT router_bras_samples_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- Raw rows only, no rollups: about 43k rows per BRAS a month.
SELECT create_hypertable('router_bras_samples', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_bras_samples', INTERVAL '30 days', if_not_exists => TRUE);
