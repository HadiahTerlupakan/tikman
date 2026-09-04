-- Generalizing the channel outbox into a broadcast outbox.
--
-- AutoMigrate runs BEFORE this file — cmd/api/main.go:56 against :68 — and it
-- creates wa_broadcast_posts from the model tags. A RENAME would therefore
-- collide with a table that already exists, the migration would fail, and the
-- API would not start. So the history is copied into the table AutoMigrate
-- made, and the old table is retired.
--
-- Guarded on the old table existing, so a fresh database runs this as a no-op
-- rather than an error.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'wa_channel_posts'
    ) THEN
        INSERT INTO wa_broadcast_posts (
            id, wa_account_id, destination, destination_jid, sender_user_id,
            kind, body, media_path, media_mime, media_size, media_filename,
            status, fail_reason, wa_message_id, created_at, sent_at)
        SELECT
            id, wa_account_id, 'channel', channel_jid, sender_user_id,
            kind, body, media_path, media_mime, media_size, media_filename,
            status, fail_reason, wa_message_id, created_at, sent_at
        FROM wa_channel_posts
        ON CONFLICT (id) DO NOTHING;

        DROP TABLE wa_channel_posts;
    END IF;
END $$;

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_status_valid;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_status_valid
    CHECK (status IN ('queued', 'sent', 'failed'));

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_destination_valid;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_destination_valid
    CHECK (destination IN ('channel', 'status'));

-- The rule the whole design rests on: a channel post names its channel, a
-- status post has no target beyond the account already on the row. Enforced
-- here rather than remembered in code.
ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS wa_broadcast_posts_jid_matches_destination;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT wa_broadcast_posts_jid_matches_destination CHECK (
    (destination = 'channel' AND destination_jid IS NOT NULL)
    OR (destination = 'status' AND destination_jid IS NULL)
);

ALTER TABLE wa_broadcast_posts DROP CONSTRAINT IF EXISTS fk_wa_broadcast_posts_account;
ALTER TABLE wa_broadcast_posts ADD CONSTRAINT fk_wa_broadcast_posts_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_wa_broadcast_posts_queued
    ON wa_broadcast_posts (wa_account_id, created_at)
    WHERE status = 'queued';
