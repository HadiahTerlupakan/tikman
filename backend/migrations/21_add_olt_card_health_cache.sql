-- Per-slot temperature, CPU and memory, and the T-CONT profiles with their
-- bandwidths. Card health is read over SNMP; the bandwidths come from the CLI
-- read that already runs for the profile names.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS card_health JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS tcont_profile_details JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS onu_type_details JSONB;
