-- Posting updates to a WhatsApp Channel.
--
-- AutoMigrate creates both tables from the model tags and runs before this
-- file, so what is left here is everything a GORM tag cannot say — the same
-- split migration 41 describes for the rest of the CS module.

ALTER TABLE wa_channels DROP CONSTRAINT IF EXISTS wa_channels_role_valid;
ALTER TABLE wa_channels ADD CONSTRAINT wa_channels_role_valid
    CHECK (role IN ('owner', 'admin'));

-- One row per channel per number. The sync replaces an account's rows on every
-- pass; without this, a pass racing another would leave the same channel
-- listed twice in the picker and the sender would not know which to pick.
CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_channels_account_jid
    ON wa_channels (wa_account_id, jid);

-- CASCADE: the mirror belongs to the number. Deleting a number should take its
-- channel list with it rather than be refused over rows WhatsApp can rebuild
-- at any time.
ALTER TABLE wa_channels DROP CONSTRAINT IF EXISTS fk_wa_channels_account;
ALTER TABLE wa_channels ADD CONSTRAINT fk_wa_channels_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE CASCADE;

-- wa_channel_posts guarded on existing: migration 49 (added after this one
-- shipped) removes the WAChannelPost model and folds its table into
-- wa_broadcast_posts, so AutoMigrate no longer creates wa_channel_posts on a
-- database that has not run this migration yet. Everywhere this migration
-- already applied, the table is real and the guard is simply true.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'wa_channel_posts'
    ) THEN
        ALTER TABLE wa_channel_posts DROP CONSTRAINT IF EXISTS wa_channel_posts_status_valid;
        ALTER TABLE wa_channel_posts ADD CONSTRAINT wa_channel_posts_status_valid
            CHECK (status IN ('queued', 'sent', 'failed'));

        -- RESTRICT, matching cs_conversations in migration 41: deleting a
        -- number that has broadcast history is a mistake worth refusing, and
        -- it is what keeps that history readable.
        ALTER TABLE wa_channel_posts DROP CONSTRAINT IF EXISTS fk_wa_channel_posts_account;
        ALTER TABLE wa_channel_posts ADD CONSTRAINT fk_wa_channel_posts_account
            FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

        -- The drainer asks only for queued rows, and history grows without
        -- bound while the queue stays near empty. A partial index keeps that
        -- claim cheap.
        CREATE INDEX IF NOT EXISTS idx_wa_channel_posts_queued
            ON wa_channel_posts (wa_account_id, created_at)
            WHERE status = 'queued';
    END IF;
END $$;
