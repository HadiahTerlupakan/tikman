-- What a group needs to read like WhatsApp: reactions, per-member receipts,
-- poll votes, profile photos, pins, stars, edits and deletions.
--
-- AutoMigrate creates the new tables and columns from the model tags before
-- this file runs; the columns are restated here so the versioned files say
-- what the schema is, and the constraints are what a tag cannot say.

ALTER TABLE wa_groups ADD COLUMN IF NOT EXISTS pinned_at timestamptz;
ALTER TABLE wa_group_participants ADD COLUMN IF NOT EXISTS push_name varchar(255) NOT NULL DEFAULT '';
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS sender_push_name varchar(255) NOT NULL DEFAULT '';
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS extra text NOT NULL DEFAULT '';
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS edited boolean NOT NULL DEFAULT false;
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS deleted boolean NOT NULL DEFAULT false;
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS forwarded boolean NOT NULL DEFAULT false;
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS starred_at timestamptz;
ALTER TABLE wa_group_messages ADD COLUMN IF NOT EXISTS member_count integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_wa_group_messages_starred
    ON wa_group_messages (group_id, starred_at DESC)
    WHERE starred_at IS NOT NULL;

-- The three per-message tables go with their message.
ALTER TABLE wa_group_reactions DROP CONSTRAINT IF EXISTS fk_wa_group_reactions_message;
ALTER TABLE wa_group_reactions ADD CONSTRAINT fk_wa_group_reactions_message
    FOREIGN KEY (message_id) REFERENCES wa_group_messages(id) ON DELETE CASCADE;

ALTER TABLE wa_group_receipts DROP CONSTRAINT IF EXISTS fk_wa_group_receipts_message;
ALTER TABLE wa_group_receipts ADD CONSTRAINT fk_wa_group_receipts_message
    FOREIGN KEY (message_id) REFERENCES wa_group_messages(id) ON DELETE CASCADE;

ALTER TABLE wa_group_poll_votes DROP CONSTRAINT IF EXISTS fk_wa_group_poll_votes_message;
ALTER TABLE wa_group_poll_votes ADD CONSTRAINT fk_wa_group_poll_votes_message
    FOREIGN KEY (message_id) REFERENCES wa_group_messages(id) ON DELETE CASCADE;

ALTER TABLE wa_avatars DROP CONSTRAINT IF EXISTS fk_wa_avatars_account;
ALTER TABLE wa_avatars ADD CONSTRAINT fk_wa_avatars_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE CASCADE;
