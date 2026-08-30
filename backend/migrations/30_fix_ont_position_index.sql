-- Migration 11 meant to create this index on (olt_id, slot, port_id, ont_id),
-- but GORM's AutoMigrate runs first and had already created the same NAME on
-- (slot, port_id, ont_id) — the model was missing priority:1 on OLTID. Its
-- CREATE UNIQUE INDEX IF NOT EXISTS then found the name taken and did nothing,
-- so the wrong index survived and was recorded as applied.
--
-- Dropping by name first is the whole point: IF NOT EXISTS cannot repair an
-- index that already exists under the right name with the wrong columns.
DROP INDEX IF EXISTS uq_onts_olt_slot_port_ont;

CREATE UNIQUE INDEX IF NOT EXISTS uq_onts_olt_slot_port_ont
    ON onts(olt_id, slot, port_id, ont_id)
    WHERE slot IS NOT NULL;
