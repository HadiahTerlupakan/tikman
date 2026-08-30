-- Nullable on purpose: an OLT whose location nobody has looked up yet must
-- still be registrable, and the map lists those separately rather than
-- pretending they are absent.
ALTER TABLE olts ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE olts ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;
