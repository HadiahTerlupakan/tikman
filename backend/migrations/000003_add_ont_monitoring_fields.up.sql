-- Add monitoring fields to onts table
ALTER TABLE onts ADD COLUMN IF NOT EXISTS rx_power DECIMAL(10,2) DEFAULT 0;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS tx_power DECIMAL(10,2) DEFAULT 0;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS distance INTEGER DEFAULT 0;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS last_online TIMESTAMP WITH TIME ZONE;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS last_offline TIMESTAMP WITH TIME ZONE;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS last_offline_reason VARCHAR(255);
ALTER TABLE onts ADD COLUMN IF NOT EXISTS uptime BIGINT DEFAULT 0;
ALTER TABLE onts ADD COLUMN IF NOT EXISTS last_down_time_duration BIGINT DEFAULT 0;

-- Add comment for better documentation
COMMENT ON COLUMN onts.rx_power IS 'Receiving optical power in dBm';
COMMENT ON COLUMN onts.tx_power IS 'Transmitting optical power in dBm';
COMMENT ON COLUMN onts.distance IS 'GPON optical distance in meters';
COMMENT ON COLUMN onts.last_online IS 'Timestamp of last online status';
COMMENT ON COLUMN onts.last_offline IS 'Timestamp of last offline status';
COMMENT ON COLUMN onts.last_offline_reason IS 'Reason for last offline event';
COMMENT ON COLUMN onts.uptime IS 'Uptime in seconds';
COMMENT ON COLUMN onts.last_down_time_duration IS 'Last downtime duration in seconds';
