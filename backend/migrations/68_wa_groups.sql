-- WhatsApp groups, kept apart from the customer inbox.
--
-- AutoMigrate creates the four tables from the model tags before this file
-- runs, so what is here is what a GORM tag cannot say, as in migrations 41
-- and 48.

CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_groups_account_jid
    ON wa_groups (wa_account_id, jid);

-- CASCADE, unlike cs_conversations: the group list is a mirror WhatsApp can
-- rebuild at any time, and CSPurgeService.DeleteAccount removes the files
-- the messages point at before the number goes.
ALTER TABLE wa_groups DROP CONSTRAINT IF EXISTS fk_wa_groups_account;
ALTER TABLE wa_groups ADD CONSTRAINT fk_wa_groups_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_wa_groups_activity
    ON wa_groups (last_message_at DESC NULLS LAST);

ALTER TABLE wa_group_participants DROP CONSTRAINT IF EXISTS fk_wa_group_participants_group;
ALTER TABLE wa_group_participants ADD CONSTRAINT fk_wa_group_participants_group
    FOREIGN KEY (group_id) REFERENCES wa_groups(id) ON DELETE CASCADE;

ALTER TABLE wa_group_participants DROP CONSTRAINT IF EXISTS wa_group_participants_role_valid;
ALTER TABLE wa_group_participants ADD CONSTRAINT wa_group_participants_role_valid
    CHECK (role IN ('member', 'admin', 'superadmin'));

CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_group_participants_group_jid
    ON wa_group_participants (group_id, jid);

ALTER TABLE wa_group_messages DROP CONSTRAINT IF EXISTS fk_wa_group_messages_group;
ALTER TABLE wa_group_messages ADD CONSTRAINT fk_wa_group_messages_group
    FOREIGN KEY (group_id) REFERENCES wa_groups(id) ON DELETE CASCADE;

ALTER TABLE wa_group_messages DROP CONSTRAINT IF EXISTS wa_group_messages_direction_valid;
ALTER TABLE wa_group_messages ADD CONSTRAINT wa_group_messages_direction_valid
    CHECK (direction IN ('in', 'out'));

ALTER TABLE wa_group_messages DROP CONSTRAINT IF EXISTS wa_group_messages_status_valid;
ALTER TABLE wa_group_messages ADD CONSTRAINT wa_group_messages_status_valid
    CHECK (status IN ('queued', 'sent', 'delivered', 'read', 'failed'));

-- Per group rather than global: a WhatsApp id is only unique within its chat,
-- and re-delivery on every reconnect must not store a message twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_group_messages_wa_id
    ON wa_group_messages (group_id, wa_message_id)
    WHERE wa_message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_wa_group_messages_history
    ON wa_group_messages (group_id, wa_timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_wa_group_messages_queued
    ON wa_group_messages (created_at)
    WHERE status = 'queued';

ALTER TABLE wa_group_actions DROP CONSTRAINT IF EXISTS fk_wa_group_actions_account;
ALTER TABLE wa_group_actions ADD CONSTRAINT fk_wa_group_actions_account
    FOREIGN KEY (wa_account_id) REFERENCES wa_accounts(id) ON DELETE CASCADE;

-- SET NULL: an action outlives the group it was about, so "left the group"
-- still reads as done after the row it named is gone.
ALTER TABLE wa_group_actions DROP CONSTRAINT IF EXISTS fk_wa_group_actions_group;
ALTER TABLE wa_group_actions ADD CONSTRAINT fk_wa_group_actions_group
    FOREIGN KEY (group_id) REFERENCES wa_groups(id) ON DELETE SET NULL;

ALTER TABLE wa_group_actions DROP CONSTRAINT IF EXISTS wa_group_actions_status_valid;
ALTER TABLE wa_group_actions ADD CONSTRAINT wa_group_actions_status_valid
    CHECK (status IN ('queued', 'done', 'failed'));

CREATE INDEX IF NOT EXISTS idx_wa_group_actions_queued
    ON wa_group_actions (wa_account_id, created_at)
    WHERE status = 'queued';
