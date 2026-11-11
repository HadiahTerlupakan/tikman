-- Port down/up notices, triggered by LibreNMS's alert webhook, for the ports
-- the graph reports name.
--
-- AutoMigrate runs before this file: it has already created both tables and
-- the new column from the model tags, under the index names restated here.
-- Each statement still stands on its own against a database built from the
-- SQL files alone.

-- Opt-in, so the customer groups that have only ever had the morning graphs
-- stay silent until an admin turns notices on for them.
ALTER TABLE graph_reports ADD COLUMN IF NOT EXISTS notify_port_status boolean NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS port_link_states (
    id uuid PRIMARY KEY,
    device_id integer NOT NULL,
    if_name varchar(255) NOT NULL,
    status varchar(16) NOT NULL,
    changed_at timestamptz NOT NULL,
    created_at timestamptz,
    updated_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_port_link_states_port ON port_link_states (device_id, if_name);

ALTER TABLE port_link_states DROP CONSTRAINT IF EXISTS chk_port_link_states_status;
ALTER TABLE port_link_states ADD CONSTRAINT chk_port_link_states_status CHECK (status IN ('up', 'down'));

CREATE TABLE IF NOT EXISTS port_alert_destination (
    id uuid PRIMARY KEY,
    wa_group_id uuid,
    telegram_chat_id uuid,
    telegram_thread_id bigint,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz,
    updated_at timestamptz
);

-- One NOC destination per installation: every row indexes the same constant.
CREATE UNIQUE INDEX IF NOT EXISTS idx_port_alert_destination_single ON port_alert_destination ((true));

-- SET NULL rather than the reports' CASCADE or RESTRICT: deleting the NOC's
-- group or chat leaves the setting with one destination fewer, not gone, and
-- must not be blocked by it.
ALTER TABLE port_alert_destination DROP CONSTRAINT IF EXISTS fk_port_alert_destination_group;
ALTER TABLE port_alert_destination ADD CONSTRAINT fk_port_alert_destination_group
    FOREIGN KEY (wa_group_id) REFERENCES wa_groups(id) ON DELETE SET NULL;

ALTER TABLE port_alert_destination DROP CONSTRAINT IF EXISTS fk_port_alert_destination_telegram_chat;
ALTER TABLE port_alert_destination ADD CONSTRAINT fk_port_alert_destination_telegram_chat
    FOREIGN KEY (telegram_chat_id) REFERENCES telegram_chats(id) ON DELETE SET NULL;
