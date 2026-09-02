-- The CS module: one WhatsApp number answered by many hands.
--
-- AutoMigrate creates these tables from the model tags and runs before this
-- file, so what is left here is everything a GORM tag cannot say: the enum
-- checks, the foreign keys, the full-text column search leans on, and the
-- partial indexes that make waiting rows cheap to find.

ALTER TABLE wa_accounts DROP CONSTRAINT IF EXISTS wa_accounts_status_valid;
ALTER TABLE wa_accounts ADD CONSTRAINT wa_accounts_status_valid
    CHECK (status IN ('disconnected', 'pairing', 'connected', 'banned'));

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS cs_conversations_status_valid;
ALTER TABLE cs_conversations ADD CONSTRAINT cs_conversations_status_valid
    CHECK (status IN ('unassigned', 'open', 'closed'));

-- A thread with nobody holding it is exactly a thread that is unassigned.
-- Without this an assignment could be cleared while the row still called itself
-- open, and the conversation would sit in no inbox while looking answered.
ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS cs_conversations_holder_matches_status;
ALTER TABLE cs_conversations ADD CONSTRAINT cs_conversations_holder_matches_status
    CHECK (status <> 'unassigned' OR assigned_user_id IS NULL);

-- RESTRICT on the account: deleting a number that still holds conversations is
-- a mistake worth refusing. SET NULL on the others: a CS can leave the company
-- and an ONT can be replaced without taking the customer's history with them.
ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_account;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_user;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_user
    FOREIGN KEY (assigned_user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE cs_conversations DROP CONSTRAINT IF EXISTS fk_cs_conversations_ont;
ALTER TABLE cs_conversations ADD CONSTRAINT fk_cs_conversations_ont
    FOREIGN KEY (ont_id) REFERENCES onts(id) ON DELETE SET NULL;

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS cs_messages_direction_valid;
ALTER TABLE cs_messages ADD CONSTRAINT cs_messages_direction_valid
    CHECK (direction IN ('in', 'out'));

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS cs_messages_status_valid;
ALTER TABLE cs_messages ADD CONSTRAINT cs_messages_status_valid
    CHECK (status IN ('queued', 'sent', 'delivered', 'read', 'failed'));

ALTER TABLE cs_messages DROP CONSTRAINT IF EXISTS fk_cs_messages_conversation;
ALTER TABLE cs_messages ADD CONSTRAINT fk_cs_messages_conversation
    FOREIGN KEY (conversation_id) REFERENCES cs_conversations(id) ON DELETE CASCADE;

-- Partial, because an outbound message carries no WhatsApp id until it is sent
-- and many may be waiting at once. What this does enforce is the rule that
-- matters: WhatsApp re-delivering an event must not store the message twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_cs_messages_wa_id ON cs_messages (wa_message_id)
    WHERE wa_message_id IS NOT NULL;

-- Full text over message bodies. At thousands of messages a day this table
-- passes a million rows within a year, where ILIKE crawls.
ALTER TABLE cs_messages ADD COLUMN IF NOT EXISTS tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', coalesce(body, ''))) STORED;
CREATE INDEX IF NOT EXISTS idx_cs_messages_tsv ON cs_messages USING GIN (tsv);

-- The sweeper asks only for messages still waiting; a partial index keeps that
-- question off the millions of rows that were sent long ago.
CREATE INDEX IF NOT EXISTS idx_cs_messages_queued ON cs_messages (created_at)
    WHERE status = 'queued';

-- Inbox ordering is always newest-first within one account.
CREATE INDEX IF NOT EXISTS idx_cs_conversations_recent
    ON cs_conversations (wa_account_id, last_message_at DESC);

-- One phone number belongs to one subscriber's ONT: two ONTs claiming it would
-- send the CS to the wrong house. Partial, because most ONTs have no number
-- recorded yet and empty values must not collide.
CREATE UNIQUE INDEX IF NOT EXISTS uq_onts_phone ON onts (phone)
    WHERE phone IS NOT NULL AND phone <> '';
