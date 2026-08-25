-- Keep ZTE card/slot/PON/ONU positions unique while preserving legacy ONTs without slot data.
ALTER TABLE onts DROP CONSTRAINT IF EXISTS onts_olt_id_port_id_ont_id_key;
DROP INDEX IF EXISTS uq_onts_olt_port_ont;
CREATE UNIQUE INDEX IF NOT EXISTS uq_onts_olt_slot_port_ont
    ON onts(olt_id, slot, port_id, ont_id)
    WHERE slot IS NOT NULL;
