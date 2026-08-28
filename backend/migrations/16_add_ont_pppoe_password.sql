-- The subscriber's PPPoE password, encrypted with the same AES-256-GCM key as
-- the OLT credentials. Reconfiguring a service has to resend it, and an
-- operator who does not have it to hand would otherwise break the session.
ALTER TABLE onts ADD COLUMN IF NOT EXISTS pppoe_password TEXT;
