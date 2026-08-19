-- Update continuous aggregate to include packet and error statistics
-- This migration adds rx_packets, tx_packets, rx_errors, tx_errors to 5-minute rollups

-- Drop existing continuous aggregate
DROP MATERIALIZED VIEW IF EXISTS ont_metrics_5min CASCADE;

-- Recreate with traffic statistics
CREATE MATERIALIZED VIEW ont_metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    ont_id,
    AVG(rx_power) AS avg_rx_power,
    MIN(rx_power) AS min_rx_power,
    MAX(rx_power) AS max_rx_power,
    AVG(tx_power) AS avg_tx_power,
    AVG(temperature) AS avg_temperature,
    MAX(temperature) AS max_temperature,
    SUM(rx_bytes) AS total_rx_bytes,
    SUM(tx_bytes) AS total_tx_bytes,
    SUM(rx_packets) AS total_rx_packets,
    SUM(tx_packets) AS total_tx_packets,
    SUM(rx_errors) AS total_rx_errors,
    SUM(tx_errors) AS total_tx_errors
FROM ont_metrics
GROUP BY bucket, ont_id;

-- Restore retention policy
SELECT add_retention_policy('ont_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);
