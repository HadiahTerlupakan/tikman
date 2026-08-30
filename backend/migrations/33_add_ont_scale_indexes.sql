-- Indexes for the shapes the ONT table is actually read in at scale.
--
-- Each was chosen from an EXPLAIN against the live table rather than guessed.
-- At 930 rows every query here already runs in about a millisecond; these exist
-- because the plans are ones whose cost grows with the whole table, and this
-- system is being built for a network three hundred times larger.
--
-- Built CONCURRENTLY. The runner applies a migration statement by statement
-- rather than in a transaction, which is what makes that possible, and it means
-- these can be added to a populated table without holding writes out. At 930
-- rows it costs nothing either way; on the table this is being built for it is
-- the difference between a pause and an outage.

-- The worker pages an OLT's ONTs by id. Without olt_id and id together, that is
-- an index scan of the whole primary key filtering on olt_id — every page walks
-- the entire table.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_onts_olt_id_id ON onts (olt_id, id);

-- The ONT list orders by physical position. The unique index on these columns
-- is partial (WHERE slot IS NOT NULL) so it cannot serve rows whose card is not
-- yet known, and the planner falls back to sorting the whole table for every
-- page it hands out.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_onts_position_order ON onts (olt_id, slot, port_id, ont_id);

-- The dashboard's weakest-signal card reads the worst readings among ONTs that
-- are still up. A partial index holds only those rows and holds them in the
-- order the card wants them.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_onts_weak_signal ON onts (rx_power)
    WHERE status = 'online' AND rx_power IS NOT NULL;

-- Search matches a substring of the serial or the name. A leading wildcard
-- cannot use a btree at all, so this is the one index shape that helps.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_onts_serial_trgm ON onts USING gin (LOWER(serial_number) gin_trgm_ops);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_onts_name_trgm ON onts USING gin (LOWER(name) gin_trgm_ops);
