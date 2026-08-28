-- The 5-minute rollup was created with a retention policy but never a refresh
-- policy, so TimescaleDB never materialised it: the view has been empty since
-- it was added. The raw table is dropped after 7 days, so nothing survived a
-- week and there was no history to report on.
--
-- Only metrics whose source OID is confirmed are rolled up. rx_packets reads
-- the same OID as rx_power (.3.50.12.1.1.10) and the byte columns are
-- documented in snmp_constants.go as oscillating fragments, so carrying them
-- into a reporting view would give those numbers a credibility they have not
-- earned.

DROP MATERIALIZED VIEW IF EXISTS ont_metrics_5min CASCADE;

CREATE MATERIALIZED VIEW IF NOT EXISTS ont_metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    ont_id,
    AVG(rx_power) AS avg_rx_power,
    MIN(rx_power) AS min_rx_power,
    MAX(rx_power) AS max_rx_power,
    AVG(tx_power) AS avg_tx_power,
    MIN(tx_power) AS min_tx_power,
    MAX(tx_power) AS max_tx_power,
    AVG(temperature) AS avg_temperature,
    MAX(temperature) AS max_temperature,
    AVG(rx_rate_mbps) AS avg_rx_mbps,
    MAX(rx_rate_mbps) AS max_rx_mbps,
    AVG(tx_rate_mbps) AS avg_tx_mbps,
    MAX(tx_rate_mbps) AS max_tx_mbps,
    COUNT(*) AS samples
FROM ont_metrics
GROUP BY bucket, ont_id;

-- Without this the view is never filled. end_offset stays behind the newest
-- data so a bucket is only materialised once the poll has finished writing to
-- it; start_offset covers a gap left by a restart.
SELECT add_continuous_aggregate_policy('ont_metrics_5min',
    start_offset => INTERVAL '2 hours',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

SELECT add_retention_policy('ont_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);

-- Backfill from the raw data still inside its 7-day window, so reporting does
-- not start from empty.
CALL refresh_continuous_aggregate('ont_metrics_5min', NULL, NULL);
