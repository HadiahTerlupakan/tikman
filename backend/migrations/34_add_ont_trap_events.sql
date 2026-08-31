-- Traps the OLTs send, kept so their meaning can be learned from evidence.
--
-- The notification OIDs a ZTE C300 sends are not documented in this repository,
-- and the arcs published for other ZTE hardware turned out to be a different MIB
-- module than this chassis actually uses. What a trap means therefore has to be
-- established by correlating it against status changes the poller recorded
-- independently, over hours rather than minutes — which container logs, being
-- rotated, cannot support.
--
-- Cariu alone sends about 200 traps a minute, so this table is written to
-- constantly and read rarely. It carries no foreign key to onts: a trap names an
-- ONU by serial, and one that arrives for an ONU not yet registered is exactly
-- the kind of event worth keeping.
CREATE TABLE IF NOT EXISTS ont_trap_events (
    id UUID PRIMARY KEY,
    olt_id UUID NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    trap_oid VARCHAR(128) NOT NULL,
    source_address VARCHAR(45) NOT NULL,
    -- Identity as the trap itself reported it, not as we resolved it. A trap
    -- naming an ONU we cannot find is evidence, and rewriting it into our own
    -- terms would destroy that.
    serial_number VARCHAR(32),
    onu_label VARCHAR(64),
    onu_name VARCHAR(255),
    if_index BIGINT,
    onu_id INTEGER,
    varbinds TEXT NOT NULL
);

-- The correlation this table exists for reads a window of one OLT's traps in
-- time order.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ont_trap_events_olt_time
    ON ont_trap_events (olt_id, received_at DESC);

-- Asking what a given ONU has been reporting is the other half of that
-- correlation.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ont_trap_events_serial
    ON ont_trap_events (serial_number, received_at DESC)
    WHERE serial_number IS NOT NULL;
