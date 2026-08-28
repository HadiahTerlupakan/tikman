-- The chassis summary and physical port inventory, both read over standard
-- SNMP MIBs during the discovery poll. Caching them keeps the OLT
-- configuration page free of live walks.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS system_info JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS ports JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS system_updated_at TIMESTAMPTZ;
