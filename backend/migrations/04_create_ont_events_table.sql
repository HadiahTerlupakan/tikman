-- Migration: Create ont_events table for logging ONT online/offline status changes
-- This table stores all status change events with timestamps, reasons, and duration

CREATE TABLE IF NOT EXISTS ont_events (
    id BIGSERIAL PRIMARY KEY,
    ont_id UUID NOT NULL,
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('online', 'offline')),
    event_time TIMESTAMP NOT NULL,
    reason TEXT,
    duration_seconds BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT ont_events_ont_id_fkey FOREIGN KEY (ont_id) REFERENCES onts(id) ON DELETE CASCADE
);

-- Index for fast lookup by ONT ID (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_ont_events_ont_id ON ont_events(ont_id);

-- Index for time-based queries (filtering by date range)
CREATE INDEX IF NOT EXISTS idx_ont_events_event_time ON ont_events(event_time DESC);

-- Composite index for ONT-specific time range queries
CREATE INDEX IF NOT EXISTS idx_ont_events_ont_time ON ont_events(ont_id, event_time DESC);

-- Index for event type filtering (online vs offline analysis)
CREATE INDEX IF NOT EXISTS idx_ont_events_type ON ont_events(event_type);

-- Comments for documentation
COMMENT ON TABLE ont_events IS 'Logs all ONT status change events (online/offline) with timestamps and reasons';
COMMENT ON COLUMN ont_events.ont_id IS 'Foreign key to onts table';
COMMENT ON COLUMN ont_events.event_type IS 'Type of event: online or offline';
COMMENT ON COLUMN ont_events.event_time IS 'When the event occurred (from SNMP or detected)';
COMMENT ON COLUMN ont_events.reason IS 'Reason for offline event (e.g., LOS, Dying-Gasp, etc.)';
COMMENT ON COLUMN ont_events.duration_seconds IS 'Duration in seconds (for offline events, how long it was offline)';
COMMENT ON COLUMN ont_events.created_at IS 'When this record was created in the database';
