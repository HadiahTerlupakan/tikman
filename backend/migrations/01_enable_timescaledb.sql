-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Note: This migration runs before GORM AutoMigrate
-- GORM will create users, sites, olts tables
-- We only create monitoring-specific tables here

-- Create ONTs table (will be linked to olts after GORM migration)
CREATE TABLE IF NOT EXISTS onts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    olt_id UUID NOT NULL,
    port_id INT NOT NULL,
    ont_id INT NOT NULL,
    serial_number VARCHAR(20) NOT NULL,
    description VARCHAR(255),
    status VARCHAR(20),
    last_seen_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(olt_id, port_id, ont_id),
    UNIQUE(serial_number)
);

CREATE INDEX IF NOT EXISTS idx_onts_olt_id ON onts(olt_id);
CREATE INDEX IF NOT EXISTS idx_onts_status ON onts(status);
CREATE INDEX IF NOT EXISTS idx_onts_serial_number ON onts(serial_number);

-- Create Alarms table
CREATE TABLE IF NOT EXISTS alarms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    olt_id UUID,
    ont_id UUID,
    severity VARCHAR(20) NOT NULL,
    type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    metric_value DECIMAL(10,2),
    threshold_value DECIMAL(10,2),
    status VARCHAR(20) DEFAULT 'active',
    occurred_at TIMESTAMP NOT NULL,
    acknowledged_at TIMESTAMP,
    acknowledged_by UUID,
    cleared_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alarms_status ON alarms(status);
CREATE INDEX IF NOT EXISTS idx_alarms_occurred_at ON alarms(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_alarms_olt_id ON alarms(olt_id);
CREATE INDEX IF NOT EXISTS idx_alarms_ont_id ON alarms(ont_id);

-- Create Alert Rules table
CREATE TABLE IF NOT EXISTS alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    condition VARCHAR(20) NOT NULL,
    threshold DECIMAL(10,2) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    notify_email BOOLEAN DEFAULT FALSE,
    notify_webhook BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
