-- Silence tracking for router interfaces. AutoMigrate adds the columns from
-- the model; they are repeated here so the file stands on its own.
ALTER TABLE router_interfaces ADD COLUMN IF NOT EXISTS last_traffic_at timestamptz;
ALTER TABLE router_interfaces ADD COLUMN IF NOT EXISTS silent_since timestamptz;
ALTER TABLE router_interfaces ADD COLUMN IF NOT EXISTS zero_in_since timestamptz;
ALTER TABLE router_interfaces ADD COLUMN IF NOT EXISTS zero_in_polls integer NOT NULL DEFAULT 0;

-- An interface is only ever marked silent after it has carried traffic. A POP
-- that went silent before this column existed would otherwise never be
-- marked, so date each interface's last traffic from the samples already
-- stored (the raw table keeps seven days).
UPDATE router_interfaces i
SET last_traffic_at = m.last_traffic
FROM (
    SELECT interface_id, MAX(time) AS last_traffic
    FROM router_interface_metrics
    WHERE in_bps > 0
    GROUP BY interface_id
) m
WHERE m.interface_id = i.id AND i.last_traffic_at IS NULL;
