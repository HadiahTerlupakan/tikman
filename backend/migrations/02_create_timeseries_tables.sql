-- Create Notification Settings table
CREATE TABLE IF NOT EXISTS notification_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    email_enabled BOOLEAN DEFAULT FALSE,
    email_address VARCHAR(255),
    webhook_enabled BOOLEAN DEFAULT FALSE,
    webhook_url VARCHAR(500),
    webhook_type VARCHAR(20) DEFAULT 'generic',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id)
);

-- Create ONT Metrics hypertable
CREATE TABLE IF NOT EXISTS ont_metrics (
    time TIMESTAMPTZ NOT NULL,
    ont_id UUID NOT NULL,
    rx_power DECIMAL(6,2),
    tx_power DECIMAL(6,2),
    temperature DECIMAL(5,2),
    voltage DECIMAL(5,2),
    distance INTEGER,
    rx_bytes BIGINT,
    tx_bytes BIGINT,
    rx_packets BIGINT,
    tx_packets BIGINT,
    rx_errors BIGINT,
    tx_errors BIGINT
);

-- Backfill for existing deployments created before distance column existed
ALTER TABLE ont_metrics ADD COLUMN IF NOT EXISTS distance INTEGER;

-- Convert to hypertable
SELECT create_hypertable('ont_metrics', 'time', if_not_exists => TRUE);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_ont_metrics_ont_time ON ont_metrics(ont_id, time DESC);

-- Create continuous aggregate for 5-minute rollups
CREATE MATERIALIZED VIEW IF NOT EXISTS ont_metrics_5min
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
    SUM(tx_bytes) AS total_tx_bytes
FROM ont_metrics
GROUP BY bucket, ont_id;

-- Create continuous aggregate for 1-hour rollups
-- Note: Cannot time_bucket from continuous aggregate directly
-- Will be created manually via API if needed, or use raw query
-- Keeping this commented for reference:
--
-- CREATE MATERIALIZED VIEW IF NOT EXISTS ont_metrics_1hour
-- WITH (timescaledb.continuous) AS
-- SELECT
--     time_bucket('1 hour', time) AS bucket,
--     ont_id,
--     AVG(rx_power) AS avg_rx_power,
--     MIN(rx_power) AS min_rx_power,
--     MAX(rx_power) AS max_rx_power,
--     AVG(temperature) AS avg_temperature,
--     MAX(temperature) AS max_temperature,
--     SUM(rx_bytes) AS total_rx_bytes,
--     SUM(tx_bytes) AS total_tx_bytes
-- FROM ont_metrics
-- GROUP BY bucket, ont_id;

-- Set data retention policies
SELECT add_retention_policy('ont_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('ont_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);
