#!/bin/bash

# Setup Script for ONT Monitoring Fix
# This script will help you configure OLT physical location settings

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="${SCRIPT_DIR}/backend"

echo "=============================================="
echo "  🚀 ONT MONITORING SETUP SCRIPT"
echo "=============================================="
echo ""

# Check if Docker is available
if ! docker ps > /dev/null 2>&1; then
    echo "⚠️  WARNING: Docker seems to be running in Desktop, not CLI"
    echo "   Make sure PostgreSQL and Redis are running via Docker Desktop"
else
    echo "✅ Docker is accessible from CLI"
fi

# Check if services are running
echo ""
echo "📊 Checking service status..."
docker ps | grep -E "(postgres|redis)" > /dev/null 2>&1 && echo "  ✅ PostgreSQL & Redis are running" || echo "  ⚠️  Services may need to be started via Docker Desktop"

# Database migration info
echo ""
echo "📝 NEXT STEPS:"
echo "---------------------------------------------------------"
echo "1. Start the backend API to run AutoMigrate:"
echo "   cd ${BACKEND_DIR}"
echo "   go run cmd/api/main.go"
echo ""
echo "2. The following changes will be applied automatically:"
echo "   - Adding 'rack' column to olts table (default: 0)"
echo "   - Adding 'shelf' column to olts table (default: 0)"
echo "   - Adding 'slot' column to olts table (default: 0)"
echo ""
echo "3. After startup, configure each OLT through the UI:"
echo "   - Go to OLT Management page"
echo "   - Click Edit on each OLT"
echo "   - Fill in Physical Location (Rack/Shelf/Slot)"
echo "   - Values should match your actual hardware setup"
echo ""
echo "4. For quick configuration, you can use SQL commands like:"
echo "   UPDATE olts SET rack = 0, shelf = 0, slot = 1 WHERE id = 'olt-uuid';"
echo ""
echo "5. Restart the worker to start collecting metrics:"
echo "   go run cmd/worker/main.go"
echo ""
echo "6. Verify metrics collection by checking logs:"
echo "   Look for: '[Worker] Collected metrics'"
echo ""
echo "---------------------------------------------------------"
echo ""

# Create a reference document
cat > "${SCRIPT_DIR}/ONT_PHYSICAL_LOCATION_REFERENCE.md" << 'EOF'
# ONT Physical Location Configuration Reference

## Overview
This document provides guidance on configuring physical location settings for ZTE C300 OLTs.

## Field Definitions

### Rack (0-15)
Number of the equipment rack where the OLT chassis is mounted.
- Format: Integer between 0 and 15
- Default: 0

### Shelf (0-7)
Number of the shelf/card cage within the rack.
- Format: Integer between 0 and 7
- Default: 0

### Slot (0-31)
Number of the slot where the GPON line card is installed.
- Format: Integer between 0 and 31
- Default: 0

## SNMP OID Calculation

For ZTE C300, the ifindex is calculated using this formula:

```
ifindex = (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)
```

Where:
- `rack` = 25-bit shift left
- `shelf` = 19-bit shift left
- `slot` = 13-bit shift left
- `port` = 8-bit shift left (PON port number 0-15)

### Examples

#### Example 1: OLT in Rack 0, Shelf 0, Slot 1
- Rack: 0
- Shelf: 0
- Slot: 1

Calculation:
```
ifindex = (0 << 25) | (0 << 19) | (1 << 13) | (port << 8)
        = 0 | 0 | 8192 | (port << 8)
        = 8192 + (port * 256)
```

For Port 0: ifindex = 8192
For Port 1: ifindex = 8448
For Port 2: ifindex = 8704

#### Example 2: OLT in Rack 1, Shelf 0, Slot 2
- Rack: 1
- Shelf: 0
- Slot: 2

Calculation:
```
ifindex = (1 << 25) | (0 << 19) | (2 << 13) | (port << 8)
        = 33554432 | 0 | 16384 | (port << 8)
        = 33570816 + (port * 256)
```

For Port 0: ifindex = 33570816
For Port 1: ifindex = 33571072
For Port 2: ifindex = 33571328

#### Example 3: OLT in Rack 8, Shelf 0, Slot 24 (Original Hardcoded Values)
- Rack: 8
- Shelf: 0
- Slot: 24

Calculation:
```
ifindex = (8 << 25) | (0 << 19) | (24 << 13) | (port << 8)
        = 67108864 | 0 | 196608 | (port << 8)
        = 67305472 + (port * 256)
```

For Port 0: ifindex = 67305472
For Port 1: ifindex = 67305728

## How to Find Your Rack/Shelf/Slot

### From Hardware Documentation
Check your data center documentation or network inventory system.

### From OLT CLI Access
If you have SSH/Telnet access to the OLT, you might find this information in:
```bash
show system rack
show system shelf
show board
```

