-- ONU types the OLT will accept in a registration command. Read in the same
-- CLI session as the profiles, so tcont_profiles_updated_at times these too.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS onu_types JSONB;
