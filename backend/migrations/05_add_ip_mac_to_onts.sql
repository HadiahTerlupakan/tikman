-- Migration: Add IP and MAC address columns to onts table
-- Created: 2026-08-18

ALTER TABLE onts ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
ALTER TABLE onts ADD COLUMN IF NOT EXISTS mac_address VARCHAR(17);

-- Create indexes for quick lookups
CREATE INDEX IF NOT EXISTS idx_onts_ip_address ON onts(ip_address) WHERE ip_address IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_onts_mac_address ON onts(mac_address) WHERE mac_address IS NOT NULL;

COMMENT ON COLUMN onts.ip_address IS 'Management IP address of the ONT (supports IPv4 and IPv6)';
COMMENT ON COLUMN onts.mac_address IS 'MAC address of the ONT';
