-- An hourly rollup kept for a year, so reports can compare months and show a
-- subscriber's data usage. The 5-minute rollup stays at 30 days for detail.
--
-- The byte and packet columns are the OLT's lifetime Counter64 totals, so a
-- period's usage is last_rx_bytes at the end minus first_rx_bytes at the
-- start. Both ends of each bucket are kept so a report can span buckets
-- without reading the raw table, which is dropped after 7 days.

CREATE MATERIALIZED VIEW IF NOT EXISTS ont_metrics_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
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
    MIN(rx_bytes) AS first_rx_bytes,
    MAX(rx_bytes) AS last_rx_bytes,
    MIN(tx_bytes) AS first_tx_bytes,
    MAX(tx_bytes) AS last_tx_bytes,
    MIN(rx_packets) AS first_rx_packets,
    MAX(rx_packets) AS last_rx_packets,
    MIN(tx_packets) AS first_tx_packets,
    MAX(tx_packets) AS last_tx_packets,
    COUNT(*) AS samples
FROM ont_metrics
GROUP BY bucket, ont_id;

SELECT add_continuous_aggregate_policy('ont_metrics_hourly',
    start_offset => INTERVAL '6 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);

SELECT add_retention_policy('ont_metrics_hourly', INTERVAL '1 year', if_not_exists => TRUE);

CALL refresh_continuous_aggregate('ont_metrics_hourly', NULL, NULL);
