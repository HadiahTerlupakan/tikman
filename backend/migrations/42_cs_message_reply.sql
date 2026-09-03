-- A reply that quotes another message points at it instead of copying its text,
-- so there is never a second version of the same words to keep in step.
--
-- AutoMigrate creates the column from the model tag and runs before this file.
-- What it cannot say is that the target is another message, and what becomes of
-- a quote whose target retention has already swept away: the quote goes, the
-- reply itself stays. A reply that vanished with the message it answered would
-- take a CS's own words out of the thread.

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS cs_messages_reply_to_fk;
ALTER TABLE cs_messages ADD CONSTRAINT cs_messages_reply_to_fk
    FOREIGN KEY (reply_to_id) REFERENCES cs_messages(id) ON DELETE SET NULL;

-- Partial: quoting is the exception in a thread, so the index only carries the
-- rows that actually quote something.
CREATE INDEX IF NOT EXISTS idx_cs_messages_reply_to
    ON cs_messages(reply_to_id) WHERE reply_to_id IS NOT NULL;
