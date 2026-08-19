# Traffic Statistics Testing Guide

## Overview
This guide helps verify that ONT traffic statistics (RX/TX packets and errors) are being collected correctly from ZTE OLT devices.

## Prerequisites

1. **Network Access**: Ensure you can reach the ZTE OLT via SNMP
2. **SNMP Community**: Valid SNMP v2c community string
3. **Go Environment**: Go 1.23+ installed

## Quick Test

### Option 1: Automated Test Script

```bash
# Set OLT credentials
export OLT_IP="172.20.1.251"
export OLT_COMMUNITY="public"

# Run test
./backend/test_traffic_stats.sh
```

### Option 2: Manual Test

```bash
cd backend

# Build test program
go build -o test_traffic_stats test_traffic_stats.go

# Run with environment variables
OLT_IP=172.20.1.251 OLT_COMMUNITY=public ./test_traffic_stats
```

## Expected Output

### Success Case:
```
Testing traffic statistics collection from ZTE OLT: 172.20.1.251
Community: public, Port: 161

Step 1: Discovering ONTs...
Found 2 slots

ONT: Slot=1 Port=1 ONTID=1 Serial=ZTEGC1234567
  ✅ Traffic Statistics:
     RX: 1234567890 bytes, 9876543 packets, 12 errors
     TX: 9876543210 bytes, 8765432 packets, 5 errors
  Optical: RX=-23.45 dBm TX=2.34 dBm Distance=1234m

=====================================
SUMMARY:
  Total ONTs discovered: 15
  ONTs with traffic data: 15
  Coverage: 100.0%
=====================================

=====================================
TARGET ONT FOUND: RTEGC609833D
=====================================
Serial: RTEGC609833D
Status: online
RX Bytes: 1234567890
TX Bytes: 9876543210
RX Packets: 9876543
TX Packets: 8765432
RX Errors: 12
TX Errors: 5

✅ SUCCESS: Traffic statistics are being collected!
```

### Failure Case - No Traffic Data:
```
=====================================
SUMMARY:
  Total ONTs discovered: 15
  ONTs with traffic data: 0
  Coverage: 0.0%
=====================================

⚠️  WARNING: No ONTs have traffic statistics!
Possible causes:
  1. All ONTs are offline
  2. OLT doesn't support traffic statistics OIDs
  3. SNMP walk timeout or community string incorrect
```

## Troubleshooting

### Issue: "No ONTs have traffic statistics"

**Possible Causes:**
1. **All ONTs are offline** - Check ONT status first
2. **OLT firmware doesn't support traffic OIDs** - Verify OLT model (C300/C320 V2.1.0 confirmed working)
3. **SNMP timeout** - OLT is slow to respond, increase timeout
4. **Wrong community string** - Verify read community

**Solutions:**
```bash
# Test SNMP connectivity first
snmpwalk -v2c -c public 172.20.1.251 1.3.6.1.2.1.1.1.0

# Test specific traffic OID manually
snmpwalk -v2c -c public 172.20.1.251 1.3.6.1.4.1.3902.1012.3.50.15.1.1.4
```

### Issue: "Target ONT not found"

**Causes:**
- ONT serial number doesn't match
- ONT is not provisioned on this OLT

**Solutions:**
```bash
# List all discovered ONTs
./test_traffic_stats 2>&1 | grep "Serial="

# Check database for ONT
psql -U postgres -d tikman_db -c "SELECT serial_number, status FROM onts WHERE serial_number LIKE '%RTEGC609833D%';"
```

### Issue: "SNMP timeout"

**Solutions:**
```go
// Increase timeout in internal/connectivity/snmp.go
client := &gosnmp.GoSNMP{
    Target:  ipAddress,
    Port:    uint16(snmpPort),
    Timeout: time.Second * 10, // Increase from 5 to 10
    Retries: 2,                // Increase retries
}
```

## Production Deployment Test

After deploying to production:

1. **Apply Migration:**
```bash
docker exec -it tikman-postgres psql -U postgres -d tikman_db -f /docker-entrypoint-initdb.d/06_add_traffic_stats_to_aggregates.sql
```

2. **Restart Services:**
```bash
docker-compose restart api worker
```

3. **Wait for Metrics Collection:**
Worker collects metrics every 5 minutes. Wait at least 5 minutes after restart.

4. **Check Database:**
```sql
-- Check if packets/errors are being stored
SELECT 
    ont_id, 
    rx_packets, 
    tx_packets, 
    rx_errors, 
    tx_errors,
    time 
FROM ont_metrics 
WHERE time > NOW() - INTERVAL '10 minutes'
ORDER BY time DESC 
LIMIT 20;
```

5. **Verify Frontend:**
- Login to TikMan UI
- Navigate to ONT list
- Click on ONT RTEGC609833D
- Check "Traffic Statistics" tab
- Verify RX/TX Packets and Errors show non-zero values

## SNMP OIDs Reference

Traffic statistics OIDs used (ZTE-AN-MIB):

```
Base OID: .1.3.6.1.4.1.3902.1012 (TYPE space)

RX Packets: .3.50.15.1.1.4
TX Packets: .3.50.15.1.1.5
RX Errors:  .3.50.15.1.1.6
TX Errors:  .3.50.15.1.1.7

Full OID example for RX Packets:
.1.3.6.1.4.1.3902.1012.3.50.15.1.1.4.<ifIndex>.<onuID>
```

Index format: `<ifIndex>.<onuID>`
- ifIndex = OnuTypeIfIndexBase + slot*OnuTypeSlotStride + pon*OnuTypeIncrement
- Example: Slot 1, Port 1 → ifIndex = 268435456 + 1*65536 + 1*256 = 268501248

## Success Criteria

✅ Test passes when:
1. At least one ONT has non-zero packet counts
2. Target ONT RTEGC609833D is found and has traffic data
3. No SNMP errors or timeouts
4. Coverage > 50% (more than half of ONTs have traffic data)

## Support

If traffic statistics still not working after following this guide:

1. Verify ZTE OLT firmware version (requires V2.1.0 or newer)
2. Check SNMP access permissions on OLT
3. Verify MIB support: `snmpwalk -v2c -c public <OLT_IP> .1.3.6.1.4.1.3902.1012.3.50.15`
4. Contact ZTE support to confirm traffic statistics OID support
