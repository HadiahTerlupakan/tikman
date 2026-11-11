-- A Telegram report may go into one topic of a forum supergroup. Customers
-- can each have a topic in one forum, so a chat may now carry a report per
-- topic, General counting as one.
--
-- AutoMigrate runs before this file: it has already added is_forum,
-- telegram_topics and telegram_thread_id from the model tags, under the index
-- names restated here. Each statement still stands on its own against a
-- database built from the SQL files alone.

ALTER TABLE telegram_chats ADD COLUMN IF NOT EXISTS is_forum boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS telegram_topics (
    id uuid PRIMARY KEY,
    telegram_chat_id uuid NOT NULL,
    thread_id bigint NOT NULL,
    name varchar(255) NOT NULL DEFAULT '',
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_telegram_topics_chat_thread ON telegram_topics (telegram_chat_id, thread_id);

-- CASCADE: the only chat row ever deleted is a supergroup's that gives way to
-- the upgraded group's row (TelegramChatService.Migrate); its topics go with
-- it and are learnt again under the row that stays.
ALTER TABLE telegram_topics DROP CONSTRAINT IF EXISTS fk_telegram_topics_chat;
ALTER TABLE telegram_topics ADD CONSTRAINT fk_telegram_topics_chat
    FOREIGN KEY (telegram_chat_id) REFERENCES telegram_chats(id) ON DELETE CASCADE;

ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS telegram_thread_id bigint;

-- One report per chat becomes one per (chat, topic). NULL is General, and is
-- folded to 0 so two General reports collide; Telegram's thread IDs are
-- message IDs, never 0. The new index leads with the chat, so it also serves
-- the lookups and the RESTRICT check the old one did.
DROP INDEX IF EXISTS idx_graph_reports_telegram_chat_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_graph_reports_telegram_destination
    ON graph_reports (telegram_chat_id, COALESCE(telegram_thread_id, 0))
    WHERE telegram_chat_id IS NOT NULL;
