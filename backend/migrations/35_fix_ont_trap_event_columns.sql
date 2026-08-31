-- Migration 34 meant to create ont_trap_events, but GORM's AutoMigrate runs
-- first and had already built the table from the model — where its naming
-- strategy renders TrapOID as trap_o_id and ONUID as on_uid. The CREATE TABLE
-- IF NOT EXISTS then found the table present and did nothing, so the wrong
-- column names survived and the migration was recorded as applied.
--
-- The model now states both column names explicitly, so AutoMigrate cannot
-- reintroduce them. This renames what it already made.
ALTER TABLE ont_trap_events RENAME COLUMN trap_o_id TO trap_oid;
ALTER TABLE ont_trap_events RENAME COLUMN on_uid TO onu_id;

-- AutoMigrate also added a plain index on serial_number from the model's index
-- tag. Migration 34 builds a partial one over the same column that skips rows
-- with no serial; the plain one is paid for on every insert and read by nothing.
DROP INDEX IF EXISTS idx_ont_trap_events_serial_number;
