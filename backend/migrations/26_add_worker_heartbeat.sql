-- The API cannot observe the polling worker any other way: the two processes
-- share this database and nothing else. Without it, a dead worker leaves every
-- ONT status frozen while /health still reports the system healthy.
CREATE TABLE IF NOT EXISTS worker_heartbeats (
    name VARCHAR(64) PRIMARY KEY,
    beat_at TIMESTAMPTZ NOT NULL
);