### From Network Discovery
Run SNMP walk to discover ONTs:
```bash
snmpwalk -v2c -c public <OLT_IP> .1.3.6.1.4.1.3902.1012.3.28.1.1.1
```

The OID suffix contains ifindex which encodes rack/shelf/slot/port.

To decode:
- Port: `(ifindex >> 8) & 0x1F` (bits 8-12)
- Slot: `(ifindex >> 13) & 0x1F` (bits 13-17)
- Shelf: `(ifindex >> 19) & 0x7` (bits 19-21)
- Rack: `(ifindex >> 25) & 0xF` (bits 25+)

## Common Scenarios

### Single OLT per Rack
Most common deployment:
- Rack: 0 (or your specific rack number)
- Shelf: 0
- Slot: Varies based on PON card position

### Multiple OLTs in Same Rack
- Rack: Same for all OLTs
- Shelf: Different shelf numbers
- Slot: Different slot numbers

### OLT Chassis with Multiple Cards
If OLT has multiple GPON cards in different slots:
- Consider creating separate OLT entries for each card
- Or calculate properly based on physical slot numbering

## Configuration Steps

### Via Web UI
1. Login to TikMan dashboard
2. Navigate to OLT Management
3. Click Edit on desired OLT
4. Scroll to "Physical Location" section
5. Enter Rack, Shelf, Slot values
6. Click Update

### Via Database (for batch updates)
```sql
-- Set default values for all OLTs
UPDATE olts SET rack = 0, shelf = 0, slot = 0;

-- Update specific OLT
UPDATE olts
SET rack = 1, shelf = 0, slot = 2
WHERE name = 'OLT-RACK1-SLOT2';

-- Update all OLTs in specific rack
UPDATE olts
SET rack = 2
WHERE site_id = 'your-site-uuid';
```

## Verification

After configuration, verify by:

1. **Metrics Collection**: Worker should successfully query ONT metrics
   ```bash
   # Check worker logs
   tail -f /tmp/tikman-worker.log
   ```

2. **Successful Query**: Look for log entries like:
   ```
   [Worker] Collected metrics serial=ZTEG12345678 port=0 ont_id=5 rx_power=-18.5 tx_power=2.3
   ```

3. **UI Display**: Metrics should appear in ONT Detail Modal with reasonable values:
   - Rx Power: -25 to -10 dBm (good), -25 to -27 dBm (warning)
   - Tx Power: 0 to +4 dBm
   - Temperature: 0 to 50°C
   - Voltage: 3.0 to 3.7V
   - Distance: 0 to 20km

## Troubleshooting

### Issue: No metrics collected
**Solution:** Verify rack/shelf/slot values match actual hardware. If incorrect, SNMP queries return wrong data.

### Issue: Metrics show unusual values
**Solution:** Double-check ifindex calculation matches ZTE C300 format. Some vendors use different encoding.

### Issue: Timeout errors when collecting metrics
**Solution:** Check:
1. SNMP community string is correct
2. ONT is actually online and responding
3. Physical location values produce valid ifindex

### Issue: Different values than expected
**Solution:** Remember that some ONTs may report invalid data when they're offline or disconnected. Only trust metrics from online ONTs.

## Notes

- These fields are **optional** but highly recommended for accurate SNMP queries
- Default value is 0 if not configured
- Changing rack/shelf/slot requires re-collecting metrics
- Each OLT may have different location settings
- Document your rack/shelf/slot assignments in a spreadsheet for reference
EOF

echo "✅ Created reference document: ONT_PHYSICAL_LOCATION_REFERENCE.md"
echo ""

echo "=============================================="
echo "  📋 READY TO START"
echo "=============================================="
echo ""
echo "Please follow these steps:"
echo ""
echo "Step 1: Start Backend API (AutoMigrate databases)"
echo "----------------------------------------"
echo "cd ${BACKEND_DIR}"
echo "go run cmd/api/main.go"
echo ""
echo "Press Ctrl+C after you see 'Server started on :8080'"
echo ""
echo "Step 2: Configure OLTs via Web UI"
echo "----------------------------------------"
echo "Open http://localhost:8080"
echo "Go to OLT Management → Edit each OLT → Add Rack/Shelf/Slot"
echo ""
echo "Step 3: Start Worker Service"
echo "----------------------------------------"
echo "cd ${BACKEND_DIR}"
echo "go run cmd/worker/main.go"
echo ""
echo "This will start collecting metrics every 5 minutes"
echo ""
echo "Reference docs created:"
echo "  • ONT_PHYSICAL_LOCATION_REFERENCE.md"
echo "  • See this file for detailed instructions"
echo ""
echo "=============================================="
echo ""

read -p "Press Enter to continue..."
