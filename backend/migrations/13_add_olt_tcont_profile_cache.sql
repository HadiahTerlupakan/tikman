-- Cache the OLT's T-CONT profile names so the provisioning form can offer them
-- without opening a CLI session. Refreshed by the discovery poll.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS tcont_profiles JSONB;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS tcont_profiles_updated_at TIMESTAMPTZ;
