-- ============================================================================
-- BATCH UPDATE SCRIPT FOR OLT PHYSICAL LOCATION CONFIGURATION
-- ============================================================================
--
-- This script provides templates to update existing OLTs with rack/shelf/slot
-- values. Customize these queries based on your actual hardware setup.
--
-- IMPORTANT: Always backup your database before running updates!
-- Run SELECT * FROM olts; first to see current data
--
-- ============================================================================

-- ============================================================================
-- STEP 1: CHECK CURRENT STATUS
-- ============================================================================

-- See all OLTs with current location settings
SELECT id, name, ip_address, rack, shelf, slot
FROM olts
ORDER BY created_at DESC;

-- Count OLTs that need configuration
SELECT
    COUNT(*) as total_olts,
    COUNT(CASE WHEN rack = 0 AND shelf = 0 AND slot = 0 THEN 1 END) as default_location,
    COUNT(CASE WHEN rack > 0 OR shelf > 0 OR slot > 0 THEN 1 END) as configured
FROM olts;

-- ============================================================================
-- STEP 2: SET DEFAULT VALUES (OPTIONAL - SAFE STARTING POINT)
-- ============================================================================

-- Set ALL OLTs to default location (Rack 0, Shelf 0, Slot 0)
-- Use this if you're not sure about the physical location yet
UPDATE olts
SET rack = 0,
    shelf = 0,
    slot = 0
WHERE rack IS NULL OR shelf IS NULL OR slot IS NULL;

-- Verify the update
SELECT id, name, ip_address, rack, shelf, slot
FROM olts
ORDER BY name;

-- ============================================================================
-- STEP 3: CONFIGURE SPECIFIC OLTs
-- ============================================================================
--
-- Replace the examples below with YOUR actual OLT configurations
--
-- Example formats:
-- OLT-RACK1-SLOT1      -> Rack 1, Shelf 0, Slot 1
-- OLT-RACK2-SLOT3      -> Rack 2, Shelf 0, Slot 3
-- OLT-MAIN-FRONT       -> Rack 0, Shelf 0, Slot 1
-- OLT-BACKUP-BACK      -> Rack 0, Shelf 0, Slot 2
--

-- EXAMPLE 1: Update OLT by name pattern
-- Update OLTs matching specific naming convention
UPDATE olts
SET rack = 1,
    shelf = 0,
    slot = 1
WHERE name LIKE '%RACK1%';

-- EXAMPLE 2: Update multiple OLTs sequentially
-- You can run these one at a time and check after each

-- First OLT
UPDATE olts
SET rack = 0,
    shelf = 0,
    slot = 1
WHERE name = 'Your-OLT-Name-1';

-- Second OLT
UPDATE olts
SET rack = 0,
    shelf = 0,
    slot = 2
WHERE name = 'Your-OLT-Name-2';

-- Third OLT
UPDATE olts
SET rack = 0,
    shelf = 1,
    slot = 0
WHERE name = 'Your-OLT-Name-3';

-- EXAMPLE 3: Bulk update with different locations
-- If you have many OLTs, use numbered updates

-- OLTs in Rack 1
UPDATE olts
SET rack = 1, shelf = 0, slot = 1
WHERE name IN ('OLT1', 'OLT2', 'OLT3');

-- OLTs in Rack 2
UPDATE olts
SET rack = 2, shelf = 0, slot = 1
WHERE name IN ('OLT4', 'OLT5', 'OLT6');

-- OLTs in Rack 0, Shelf 1
UPDATE olts
SET rack = 0, shelf = 1, slot = 1
WHERE name IN ('OLT7', 'OLT8');

-- ============================================================================
-- STEP 4: VERIFICATION
-- ============================================================================

-- Final verification - all OLTs with their new settings
SELECT
    id,
    name,
    ip_address,
    rack,
    shelf,
    slot,
    CONCAT(rack, '/', shelf, '/', slot) as full_location
FROM olts
ORDER BY rack, shelf, slot, name;

-- Group by location
SELECT
    rack,
    shelf,
    slot,
    COUNT(*) as olt_count,
    ARRAY_AGG(name ORDER BY name) as olt_names
FROM olts
GROUP BY rack, shelf, slot
ORDER BY rack, shelf, slot;

-- ============================================================================
-- STEP 5: RESTORE POINT (IF NEEDED)
-- ============================================================================

-- If you want to create a restore point, export current state:
-- pg_dump -U tikman -t olts tikman > /tmp/olts_backup_before_update.sql

-- To restore from backup:
-- psql -U tikman -d tikman -f /tmp/olts_backup_before_update.sql

-- ============================================================================
-- QUICK REFERENCE TABLE
-- ============================================================================
--
-- Common rack/shelf/slot combinations for ZTE C300:
--
-- Location          | Rack | Shelf | Slot | Note
-- ------------------|------|-------|------|-------------------
-- Primary OLT       |   0  |   0   |   1  | Default position
-- Secondary OLT     |   0  |   0   |   2  | Backup position
-- Rack 1, Slot 1    |   1  |   0   |   1  | First rack
-- Rack 1, Slot 2    |   1  |   0   |   2  | Second card in rack 1
-- Shelf 1, Slot 0   |   0  |   1   |   0  | Different shelf
-- Multiple cards    |   0  |   0   | 1-N  | Sequential slots
--
-- Remember: PON port numbers are 0-15, not related to slot number!
-- Each OLT has 16 PON ports (port 0 to port 15)
--
-- ============================================================================
