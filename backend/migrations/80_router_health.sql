-- The router's health sensors (PSU state, temperatures, fans, power) from
-- MIKROTIK-MIB's gauge table, so the faceplate's PWR and FAULT LEDs show real
-- data. jsonb rather than columns because each model reports a different set.
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS health jsonb;
ALTER TABLE mikrotik_routers ADD COLUMN IF NOT EXISTS health_at timestamptz;
