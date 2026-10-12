-- MikroTik border routers. AutoMigrate builds the tables from
-- internal/models/mikrotik.go; this adds what tags cannot express: foreign
-- keys, the hypertable and its rollups.

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_site_fk;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_site_fk
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL;

ALTER TABLE router_interfaces DROP CONSTRAINT IF EXISTS router_interfaces_router_fk;
ALTER TABLE router_interfaces ADD CONSTRAINT router_interfaces_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_bgp_sessions DROP CONSTRAINT IF EXISTS router_bgp_sessions_router_fk;
ALTER TABLE router_bgp_sessions ADD CONSTRAINT router_bgp_sessions_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- A session whose interface disappears in discovery keeps its row; the next
-- match fills the column again.
ALTER TABLE router_bgp_sessions DROP CONSTRAINT IF EXISTS router_bgp_sessions_interface_fk;
ALTER TABLE router_bgp_sessions ADD CONSTRAINT router_bgp_sessions_interface_fk
    FOREIGN KEY (interface_id) REFERENCES router_interfaces(id) ON DELETE SET NULL;

ALTER TABLE router_bgp_events DROP CONSTRAINT IF EXISTS router_bgp_events_router_fk;
ALTER TABLE router_bgp_events ADD CONSTRAINT router_bgp_events_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_poll_jobs DROP CONSTRAINT IF EXISTS router_poll_jobs_router_fk;
ALTER TABLE router_poll_jobs ADD CONSTRAINT router_poll_jobs_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- No foreign key from the metrics to router_interfaces. A cascading delete into
-- compressed chunks is the expensive path TimescaleDB handles worst, and an
-- orphaned sample is harmless: every read filters by interface_id, and
-- retention drops it within the week.
SELECT create_hypertable('router_interface_metrics', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

ALTER TABLE router_interface_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'interface_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Two days, as for ont_metrics: the rollups refresh from the last hours, and
-- compressing what they still read would slow every refresh.
SELECT add_compression_policy('router_interface_metrics', INTERVAL '2 days', if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics', INTERVAL '7 days', if_not_exists => TRUE);

-- Real-time aggregates: a read unions the materialized buckets with raw rows
-- past the last refresh, so the right edge of a graph is not up to an hour
-- behind the per-minute samples it rolls up.
CREATE MATERIALIZED VIEW IF NOT EXISTS router_interface_metrics_5min
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    interface_id,
    AVG(in_bps) AS avg_in_bps,
    MAX(in_bps) AS max_in_bps,
    AVG(out_bps) AS avg_out_bps,
    MAX(out_bps) AS max_out_bps
FROM router_interface_metrics
GROUP BY bucket, interface_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_interface_metrics_5min',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);

CREATE MATERIALIZED VIEW IF NOT EXISTS router_interface_metrics_hourly
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    interface_id,
    AVG(in_bps) AS avg_in_bps,
    MAX(in_bps) AS max_in_bps,
    AVG(out_bps) AS avg_out_bps,
    MAX(out_bps) AS max_out_bps
FROM router_interface_metrics
GROUP BY bucket, interface_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_interface_metrics_hourly',
    start_offset => INTERVAL '6 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics_hourly', INTERVAL '1 year', if_not_exists => TRUE);
