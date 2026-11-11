-- Graph reports may go to a Telegram chat as well as, or instead of, a
-- WhatsApp group.
--
-- AutoMigrate runs before this file: it has already created telegram_chats
-- and the new columns from the model tags, under the index names restated
-- here, and dropped NOT NULL from wa_group_id. Each statement is still written
-- to stand on its own against a database built from the SQL files alone.

CREATE TABLE IF NOT EXISTS telegram_chats (
    id uuid PRIMARY KEY,
    chat_id bigint NOT NULL,
    title varchar(255) NOT NULL DEFAULT '',
    type varchar(32) NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telegram_chats_chat_id ON telegram_chats (chat_id);

-- A Telegram-only report has no WhatsApp group. The unique index on it stays:
-- NULLs do not collide.
ALTER TABLE graph_reports ALTER COLUMN wa_group_id DROP NOT NULL;

ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS telegram_chat_id uuid;
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_reports_telegram_chat_id ON graph_reports (telegram_chat_id);

-- RESTRICT rather than the WhatsApp group's CASCADE: nothing deletes a chat
-- row, and one deleted by hand must not take a report with it unseen.
ALTER TABLE graph_reports DROP CONSTRAINT IF EXISTS fk_graph_reports_telegram_chat;
ALTER TABLE graph_reports ADD CONSTRAINT fk_graph_reports_telegram_chat
    FOREIGN KEY (telegram_chat_id) REFERENCES telegram_chats(id) ON DELETE RESTRICT;

ALTER TABLE graph_reports DROP CONSTRAINT IF EXISTS chk_graph_reports_destination;
ALTER TABLE graph_reports ADD CONSTRAINT chk_graph_reports_destination
    CHECK (wa_group_id IS NOT NULL OR telegram_chat_id IS NOT NULL);

-- Per destination, the day it last got the daily report. Every report so far
-- went to WhatsApp alone, so its last daily send was WhatsApp's.
ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS wa_sent_on date;
ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS telegram_sent_on date;
UPDATE graph_reports SET wa_sent_on = last_sent_on WHERE wa_sent_on IS NULL;
