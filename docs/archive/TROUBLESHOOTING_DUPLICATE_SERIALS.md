# 🔧 Troubleshooting: Duplicate ONT Serial Numbers

## Problem Identified

Your ONT Monitoring shows **duplicate serial numbers**:
```
HG8245H (Port 1/55)   ← Same serial!
HG8245H5 (Port 1/18)  ← Also same serial!
```

This means **ONT metrics are coming from wrong SNMP OIDs** due to incorrect rack/shelf/slot configuration.

---

## Root Cause Analysis

### How It Happens

When you configure an OLT with **wrong rack/shelf/slot**, the SNMP OID calculation produces the wrong ifindex:

```go
// WRONG configuration causes wrong ifindex
wrong_ifindex = (wrong_rack << 25) | (wrong_shelf << 19) | (wrong_slot << 13) | (port << 8)

// SNMP query then fetches data from WRONG ONT
OID_RX = .1.3.6.1.4.1.3902.1012.3.28.1.1.5.{wrong_ifindex}.{ont_id}
         └─┬─┘ └──── WRONG IFINDEX HERE ────┘ └── ont_id
          Base      ^ This doesn't match your hardware!
```

**Result:** 
- SNMP queries different physical ONTs than intended
- Serial numbers appear duplicated because you're querying the same remote ONT via wrong paths
- Metrics are completely wrong/inaccurate

---

## Solution: Discover Correct Physical Location

### Step 1: Check Current Configuration

Run this SQL in PostgreSQL to see current setup:

```sql
SELECT id, name, ip_address, rack, shelf, slot FROM olts;
```

If all values are `0`, that's your problem!

### Step 2: Find Actual Rack/Shelf/Slot

There are **THREE ways** to discover your actual location:

#### Method A: Via Web Dashboard (EASIEST)

1. Login to http://localhost:8080
2. Go to **OLT Management**
3. Click **Edit** on your OLT at IP 113.192.1.98
4. Look for existing rack/shelf/slot values
5. If all zeros, proceed to Method B

#### Method B: From Hardware Labels (MOST RELIABLE)

Look at your physical equipment:

1. **Find the Rack Number**
   - Check rack label/drawer (usually numbered 0-15)
   - Your OLT chassis should be in a specific rack
   
2. **Find the Shelf Number**
   - Inside the rack, look for shelf/card cage numbers (0-7)
   - Each shelf holds line cards
   
3. **Find the Slot Number**
   - The GPON card itself has a slot position
   - Usually labeled as "Slot 1", "Slot 2", etc.
   
**Example:** If OLT is in Rack 1, Shelf 0, Slot 2 → Configure: `rack=1, shelf=0, slot=2`

#### Method C: Network Discovery via SNMP

If you can access OLT via SSH/Telnet:

```bash
ssh admin@113.192.1.98
# Then run commands like:
show system rack
show system shelf  
show board
```

Or use SNMP walk from another machine:

```bash
snmpwalk -v2c -c ufiber2 113.192.1.98:23161 .1.3.6.1.4.1.3902.1012.3.28.1.1.1
```

Parse the OID suffix to extract ifindex, then decode:
```
ifindex = 134217728 (example)
rack    = (134217728 >> 25) & 0xF = 8
shelf   = (134217728 >> 19) & 0x7 = 0
slot    = (134217728 >> 13) & 0x1F = 26
```

---

## Fix Implementation

### Option 1: Via Web UI (Recommended)

1. Login to dashboard
2. **OLT Management** → Edit OLT `113.192.1.98`
3. Fill in Physical Location section:
   ```
   Rack:     [your discovered rack]
   Shelf:    [your discovered shelf]
   Slot:     [your discovered slot]
   ```
4. Click **Update**
5. Restart worker service: `go run cmd/worker/main.go`

### Option 2: Via SQL Database

If you prefer direct database update:

```sql
-- Update OLT configuration based on discovered location
UPDATE olts
SET rack = X,   -- Replace X with your rack (0-15)
    shelf = Y,  -- Replace Y with your shelf (0-7)
    slot = Z    -- Replace Z with your slot (0-31)
WHERE ip_address = '113.192.1.98';
```

