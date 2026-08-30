-- The inventory walk does not report a serial for every ONU it finds, so those
-- rows are stored with an empty one. A plain unique index treats "" as a value,
-- which let exactly one serial-less ONT exist across the whole table and
-- rejected every other with "ONT with serial number  already exists" — five
-- real subscribers, on two different OLTs, held out by one row.
DROP INDEX IF EXISTS idx_onts_serial_number;

CREATE UNIQUE INDEX IF NOT EXISTS idx_onts_serial_number
    ON onts(serial_number)
    WHERE serial_number <> '';
