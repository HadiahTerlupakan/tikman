-- The line cards fitted to the OLT, read from its running config. A card with
-- no ONUs on it cannot be inferred from where ONUs live, so it is stored.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS cards JSONB;
