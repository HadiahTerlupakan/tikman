-- The 5-minute rollup left the byte and packet columns out because, when it was
-- written, they held an optical power reading rather than traffic. They are
-- real counters now, and the middle graph tier reads this view, so it needs
-- them to report data used over a period.
--
-- Recreating loses the materialised rows, which are rebuilt from the raw table
-- in the same statement. Only the 7 days the raw table keeps can be recovered;
-- the hourly rollup holds the longer history.

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
    MIN(rx_bytes) AS first_rx_bytes,
    MAX(rx_bytes) AS last_rx_bytes,
    MIN(tx_bytes) AS first_tx_bytes,
    MAX(tx_bytes) AS last_tx_bytes,
    COUNT(*) AS samples
FROM ont_metrics
GROUP BY bucket, ont_id;

SELECT add_continuous_aggregate_policy('ont_metrics_5min',
    start_offset => INTERVAL '2 hours',
    end_offset => INTERVAL '10 minutes',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

SELECT add_retention_policy('ont_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);

CALL refresh_continuous_aggregate('ont_metrics_5min', NULL, NULL);
