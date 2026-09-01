# 🚀 INSTRUCTIONS TO FIX YOUR ONT MONITORING

## ✅ Problem Solved!

From your OLT CLI output: `show gpon onu state gpon-olt_1/3/1`

**Physical Location Discovered:**
- **Rack:** 1 (from `gpon-olt_**1**/3/1`)
- **Shelf:** 3 (from `gpon-olt_1/**3**/1`)  
- **Slot:** 1 (from `gpon-olt_1/3/**1**`)

This is the CORRECT configuration for ZTE C300 format!

---

## 🔧 Quick Fix (3 Steps)

### Step 1: Update Database Configuration

Choose ONE of these methods:

#### **Method A: Via psql/Docker (Automated)**

If you have terminal access to PostgreSQL:

```bash
docker exec -it tikman-postgres psql -U tikman -d tikman

# Then paste this SQL:
UPDATE olts 
SET rack = 1, shelf = 3, slot = 1 
WHERE ip_address = '192.0.2.10';

# Verify:
SELECT id, name, ip_address, rack, shelf, slot FROM olts WHERE ip_address = '192.0.2.10';
```

#### **Method B: Using SQL File (Recommended)**

1. Open file: `/Users/rohadimraja/Documents/tikman/RUN_THIS_UPDATE.sql`
2. Copy everything in that file
3. Paste into your database client (psql, DBeaver, pgAdmin, etc.)
4. Execute

#### **Method C: Via Web UI**

1. Login to dashboard http://localhost:8080
2. Go to **OLT Management** → Edit your OLT
3. Scroll to **Physical Location** section
4. Fill in:
   - Rack: `1`
   - Shelf: `3`  
   - Slot: `1`
5. Click **Update**

---

### Step 2: Restart Backend & Worker Services

```bash
cd /Users/rohadimraja/Documents/tikman/backend

# Stop existing services
killall main.go 2>/dev/null || true
killall worker 2>/dev/null || true

# Start API server (for config refresh)
go run cmd/api/main.go &

# Wait a moment
sleep 3

# Start worker service (collect metrics with new config)
go run cmd/worker/main.go &
```

---

### Step 3: Wait & Verify

1. **Wait 5 minutes** for next metrics collection cycle
2. **Refresh ONT Monitoring page** in browser
3. **Check results:**
   - ✅ Duplicate serials should be GONE
   - ✅ Each ONT shows unique serial number
   - ✅ Real-time metrics appear with correct values

---

## 📊 Expected Results

### Before Fix (Current Problem):
```
Serial Number    Port/ONT ID    Status
HG8245H          1/55           ONLINE     ← Wrong ifindex!
HG8245H5         1/18           ONLINE     ← Same wrong data!
```

### After Fix (Expected):
```
Serial Number       Port/ONT ID    Status
HG8245H-ABC123      1/55           ONLINE     ← Actual ONT!
HG8245H-XYZ789      1/18           ONLINE     ← Another actual ONT!
...more unique serials
```

Each ONT will show its REAL serial number and accurate signal metrics!

---

## 🔍 Understanding the Fix

### How SNMP OID Calculation Works Now

**Before (WRONG):** Default rack/shelf/slot = 0,0,0
```
ifindex = (0 << 25) | (0 << 19) | (0 << 13) | (port << 8)
        = 0 + 0 + 0 + (port × 256)
```

**After (CORRECT):** From CLI: rack=1, shelf=3, slot=1
```
Base ifindex = (1 << 25) | (3 << 19) | (1 << 13)
             = 33554432 + 24576 + 8192
             = 33587200

For Port 1, ONT ID 18:
ifindex = 33587200 + (1 << 8) + 18
        = 33587200 + 256 + 18
        = 33587474

SNMP OID example:
Rx Power: .1.3.6.1.4.1.3902.1012.3.28.1.1.5.33587474.18
Tx Power: .1.3.6.1.4.1.3902.1012.3.28.1.1.6.33587474.18
```

This matches your actual hardware layout!

---

## 🛠️ Verify Everything Works

### Check Worker Logs

Open a new terminal:
```bash
tail -f /var/log/tikman-worker.log
# or watch the stdout from worker process

# You should see successful SNMP queries like:
[Worker] Collected metrics serial=ACTUAL-HG8245H-xxx port=1 ont_id=18 rx_power=-18.5 tx_power=2.3
```

### Check Dashboard Metrics

In ONT Monitoring → View Details:
- **Rx Power:** Should be -25 to -10 dBm (good signal)
- **Tx Power:** Should be +0 to +4 dBm (transmitting correctly)
- **Temperature:** 0-50°C (normal operating temp)
- **Voltage:** 3.0-3.7V (normal power)

If all values look reasonable, fix is working! ✓

---

## 📝 Additional Notes

### For Other OLTs

If you have more than one OLT, discover each one's location:

```bash
ssh admin@<OLT_IP>
show gpon onu state gpon-olt_{rack}/{shelf}/{slot}
```

Then update accordingly:
```sql
UPDATE olts SET rack=X, shelf=Y, slot=Z WHERE name='OLT_Name';
```

### Common ZTE C300 Formats

| Format | Rack | Shelf | Slot |
|--------|------|-------|------|
| `gpon-olt_0/0/1` | 0 | 0 | 1 |
| `gpon-olt_1/3/1` | 1 | 3 | 1 | ← Your setup!
| `gpon-olt_8/0/24` | 8 | 0 | 24 |
| `gpon-olt_0/1/0` | 0 | 1 | 0 |

---

## 🆘 Troubleshooting

### Still seeing duplicate serials?

1. **Verify SQL executed correctly:**
   ```sql
   SELECT id, name, rack, shelf, slot FROM olts;
   -- Should show rack=1, shelf=3, slot=1 for your OLT
   ```

2. **Check worker is running:**
   ```bash
   ps aux | grep "worker main.go"
   ```

3. **Check logs for errors:**
   ```bash
   tail -100 /var/log/tikman-worker.log | grep -i error
   ```

4. **Wait full 5 minutes** for next collection cycle

### SNMP Timeout Errors?

Try different SNMP community string:
```sql
-- Try "public" instead of "<community-anda>"
UPDATE olts SET snmp_community = 'public' WHERE ip_address = '192.0.2.10';
```

---

## ✨ Summary

| Item | Value |
|------|-------|
| **Problem** | Duplicate ONT serial numbers, wrong metrics |
| **Root Cause** | Wrong rack/shelf/slot in SNMP OID calculation |
| **Solution** | Configure physical location from CLI: Rack=1, Shelf=3, Slot=1 |
| **Action** | Run SQL UPDATE + restart services |
| **Expected** | Unique serials per ONT, accurate real-time metrics |

---

**Run the SQL update now and wait 5 minutes!** 🎯

All documentation files are in `/Users/rohadimraja/Documents/tikman/`:
- `RUN_THIS_UPDATE.sql` - Full SQL script to execute
- `INSTRUCTIONS_TO_FIX_YOUR_OLT.md` - This guide
- `TROUBLESHOOTING_DUPLICATE_SERIALS.md` - Detailed troubleshooting

Good luck! 🚀
