# 🚀 ONT Monitoring Quick Start Guide

## ✅ What Was Fixed?

Your ONT monitoring was showing wrong data because SNMP queries used hardcoded rack/shelf/slot values (Rack 8, Shelf 0, Slot 24) for ALL OLTs.

**Now fixed:** Each OLT has its own configurable rack/shelf/slot settings that match your actual hardware location.

---

## ⚡ Quick Setup (3 Steps)

### Step 1: Run Migration

```bash
cd backend
go run cmd/api/main.go
```

Wait until you see server startup, then press `Ctrl+C`.

GORM will automatically add new columns to your database:
- `rack` (integer, default: 0)
- `shelf` (integer, default: 0)  
- `slot` (integer, default: 0)

### Step 2: Configure Your OLTs

1. Open browser: http://localhost:8080
2. Login with admin/admin123
3. Go to **OLT Management** page
4. Click **Edit** on each OLT
5. Fill in the **Physical Location** section:
   - **Rack**: 0-15 (your equipment rack number)
   - **Shelf**: 0-7 (shelf number in rack)
   - **Slot**: 0-31 (slot for GPON card)
6. Click **Update**

💡 **Not sure about rack/shelf/slot?** Use defaults (0, 0, 0) first and adjust later if needed.

### Step 3: Start Worker Service

```bash
cd backend
go run cmd/worker/main.go
```

This service collects ONT metrics every 5 minutes via SNMP.

Watch logs for success messages:
```
[Worker] Collected metrics serial=ZTEG12345678 port=0 ont_id=5 rx_power=-18.5 tx_power=2.3
```

---

## 🔍 Verify Everything Works

### Check Metrics Collection

Open a new terminal:
```bash
tail -f /var/log/tikman-worker.log
# or check stdout from worker process
```

Look for:
- ✅ "Collected metrics" → Success!
- ❌ "SNMP query failed" → Check rack/shelf/slot values

### View Metrics in Dashboard

1. Go to **ONT Monitoring** page
2. Click **View** button on any ONT
3. Check if signal metrics appear:
   - Rx Power (should be -25 to -10 dBm for good signal)
   - Tx Power (should be 0 to +4 dBm)
   - Temperature, Voltage, Distance
   - Traffic counters

If metrics show "No metrics data available yet", wait 5 minutes for next collection cycle.

---

## 📊 Understanding Rack/Shelf/Slot

### ZTE C300 Hardware Layout

```
Rack → Contains multiple Shelves
└── Shelf → Contains multiple Slots
    └── Slot → Holds GPON Card
        └── Port → PON port (0-15)
            └── ONT → User device (0-127)
```

### Example Configuration

**Scenario:** OLT with GPON card in slot 2 of shelf 0 in rack 1

```
Rack: 1
Shelf: 0
Slot: 2
Port: 0 (first PON port on card)
```

### SNMP OID Formula

The system calculates SNMP OIDs using:
```
ifindex = (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)
```

For our example (Rack 1, Shelf 0, Slot 2, Port 0):
```
ifindex = (1 << 25) | (0 << 19) | (2 << 13) | (0 << 8)
        = 33554432 + 0 + 16384 + 0
        = 33570816
```

SNMP query becomes:
```
.1.3.6.1.4.1.3902.1012.3.28.1.1.5.33570816.{ont_id}
         └─┬─┘ └─┬─┘ └─────┬─────┘ └────┬────┘ └────┬────┘
           Base   Type  ifindex     ont_id
```

---

## 🎯 Common Scenarios

### Scenario A: Single OLT per Site
```
Rack: 0
Shelf: 0
Slot: 1  (GPON card typically in slot 1)
```

### Scenario B: Multiple OLTs in Same Rack
```
OLT-1: Rack=0, Shelf=0, Slot=1
OLT-2: Rack=0, Shelf=0, Slot=2
OLT-3: Rack=0, Shelf=1, Slot=1  (different shelf)
```

### Scenario C: Unknown Configuration
Start with all zeros and test:
```
Rack: 0
Shelf: 0
Slot: 0
```

If metrics don't appear after 5 minutes, check logs and adjust.

---

## 🛠️ Advanced: Batch Update Existing OLTs

If you have many OLTs to update, use SQL directly:

```sql
-- Set default values for all OLTs
UPDATE olts SET rack = 0, shelf = 0, slot = 0;

-- Update specific OLT by name
UPDATE olts 
SET rack = 1, shelf = 0, slot = 2 
WHERE name = 'Office-Building-A-OLT';

-- Or use UUID if known
UPDATE olts 
SET rack = 0, shelf = 1, slot = 3 
WHERE id = 'your-olt-uuid-here';
```

---

## 📝 Troubleshooting

### Issue: No metrics appearing
**Symptoms:** ONT detail modal shows "No metrics data available yet"

**Checks:**
1. ✅ Is worker running? (`pgrep -f "worker main.go"`)
2. ✅ Are there any error logs?
3. ✅ Are ONTs actually online? (check status column)
4. ✅ Is SNMP community string correct in OLT config?

**Solution:** Wait 5 minutes for next collection cycle. Check worker logs for errors.

---

### Issue: Wrong SNMPPort configured
**Symptoms:** "SNMP query failed" errors

**Check:**
```bash
docker ps | grep postgres  # Ensure PostgreSQL is running
```

**Solution:** Verify SNMP port is set correctly in OLT configuration (default 161).

---

### Issue: Invalid Rx/Tx Power values
**Symptoms:** Negative values, extremely high/low numbers

**Possible causes:**
- ONT is offline/disconnected
- Incorrect rack/shelf/slot causing wrong IFINDEX
- ONT reporting bad data

**Solution:** Check ONT is online, verify physical location config matches hardware.

---

## 🎓 Next Steps

1. **Document your rack/shelf/slot assignments** in a spreadsheet
2. **Set up alerts** for ONT offline detection
3. **Monitor traffic patterns** from rx_bytes/tx_bytes metrics
4. **Configure email notifications** for alarms

---

## 📚 Additional Resources

See `ONT_PHYSICAL_LOCATION_REFERENCE.md` for:
- Detailed SNMP OID explanation
- Rack/shelf/slot discovery methods
- Complete troubleshooting guide
- Batch update scripts

---

## ✨ Summary

| Before | After |
|--------|-------|
| ❌ Hardcoded rack=8, shelf=0, slot=24 for ALL OLTs | ✅ Configurable rack/shelf/slot per OLT |
| ❌ Wrong ONT data from incorrect SNMP OIDs | ✅ Accurate data from correct OID calculations |
| ❌ Metrics appeared but were invalid | ✅ Real-time accurate metrics collection |

Your ONT monitoring is now production-ready! 🎉
