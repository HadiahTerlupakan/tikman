# Topology Discovery Metrics Fix

## Status
✅ **FIXED** - Distance, RX Power, TX Power sekarang muncul di topology discovery

## Masalah
Ketika menggunakan topology discovery (pilih OLT → Card → Port), kolom Distance (m), RX Power (dBm), dan TX Power (dBm) menampilkan "-" padahal backend seharusnya bisa mengambil data metrics dari OLT.

## Root Cause Analysis

### Discovery ada 2 Mode:
1. **Database Mode** - Menampilkan ONT yang sudah registered di database
2. **Topology Discovery Mode** - Real-time discovery dari OLT via SNMP

### Masalah Ditemukan:
1. **Backend:** Topology discovery menggunakan `walkONTMetricsForPort()` yang melakukan individual SNMP GET per ONT, tetapi query-nya gagal tanpa error log
2. **Frontend:** Menggunakan operator `||` untuk konversi snake_case ke camelCase, yang menyebabkan nilai `0` atau `null` ter-replace dengan undefined

## Solusi yang Diterapkan

### 1. Backend - Gunakan WalkONTMetrics (backend/internal/connectivity/snmp.go)

**Sebelum:**
```go
func DiscoverOLTTopology(...) {
    // ... untuk setiap port ...
    metricMetrics, err := walkONTMetricsForPort(...)  // Individual GET per ONT
    // ...
}
```

**Sesudah:**
```go
func DiscoverOLTTopology(...) {
    statuses, err := WalkONTStatuses(...)
    
    // Walk metrics table ONCE untuk semua ONTs
    allMetrics, err := WalkONTMetrics(ipAddress, community, snmpPort)
    log.Printf("[Topology] Retrieved metrics for %d ONTs", len(allMetrics))
    
    // ... untuk setiap port ...
    // Copy metrics dari allMetrics untuk ONTs di port ini
    for _, loc := range ontLocations {
        if m, ok := allMetrics[loc]; ok {
            if m.RxPower != nil || m.TxPower != nil || m.Distance > 0 {
                metricsCopy := m
                metrics[loc] = &metricsCopy
            }
        }
    }
    // ...
}
```

**Keuntungan:**
- 1 SNMP walk untuk semua metrics vs N individual GET queries
- Konsisten dengan monitoring worker yang sudah proven working
- Lebih cepat dan efisien

### 2. Frontend - Fix Snake Case Conversion (frontend/src/presentation/pages/OntListPage.tsx)

**Sebelum:**
```typescript
onts: port.onts?.map((ont: any) => ({
  rxPower: ont.rx_power || ont.rxPower,  // ❌ nilai 0 akan jadi undefined
  txPower: ont.tx_power || ont.txPower,  // ❌ nilai null akan jadi undefined
  distance: ont.distance,
}))
```

**Sesudah:**
```typescript
onts: port.onts?.map((ont: any) => ({
  rxPower: ont.rx_power !== undefined ? ont.rx_power : ont.rxPower,  // ✅ preserves 0 and null
  txPower: ont.tx_power !== undefined ? ont.tx_power : ont.txPower,  // ✅ preserves 0 and null
  distance: ont.distance,
}))
```

**Mengapa perlu diubah:**
- Operator `||` menganggap `0`, `null`, `""`, `false` sebagai falsy
- Metrics bisa bernilai `0` (valid) atau `null` (no signal)
- Harus pakai explicit `undefined` check

## Cara Testing

### 1. Restart Services
```bash
# Rebuild backend dan frontend
docker-compose build api frontend
docker-compose restart api frontend

# Atau restart semua
docker-compose down && docker-compose up -d
```

### 2. Test Topology Discovery
1. Buka http://localhost:3000
2. Login: admin / admin123
3. Navigasi ke **ONT Monitoring**
4. Pilih **OLT** (contoh: Cariu)
5. Pilih **Card/Slot** (contoh: Card 3)
6. Pilih **PON Port** (contoh: Port 1)

### 3. Verifikasi Hasil
Tabel akan menampilkan:
- **Distance (m)**: angka dalam meter (contoh: 983, 796)
- **RX Power (dBm)**: nilai negatif (contoh: -24.82, -27.96)
- **TX Power (dBm)**: nilai positif (contoh: 2.23, 2.26)

### 4. Check Backend Logs
```bash
docker logs tikman-api 2>&1 | grep -E "\[Topology\]" | tail -20
```

Output yang diharapkan:
```
[Topology] Starting discovery for OLT 113.192.1.98:23161
[Topology] Found 198 ONTs in phase state table
[Topology] Retrieved metrics for 198 ONTs
[Topology] Processing slot 3, port 1 (ifIndex=285278977)
[Topology] Mapped 49 metrics for slot 3 port 1
```

## Perbandingan: Database Mode vs Discovery Mode

### Database Mode (ONT List tanpa filter)
- Data dari tabel `onts` di PostgreSQL
- Metrics dari tabel `ont_metrics` (collected every 5 min)
- Hanya untuk ONT yang sudah di-register
- **Fix:** Sudah dikerjakan di commit sebelumnya (0070418)

### Discovery Mode (Pilih OLT → Card → Port)
- Data real-time dari OLT via SNMP
- Metrics langsung dari ZXGPON-MIB tables
- Menampilkan SEMUA ONT di OLT, termasuk yang belum register
- **Fix:** Commit ini (beaf4d4)

## Commits

1. **0070418** - fix(monitoring): display ONT metrics in monitoring table
   - Tambah metrics ke API `/api/v1/onts` (database mode)
   
2. **beaf4d4** - fix(topology): use WalkONTMetrics for topology discovery
   - Fix metrics collection di topology discovery (discovery mode)

## Catatan Penting

### Kapan Metrics Muncul?
- **Discovery Mode:** Real-time saat topology discovery
- **Database Mode:** Setelah monitoring worker runs (setiap 5 menit)

### Nilai NULL vs 0 vs "-"
- `NULL` di database = ONT tidak melaporkan sinyal optik
- `0` = nilai valid (jarang untuk optical power)
- `-` di frontend = tidak ada data atau belum ada metrics

### Performance
- Topology discovery dengan metrics: ~10 detik untuk 200 ONTs
- Monitoring worker metrics collection: ~10 detik untuk semua ONTs online

## Troubleshooting

### Metrics masih "-" di Discovery Mode?
1. Cek backend logs: `docker logs tikman-api | grep Topology`
2. Pastikan ada log "Retrieved metrics for X ONTs"
3. Pastikan ada log "Mapped X metrics for slot Y port Z"

### Metrics masih "-" di Database Mode?
1. Cek apakah ONT status ONLINE (hanya online ONT yang di-collect)
2. Tunggu 5 menit untuk cycle berikutnya
3. Cek database: `SELECT * FROM ont_metrics ORDER BY time DESC LIMIT 10;`

### Frontend tidak update setelah rebuild?
1. Hard refresh browser: Ctrl+F5 (Windows) atau Cmd+Shift+R (Mac)
2. Clear browser cache
3. Buka DevTools → Network tab → Disable cache

## Next Steps (Optional)

### Optimasi Performance
- Batch query metrics untuk database mode (1 query vs N queries)
- Cache topology discovery results (TTL 30 detik)
- Pagination untuk large ONT lists

### UI Improvements
- Loading indicator saat topology discovery
- Error message jika metrics gagal di-retrieve
- Tooltip untuk explain NULL vs no-signal

### Monitoring
- Alert jika metrics collection fails
- Dashboard untuk metrics collection success rate
