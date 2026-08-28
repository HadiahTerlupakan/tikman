-- VLAN profile names in use on the OLT's ONUs. Read in the same CLI session as
-- the T-CONT profiles, so tcont_profiles_updated_at times both.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS vlan_profiles JSONB;
