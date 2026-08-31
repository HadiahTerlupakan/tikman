-- Migration 34 meant to create ont_trap_events, but GORM's AutoMigrate runs
-- first and had already built the table from the model — where its naming
-- strategy renders TrapOID as trap_o_id and ONUID as on_uid. The CREATE TABLE
-- IF NOT EXISTS then found the table present and did nothing, so the wrong
-- column names survived and the migration was recorded as applied.
--
-- The model now states both column names explicitly, so AutoMigrate cannot
-- reintroduce them.
--
-- Each rename is guarded. AutoMigrate runs before this file on every startup,
-- and once the model and the table agree it will have nothing to do — but an
-- unguarded ALTER would then fail on a column that is already correct, which is
-- how a repair turns into a permanent startup failure.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'ont_trap_events' AND column_name = 'trap_o_id') THEN
        ALTER TABLE ont_trap_events RENAME COLUMN trap_o_id TO trap_oid;
    END IF;

    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_name = 'ont_trap_events' AND column_name = 'on_uid') THEN
        ALTER TABLE ont_trap_events RENAME COLUMN on_uid TO onu_id;
    END IF;
END $$;

-- AutoMigrate also added a plain index on serial_number from the model's index
-- tag. Migration 34 builds a partial one over the same column that skips rows
-- with no serial; the plain one is paid for on every insert and read by nothing.
DROP INDEX IF EXISTS idx_ont_trap_events_serial_number;
