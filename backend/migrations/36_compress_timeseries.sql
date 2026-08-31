-- Storage limits, measured rather than assumed.
--
-- At 930 ONTs across three OLTs the trap receiver records 256 traps a minute,
-- and ont_trap_events reached 118,771 rows and 92 MB in eleven hours. Scaled to
-- the hundreds of thousands of subscribers this system targets, that is roughly
-- 460 traps a second, 40 million rows and 32 GB a day, and 220 GB across the
-- seven-day retention window.
--
-- Two things made that unaffordable rather than merely large. The trap table was
-- an ordinary table swept with DELETE, which at that rate means deleting tens of
-- millions of rows a day and leaving the space behind. And ont_metrics, despite
-- the design assuming otherwise, had compression switched off entirely: its own
-- estimate goes from about 1 GB a day to 10.8 GB without it.

-- ont_trap_events becomes a hypertable so retention drops whole chunks instead
-- of deleting rows one by one.
--
-- The primary key has to carry the partitioning column, so it moves from id
-- alone to (received_at, id). Nothing reads a trap by id — the table is written
-- constantly and read in time-ordered windows per OLT — so this costs nothing.
-- Guarded on the hypertable not existing yet, because the second run of this
-- migration must not drop a key it already replaced.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.hypertables
        WHERE hypertable_name = 'ont_trap_events'
    ) THEN
        ALTER TABLE ont_trap_events DROP CONSTRAINT IF EXISTS ont_trap_events_pkey;
        ALTER TABLE ont_trap_events ADD PRIMARY KEY (received_at, id);
    END IF;
END $$;

-- One hour per chunk. Retention then drops an hour at a time, which is cheap and
-- keeps a dropped chunk small enough that the drop never blocks the writers.
SELECT create_hypertable('ont_trap_events', 'received_at',
    chunk_time_interval => INTERVAL '1 hour',
    migrate_data => TRUE,
    if_not_exists => TRUE);

-- Segmented by OLT because that is how the rows are read, and because a single
-- chassis repeats the same handful of notification OIDs and varbind shapes for
-- hours: grouping them is what makes the varbind text compress at all.
ALTER TABLE ont_trap_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'olt_id',
    timescaledb.compress_orderby = 'received_at DESC'
);

-- Six hours leaves recent traps uncompressed, which is the window anyone
-- correlating an outage against the poller actually reads.
SELECT add_compression_policy('ont_trap_events', INTERVAL '6 hours', if_not_exists => TRUE);

-- Replaces the sweeper the trap daemon ran on its own timer. A retention policy
-- drops chunks; the sweeper issued a DELETE over a growing table every hour.
SELECT add_retention_policy('ont_trap_events', INTERVAL '7 days', if_not_exists => TRUE);

-- ont_metrics: compression was never enabled, and its chunks span seven days.
-- At the target write rate one such chunk would hold a week of metrics in a
-- single uncompressed piece.
SELECT set_chunk_time_interval('ont_metrics', INTERVAL '1 day');

ALTER TABLE ont_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'ont_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Two days, not one: the hourly continuous aggregate refreshes on its own
-- schedule, and compressing a chunk it has not finished reading is how a
-- rollup ends up with a hole in it.
SELECT add_compression_policy('ont_metrics', INTERVAL '2 days', if_not_exists => TRUE);
