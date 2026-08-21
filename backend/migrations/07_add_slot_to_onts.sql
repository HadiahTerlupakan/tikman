-- Add slot column to onts table to store SNMP slot information
-- This enables direct SNMP queries without walking entire OID table

ALTER TABLE onts ADD COLUMN IF NOT EXISTS slot INTEGER;

-- Create index for slot lookups
CREATE INDEX IF NOT EXISTS idx_onts_slot ON onts(slot);

-- Add comment
COMMENT ON COLUMN onts.slot IS 'SNMP slot number (e.g., slot 3 in gpon-onu_1/3/12:2)';
