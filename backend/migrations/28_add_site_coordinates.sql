-- Nullable on purpose: existing sites have no coordinates, and a site that
-- cannot be placed on the map must still be a valid site.
ALTER TABLE sites ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE sites ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
