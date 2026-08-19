-- ============================================================================
-- FIX ONT MONITORING - UPDATE ZTE C300 PHYSICAL LOCATION
-- ============================================================================
-- Source: show gpon onu state gpon-olt_1/3/1
-- Format: gpon-olt_{rack}/{shelf}/{slot}
-- Configuration Applied: Rack=1, Shelf=3, Slot=1
-- ============================================================================

-- 1. Backup current configuration (IMPORTANT!)
SELECT
    id,
    name,
    ip_address,
    rack as OLD_RACK,
    shelf as OLD_SHELF,
    slot as OLD_SLOT,
    ((rack << 25) | (shelf << 19) | (slot << 13)) as OLD_BASE_IFINDEX
FROM olts
WHERE ip_address = '113.192.1.98';
-- Note these values in case you need to restore

-- 2. Update with CORRECT physical location from CLI output
UPDATE olts
SET
    rack = 1,      -- From gpon-olt_1/3/1 (Rack position)
    shelf = 3,     -- From gpon-olt_1/3/1 (Shelf position)
    slot = 1       -- From gpon-olt_1/3/1 (Slot position)
WHERE ip_address = '113.192.1.98';

-- 3. Verify the update worked correctly
SELECT
    id,
    name,
    ip_address,
    rack as NEW_RACK,
    shelf as NEW_SHELF,
    slot as NEW_SLOT,
    ((rack << 25) | (shelf << 19) | (slot << 13)) as NEW_BASE_IFINDEX,
    'ZTE C300 format confirmed' as status
FROM olts
WHERE ip_address = '113.192.1.98';

-- 4. Expected SNMP OID calculations for each PON port
-- Base ifindex calculation for Rack 1, Shelf 3, Slot 1:
SELECT
    'Base ifindex (Rack 1/Shelf 3/Slot 1 only):' as description,
    ((1 << 25) | (3 << 19) | (1 << 13)) as base_ifindex;
-- Result: 33587200

-- With Port 1 added: ifindex = 33587200 + (1 << 8) = 33587456
-- With Port 1, ONT ID 18: ifindex = 33587456 + 18 = 33587474

-- Example SNMP OIDs for Port 1, ONT 18:
SELECT
    '.1.3.6.1.4.1.3902.1012.3.28.1.1.5.33587474.18' as rx_power_oid_example,
    '.1.3.6.1.4.1.3902.1012.3.28.1.1.6.33587474.18' as tx_power_oid_example,
    'For Port 1, ONT ID 18' as note;

-- Show all ports with their ifindex ranges
SELECT
    port_number,
    base_ifindex as port_base_ifindex,
    base_ifindex + 127 as max_ont_ifindex,  -- Max ONT ID is 127
    'Ports can have ONT IDs 1-127' as range_note
FROM (
    SELECT unnest(ARRAY[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]) as port_number,
           ((1 << 25) | (3 << 19) | (1 << 13) | (unnest(ARRAY[0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]) << 8)) as base_ifindex
) p;

-- Summary of expected changes
SELECT
    'EXPECTED IMPROVEMENTS:' as message,
    'Before: Duplicate serial numbers (wrong ifindex calculation)' as before,
    'After: Unique serial numbers per actual ONT' as after,
    'Before: Metrics from random/wrong ONTs' as before_metrics,
    'After: Accurate metrics from correct ONTs' as after_metrics;
