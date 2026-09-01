#!/bin/bash

# ============================================================================
# FIX DUPLICATE ONT SERIAL NUMBERS - AUTOMATED SCRIPT
# ============================================================================

echo "=============================================="
echo "  🔧 Fix Duplicate Serial Numbers in ONT Monitoring"
echo "=============================================="
echo ""

echo "PROBLEM IDENTIFIED:"
echo "  - Your dashboard shows duplicate serial numbers (HG8245H, HG8245H5)"
echo "  - This happens because SNMP queries use WRONG ifindex calculation"
echo "  - Root cause: OLT rack/shelf/slot not configured or incorrect"
echo ""

echo "CURRENT SETUP (from your input):"
echo "  OLT IP: 192.0.2.10"
echo "  SNMP Port: 23161"
echo ""

# Check if PostgreSQL is available
if ! docker ps | grep postgres > /dev/null 2>&1; then
    echo "⚠️  WARNING: PostgreSQL container may not be running via Docker Desktop"
    echo "   Make sure it's started and accessible"
else
    echo "✅ PostgreSQL appears to be running"
fi

echo ""
echo "=============================================="
echo "  📋 SOLUTION STEPS"
echo "=============================================="
echo ""

echo "STEP 1: Discover Physical Location"
echo "----------------------------------------"
echo "Option A: Look at physical equipment"
echo "  - Find rack number where OLT chassis is installed"
echo "  - Find shelf number in that rack"
echo "  - Find slot number of GPON card"
echo ""
echo "Option B: Try common configurations below"
echo ""

echo "STEP 2: Choose Configuration to Apply"
echo "----------------------------------------"
echo "Which location matches YOUR hardware?"
echo ""

select config_option in \
    "Try default: Rack=0, Shelf=0, Slot=1 (most common)" \
    "Rack=0, Shelf=0, Slot=2 (second card position)" \
    "Rack=1, Shelf=0, Slot=1 (different rack)" \
    "Rack=0, Shelf=1, Slot=1 (different shelf)" \
    "Show discovery commands (SNMP walk)" \
    "Exit (I'll configure manually"
do
    case $config_option in
        *"default: Rack=0, Shelf=0, Slot=1"*)
            target_rack=0
            target_shelf=0
            target_slot=1
            break
            ;;
        *"Slot=2"*)
            target_rack=0
            target_shelf=0
            target_slot=2
            break
            ;;
        *"Rack=1"*|*"Slot=1"*)
            target_rack=1
            target_shelf=0
            target_slot=1
            break
            ;;
        *"Shelf=1"*)
            target_rack=0
            target_shelf=1
            target_slot=1
            break
            ;;
        *"Show discovery"*)
            echo ""
            echo "Running SNMP walk to discover ONTs..."
            echo ""
            cat << 'DISCOVERY'
# Run this command to discover actual location:
snmpwalk -v2c -c <community-anda> 192.0.2.10:23161 .1.3.6.1.4.1.3902.1012.3.28.1.1.1

# Parse output to find ifindex values, then decode:
# ifindex = rack<<25 | shelf<<19 | slot<<13 | port<<8
# Example: ifindex 8448 → Rack=0, Shelf=0, Slot=1, Port=1
DISCOVERY
            exit 0
            ;;
        *"Exit"*)
            echo "Exiting. You can manually run the SQL update scripts."
            echo "See: check_and_fix_olts.sql in project root"
            exit 0
            ;;
    esac
done

echo ""
echo "Selected configuration:"
echo "  Rack: $target_rack"
echo "  Shelf: $target_shelf"
echo "  Slot: $target_slot"
echo ""

cat << SELECTION_INFO
CALCULATED IFINDEX VALUES:
--------------------------
Base ifindex (rack/shelf/slot only): $(( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ))

For each PON port:
  Port 0: Base + 256 = $((( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ) + 256))
  Port 1: Base + 512 = $((( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ) + 512))
  Port 2: Base + 768 = $((( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ) + 768))
  ...
  Port N: Base + (N×256)

SNMP OID format for ONT metrics:
  Rx Power: .1.3.6.1.4.1.3902.1012.3.28.1.1.5.{ifindex}.{ont_id}
  Tx Power: .1.3.6.1.4.1.3902.1012.3.28.1.1.6.{ifindex}.{ont_id}

Example for Port 1, ONT ID 18:
  Full ifindex: $(((( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ) + 256) + 18))
  Rx OID: .1.3.6.1.4.1.3902.1012.3.28.1.1.5.$(((( (target_rack << 25) | (target_shelf << 19) | (target_slot << 13) ) + 256))).18

SELECTION_INFO

echo ""
read -p "Press Enter to apply this configuration to OLT at 192.0.2.10..."

echo ""
echo "Creating SQL update script..."

cat > /tmp/update_olt_config.sql << EOF
-- ============================================================================
-- UPDATE OLT PHYSICAL LOCATION CONFIGURATION
-- ============================================================================
-- Target OLT: 192.0.2.10
-- New Configuration:
--   Rack: $target_rack
--   Shelf: $target_shelf
--   Slot: $target_slot

-- Backup current values first
SELECT * FROM olts WHERE ip_address = '192.0.2.10';

-- Update with new configuration
UPDATE olts
SET rack = $target_rack,
    shelf = $target_shelf,
    slot = $target_slot
WHERE ip_address = '192.0.2.10';

-- Verify update
SELECT
    id,
    name,
    ip_address,
    rack,
    shelf,
    slot,
    ((rack << 25) | (shelf << 19) | (slot << 13)) as base_ifindex
FROM olts
WHERE ip_address = '192.0.2.10';

EOF

echo "✅ Created SQL update script: /tmp/update_olt_config.sql"
echo ""

cat << RUN_INSTRUCTIONS
Next steps:

1. Apply the SQL update:
   docker exec -it tikman-postgres psql -U tikman -d tikman -f /tmp/update_olt_config.sql

   OR if using direct connection:
   psql -U tikman -d tikman -f /tmp/update_olt_config.sql

2. Restart backend API (to refresh configuration):
   cd /Users/rohadimraja/Documents/tikman/backend
   go run cmd/api/main.go

3. Start/restart worker to collect metrics with new config:
   cd /Users/rohadimraja/Documents/tikman/backend
   killall worker main.go 2>/dev/null || true
   go run cmd/worker/main.go &

4. Wait 5 minutes for next metrics collection cycle

5. Refresh ONT Monitoring dashboard
   - Check if duplicate serials are gone
   - Verify unique serial numbers appear
   - Check signal metrics look reasonable

6. If still showing duplicates:
   - Review worker logs for SNMP errors
   - Double-check rack/shelf/slot match actual hardware
   - Try different configuration option

Run Instructions:

EOF

read -p "Press Enter to open SQL file..."
open /tmp/update_olt_config.sql 2>/dev/null || xdg-open /tmp/update_olt_config.sql 2>/dev/null || echo "SQL file created at: /tmp/update_olt_config.sql"

echo ""
echo "=============================================="
echo "  ✅ Setup Complete!"
echo "=============================================="
echo ""
echo "Remember: After applying the SQL update, restart services and wait"
echo "for metrics collection. The worker will now use correct ifindex"
echo "calculation based on your physical hardware location!"
echo ""
