-- cs_waits: one row per customer waiting for an answer, recorded as it happens.
--
-- AutoMigrate creates the table from models.CSWait before this runs; this file
-- adds what model tags cannot say. There is deliberately no foreign key to
-- cs_conversations or wa_accounts: deleting a number deletes its threads, and a
-- wait must outlive that, both so the figures cannot be changed by deleting and
-- so the delete is not refused, as the broadcast history once refused it.

-- A thread waits on at most one thing at a time. A second open wait would be
-- answered twice and counted twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cs_waits_one_open_per_thread
    ON cs_waits (conversation_id) WHERE ended_at IS NULL;

ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_end_reason_valid;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_end_reason_valid
    CHECK (end_reason IS NULL OR end_reason IN ('replied', 'phone', 'closed', 'abandoned'));

ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_ended_together;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_ended_together
    CHECK ((ended_at IS NULL) = (end_reason IS NULL));

-- Who ended a wait, and with which message, follows from how it ended. A CASE
-- rather than ORed conditions: with end_reason NULL an OR chain evaluates to
-- NULL, and a CHECK passes on NULL.
ALTER TABLE cs_waits DROP CONSTRAINT IF EXISTS cs_waits_actor_matches_reason;
ALTER TABLE cs_waits ADD CONSTRAINT cs_waits_actor_matches_reason CHECK (
    CASE COALESCE(end_reason, 'open')
        WHEN 'replied' THEN ended_by IS NOT NULL AND reply_message_id IS NOT NULL
        WHEN 'phone' THEN ended_by IS NULL AND reply_message_id IS NOT NULL
        WHEN 'closed' THEN ended_by IS NOT NULL AND reply_message_id IS NULL
        ELSE ended_by IS NULL AND reply_message_id IS NULL
    END
);

-- The report reads waits by when they ended, and one CS's waits by who ended
-- them.
CREATE INDEX IF NOT EXISTS idx_cs_waits_ended_at ON cs_waits (ended_at);
CREATE INDEX IF NOT EXISTS idx_cs_waits_ended_by_ended_at ON cs_waits (ended_by, ended_at);
