-- Rollback monitoring fields from onts table
ALTER TABLE onts DROP COLUMN IF EXISTS rx_power;
ALTER TABLE onts DROP COLUMN IF EXISTS tx_power;
ALTER TABLE onts DROP COLUMN IF EXISTS distance;
ALTER TABLE onts DROP COLUMN IF EXISTS last_online;
ALTER TABLE onts DROP COLUMN IF EXISTS last_offline;
ALTER TABLE onts DROP COLUMN IF EXISTS last_offline_reason;
ALTER TABLE onts DROP COLUMN IF EXISTS uptime;
ALTER TABLE onts DROP COLUMN IF EXISTS last_down_time_duration;
