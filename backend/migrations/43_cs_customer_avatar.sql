-- A customer's profile photo, so the inbox shows faces rather than a column of
-- identical placeholders.
--
-- AutoMigrate creates the columns from the model tags. What it cannot say is
-- how the refresh sweep finds its work: oldest check first, and the ones never
-- checked at all before any of them.
CREATE INDEX IF NOT EXISTS idx_cs_conversations_avatar_stale
    ON cs_conversations(avatar_checked_at ASC NULLS FIRST);