**Common configurations to try:**

If you don't know exact location, try these common setups:

```sql
-- Try #1: Standard single GPON card setup
UPDATE olts SET rack = 0, shelf = 0, slot = 1 WHERE ip_address = '113.192.1.98';

-- Try #2: If in different rack
UPDATE olts SET rack = 1, shelf = 0, slot = 1 WHERE ip_address = '113.192.1.98';

-- Try #3: Multiple card setup (slot varies)
UPDATE olts SET rack = 0, shelf = 0, slot = 2 WHERE ip_address = '113.192.1.98';
```

After each update, restart worker and check if duplicate serials disappear.

---

## Verification

After updating configuration:

### 1. Check Worker Logs

Worker should now collect metrics correctly:

```bash
tail -f /var/log/tikman-worker.log
```

You should see:
```
[Worker] Collected metrics serial=HG8245H... port=1 ont_id=18 rx_power=-18.5 tx_power=2.3
[Worker] Collected metrics serial=HG8245H5... port=1 ont_id=55 rx_power=-20.1 tx_power=2.1
```

Each ONT should have **unique serial number**.

### 2. Check Dashboard

Refresh ONT Monitoring page:
- ✅ Each ONT should have distinct serial number
- ✅ No more duplicate HG8245H entries
- ✅ Real-time metrics displayed correctly

### 3. Verify Metrics Values

Check that signal values make sense:
- **Rx Power**: -25 to -10 dBm (good), -27 to -30 dBm (warning)
- **Tx Power**: +0 to +4 dBm
- **Temperature**: 0-50°C
- **Distance**: 0-20km

---

## Why Duplicates Occur

The issue happens because:

1. **Wrong ifindex calculation** → SNMP queries wrong ONT
2. **Multiple ports map to same wrong ONT** → Appears as duplicates
3. **Serial numbers repeat** because they come from same physical device accessed via wrong path

**Example Scenario:**
- Your OLT has ONT @ Port 1/18 with real serial "ACTUAL-HG8245H"
- Wrong config calculates ifindex pointing to different ONT
- That ONT returns serial "HG8245H" 
- Same happens for Port 1/55 → Also gets "HG8245H"
- Result: Two different ports show same serial!

**Fix:** Correct ifindex calculation ensures each port queries its actual ONT.

---

## Quick Reference

| Field | Range | Description | Example |
|-------|-------|-------------|---------|
| Rack | 0-15 | Equipment rack number | 0 (default) |
| Shelf | 0-7 | Shelf within rack | 0 (default) |
| Slot | 0-31 | GPON card slot | 1 (common) |

**Calculate ifindex formula:**
```
ifindex = (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)
```

**For example (Rack 0, Shelf 0, Slot 1):**
```
Base ifindex = (0 << 25) | (0 << 19) | (1 << 13) = 8192
Port 1 ONT 18: 8192 + (1 << 8) + 18 = 8192 + 256 + 18 = 8466
SNMP OID: .1.3.6.1.4.1.3902.1012.3.28.1.1.5.8466.18
```

---

## Next Steps

1. ✅ Discover correct rack/shelf/slot using one of three methods
2. ✅ Update OLT configuration via UI or SQL
3. ✅ Restart worker service
4. ✅ Wait 5 minutes for next metrics collection
5. ✅ Verify dashboard shows unique serial numbers
6. ✅ Check metrics values are reasonable

---

## Still Having Issues?

If duplicates persist after fixing:

1. **Check SNMP community string** - Must match what OLT expects (try "public" if unsure)
2. **Verify SNMP port** - Should be 23161 per your input
3. **Confirm ONTs are online** - Disconnected ONTs may return cached/wrong data
4. **Test direct SNMP connection** - Use snmpwalk command-line tool

Contact network team to verify OLT configuration matches expectations.

---

**Summary:** The duplicate serials are a symptom of wrong rack/shelf/slot causing incorrect SNMP queries. Fix the physical location config, and the problem will resolve! 🎯
