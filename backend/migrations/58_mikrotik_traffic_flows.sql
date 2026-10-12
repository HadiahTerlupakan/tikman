-- Traffic destinations from IPFIX (flowd). AutoMigrate builds the tables and
-- columns from internal/models/flow.go and mikrotik.go; this adds what tags
-- cannot express: checks, foreign keys, hypertables, rollups and retention.

ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_sampling integer NOT NULL DEFAULT 100;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS flow_last_seen_at timestamptz;

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_flow_sampling_check;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_flow_sampling_check
    CHECK (flow_sampling BETWEEN 1 AND 10000);

ALTER TABLE router_routed_prefixes DROP CONSTRAINT IF EXISTS router_routed_prefixes_router_fk;
ALTER TABLE router_routed_prefixes ADD CONSTRAINT router_routed_prefixes_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- Without its interface a routed block belongs to no POP; the next discovery
-- stores it again under whatever interface now holds the gateway.
ALTER TABLE router_routed_prefixes DROP CONSTRAINT IF EXISTS router_routed_prefixes_interface_fk;
ALTER TABLE router_routed_prefixes ADD CONSTRAINT router_routed_prefixes_interface_fk
    FOREIGN KEY (interface_id) REFERENCES router_interfaces(id) ON DELETE CASCADE;

ALTER TABLE router_flow_minute DROP CONSTRAINT IF EXISTS router_flow_minute_direction_check;
ALTER TABLE router_flow_minute ADD CONSTRAINT router_flow_minute_direction_check
    CHECK (direction IN ('download', 'upload'));

ALTER TABLE router_flow_top_ips DROP CONSTRAINT IF EXISTS router_flow_top_ips_direction_check;
ALTER TABLE router_flow_top_ips ADD CONSTRAINT router_flow_top_ips_direction_check
    CHECK (direction IN ('download', 'upload'));

-- No foreign keys from the flow tables, as for router_interface_metrics: a
-- cascading delete into compressed chunks is what TimescaleDB handles worst,
-- and every read filters by router_id.
SELECT create_hypertable('router_flow_minute', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

ALTER TABLE router_flow_minute SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'router_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Four days: past the daily rollup's three-day refresh window, so no refresh
-- reads compressed chunks.
SELECT add_compression_policy('router_flow_minute', INTERVAL '4 days', if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_minute', INTERVAL '7 days', if_not_exists => TRUE);

SELECT create_hypertable('router_flow_top_ips', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_top_ips', INTERVAL '7 days', if_not_exists => TRUE);

-- Real-time aggregates, as for the interface rollups: a read unions the
-- materialized buckets with raw minutes past the last refresh.
CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_5min
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes)::bigint AS bytes,
    SUM(packets)::bigint AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_5min',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_5min', INTERVAL '90 days', if_not_exists => TRUE);

-- Rollup rows grow with POPs x upstreams x ASNs on a shared VPS, so each
-- rollup is compressed once its refresh window (start_offset) is behind it.
ALTER MATERIALIZED VIEW router_flow_5min SET (timescaledb.compress = true);
SELECT add_compression_policy('router_flow_5min', INTERVAL '2 days', if_not_exists => TRUE);

CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_hourly
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes)::bigint AS bytes,
    SUM(packets)::bigint AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_hourly',
    start_offset => INTERVAL '6 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_flow_hourly', INTERVAL '2 years', if_not_exists => TRUE);

ALTER MATERIALIZED VIEW router_flow_hourly SET (timescaledb.compress = true);
SELECT add_compression_policy('router_flow_hourly', INTERVAL '7 days', if_not_exists => TRUE);

-- Kept forever: years of planning data at one row per day per key. A refresh
-- window must span at least two buckets, hence three days back to one.
-- Days are WIB days: the operator reads "per hari" in WIB, and a continuous
-- aggregate's definition cannot change once data has accumulated, so the zone
-- is fixed here rather than corrected later.
CREATE MATERIALIZED VIEW IF NOT EXISTS router_flow_daily
WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 day', time, 'Asia/Jakarta') AS bucket,
    router_id,
    pop_interface_id,
    upstream_interface_id,
    direction,
    asn,
    SUM(bytes)::bigint AS bytes,
    SUM(packets)::bigint AS packets
FROM router_flow_minute
GROUP BY bucket, router_id, pop_interface_id, upstream_interface_id, direction, asn
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_flow_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

ALTER MATERIALIZED VIEW router_flow_daily SET (timescaledb.compress = true);
SELECT add_compression_policy('router_flow_daily', INTERVAL '30 days', if_not_exists => TRUE);
