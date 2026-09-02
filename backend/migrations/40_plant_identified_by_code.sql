-- A cabinet and a distribution box are called by their code, not by a separate
-- name. Keeping both meant two columns holding the same words, which drift.
--
-- Safe to drop outright: these tables were added one migration ago and hold no
-- rows anywhere, so nothing is being thrown away.
ALTER TABLE odcs DROP COLUMN IF EXISTS name;
ALTER TABLE odps DROP COLUMN IF EXISTS name;

-- The code is now the identity, so it has to be there.
UPDATE odcs SET code = 'ODC-' || left(id::text, 8) WHERE code IS NULL OR code = '';
UPDATE odps SET code = 'ODP-' || left(id::text, 8) WHERE code IS NULL OR code = '';

ALTER TABLE odcs ALTER COLUMN code SET NOT NULL;
ALTER TABLE odps ALTER COLUMN code SET NOT NULL;
