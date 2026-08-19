-- Migration: Add extended fields for ONT device info and metrics
-- Created: 2026-08-18
-- Purpose: Add device info to onts table and health/traffic metrics to ont_metrics table

-- Add device information fields to onts table
ALTER TABLE onts
ADD COLUMN IF NOT EXISTS device_type VARCHAR(100),
ADD COLUMN IF NOT EXISTS hardware_version VARCHAR(50),
ADD COLUMN IF NOT EXISTS software_version VARCHAR(50),
ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45),
ADD COLUMN IF NOT EXISTS mac_address VARCHAR(17);

-- Add health monitoring fields to ont_metrics table
ALTER TABLE ont_metrics
ADD COLUMN IF NOT EXISTS tx_bias_current NUMERIC(6,2);

-- Traffic statistics fields already exist (rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors)
-- These were created in migration 02_create_timeseries_tables.sql

-- Create indexes for new searchable fields
CREATE INDEX IF NOT EXISTS idx_onts_device_type ON onts(device_type);
CREATE INDEX IF NOT EXISTS idx_onts_ip_address ON onts(ip_address);

-- Comment the new columns for documentation
COMMENT ON COLUMN onts.device_type IS 'ONU device model/type from SNMP';
COMMENT ON COLUMN onts.hardware_version IS 'Hardware revision from SNMP';
COMMENT ON COLUMN onts.software_version IS 'Firmware version from SNMP';
COMMENT ON COLUMN onts.ip_address IS 'ONT management IP address';
COMMENT ON COLUMN onts.mac_address IS 'ONT MAC address';
COMMENT ON COLUMN ont_metrics.tx_bias_current IS 'Laser transmit bias current in mA';
