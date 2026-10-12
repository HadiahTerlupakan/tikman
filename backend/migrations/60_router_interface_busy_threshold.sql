-- Only inbound traffic of 64 kbit/s and up counts as traffic (silentBusyBps
-- in services/router_silence.go). Migration 59 counted any packet, so a
-- management VLAN or loopback that only ever trickles was dated as busy and
-- then marked silent between its few packets.
UPDATE router_interfaces i
SET last_traffic_at = m.last_busy
FROM (
    SELECT interface_id, MAX(time) AS last_busy
    FROM router_interface_metrics
    WHERE in_bps >= 64000
    GROUP BY interface_id
) m
WHERE m.interface_id = i.id;

-- An interface dated from samples still stored, none of them busy, rested on
-- a trickle alone. One dated before the raw table's seven days is left: its
-- traffic is no longer there to check.
UPDATE router_interfaces i
SET last_traffic_at = NULL,
    silent_since = NULL,
    zero_in_since = NULL,
    zero_in_polls = 0
WHERE i.last_traffic_at >= now() - interval '7 days'
  AND NOT EXISTS (
    SELECT 1 FROM router_interface_metrics m
    WHERE m.interface_id = i.id AND m.in_bps >= 64000
  );
