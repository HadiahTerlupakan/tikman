-- Each ONU's provisioned service as the OLT currently has it, so the configure
-- form can open pre-filled. Written by the discovery poll from the running
-- config; it holds no credentials beyond the PPPoE username.
ALTER TABLE onts ADD COLUMN IF NOT EXISTS service_config JSONB;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS service_config_at TIMESTAMPTZ;
