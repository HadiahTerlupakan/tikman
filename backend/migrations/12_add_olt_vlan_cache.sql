-- Cache the OLT's VLAN table so the provisioning form can offer a list without
-- walking SNMP every time it opens. Refreshed by the discovery poll.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS vlans JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS vlans_updated_at TIMESTAMPTZ;
