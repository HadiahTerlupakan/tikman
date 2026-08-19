-- ============================================================================
-- CHECK AND FIX OLT CONFIGURATION FOR CORRECT ONT MONITORING
-- ============================================================================

-- 1. Check current OLT configuration
SELECT
    id,
    name,
    ip_address,
    snmp_community,
    snmp_port,
    rack,
    shelf,
    slot,
    CONCAT(
        'ip:', ip_address, ' port:', CAST(snmp_port AS text),
        ' comm:', snmp_community,
        ' loc:R', CAST(rack AS text), '/S', CAST(shelf AS text), '/S', CAST(slot AS text)
    ) as config_summary
FROM olts
ORDER BY created_at DESC;

-- 2. Check what ONTs are assigned to which OLTs
SELECT
    o.name as olt_name,
    o.ip_address,
    o.rack,
    o.shelf,
    o.slot,
    ont.serial_number,
    ont.port_id,
    ont.ont_id,
    ont.status,
    -- Calculate the ifindex based on current OLT config
    ((o.rack << 25) | (o.shelf << 19) | (o.slot << 13) | (ont.port_id << 8)) as calculated_ifindex
FROM olts o
LEFT JOIN onts ont ON ont.olt_id = o.id
ORDER BY o.name, ont.port_id, ont.ont_id;

-- 3. If you have discovered the correct location, use this script to update:
-- Replace VALUES with your actual discovered location
/*
UPDATE olts
SET
    rack = X,      -- Replace with discovered rack (0-15)
    shelf = Y,     -- Replace with discovered shelf (0-7)
    slot = Z       -- Replace with discovered slot (0-31)
WHERE name = 'Your-OFT-Name';
*/

-- Example for your OLT at 113.192.1.98
-- Based on common ZTE C300 deployment, try these configurations:

-- Option 1: Rack 0, Shelf 0, Slot 1 (most common single card setup)
/*
UPDATE olts
SET rack = 0, shelf = 0, slot = 1
WHERE ip_address = '113.192.1.98';
*/

-- Option 2: If you know it's in different slot, update accordingly
/*
UPDATE olts
SET rack = X, shelf = Y, slot = Z
WHERE ip_address = '113.192.1.98';
*/

-- 4. After updating, verify changes
SELECT
    id,
    name,
    ip_address,
    rack,
    shelf,
    slot,
    ((rack << 25) | (shelf << 19) | (slot << 13)) as base_ifindex
FROM olts
WHERE ip_address = '113.192.1.98';
