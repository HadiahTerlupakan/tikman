-- Which side spoke last in a thread, so the inbox can show what is waiting on
-- a CS: a new chat nobody has answered, and a customer who wrote again after
-- their thread was closed, are the same fact under one rule.
--
-- AutoMigrate creates the column. Computing it from cs_messages on every
-- listing instead would get slower as threads pile up, and this is the view a
-- CS opens most.
UPDATE cs_conversations c
SET last_message_direction = m.direction
FROM (
    SELECT DISTINCT ON (conversation_id) conversation_id, direction
    FROM cs_messages
    ORDER BY conversation_id, wa_timestamp DESC, created_at DESC
) m
WHERE m.conversation_id = c.id
  AND (c.last_message_direction IS NULL OR c.last_message_direction = '');

CREATE INDEX IF NOT EXISTS idx_cs_conversations_awaiting
    ON cs_conversations(last_message_direction, last_message_at DESC);
