# ONT Metrics Display Fix - Documentation

## Masalah
Data Distance (m), RX Power (dBm), dan TX Power (dBm) tidak muncul di tabel ONT Monitoring, menampilkan "-" untuk semua baris.

## Penyebab
- Monitoring worker sudah berhasil mengumpulkan dan menyimpan metrics ke tabel `ont_metrics`
- Namun API endpoint `/api/v1/onts` tidak mengambil dan menyertakan data metrics dalam response
- Frontend tidak menerima data metrics untuk ditampilkan

## Solusi yang Diterapkan

### 1. Backend API - DTO (backend/internal/api/dto.go)

**Perubahan:**
```go
type ONTResponse struct {
    ID           uuid.UUID         `json:"id"`
    OLTID        uuid.UUID         `json:"olt_id"`
    OLTName      string            `json:"olt_name"`
    PortID       int               `json:"port_id"`
    ONTID        int               `json:"ont_id"`
    SerialNumber string            `json:"serial_number"`
    Description  string            `json:"description"`
    Status       models.ONTStatus  `json:"status"`
    LastSeenAt   *time.Time        `json:"last_seen_at"`
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
    // Fields baru untuk metrics
    Distance     *int              `json:"distance,omitempty"`
    RxPower      *float64          `json:"rx_power,omitempty"`
    TxPower      *float64          `json:"tx_power,omitempty"`
}

// Function baru untuk convert dengan metrics
func ToONTResponseWithMetrics(ont *models.ONT, metrics *services.ONTMetricsRow) ONTResponse {
    resp := ToONTResponse(ont)
    if metrics != nil {
        resp.Distance = &metrics.Distance
        resp.RxPower = metrics.RxPower
        resp.TxPower = metrics.TxPower
    }
    return resp
}
```

### 2. Backend Handler (backend/internal/api/ont_handler.go)

**Perubahan pada ONTHandler struct:**
```go
type ONTHandler struct {
    ontService     *services.ONTService
    metricsService *services.MetricsService  // BARU
    auditService   *services.AuditService
}

func NewONTHandler(
    ontService *services.ONTService, 
    metricsService *services.MetricsService,  // BARU
    auditService *services.AuditService,
) *ONTHandler {
    return &ONTHandler{
        ontService:     ontService,
        metricsService: metricsService,  // BARU
        auditService:   auditService,
    }
}
```

**Perubahan pada List() handler:**
```go
responses := make([]ONTResponse, len(onts))
for i, ont := range onts {
    // Fetch latest metrics untuk setiap ONT
    metrics, _ := h.metricsService.GetLatestMetrics(ont.ID)
    
    // Gunakan function baru dengan metrics
    resp := ToONTResponseWithMetrics(&ont, metrics)
    resp.OLTName = oltMap[ont.OLTID]
    responses[i] = resp
}
```

### 3. Backend Router (backend/internal/api/router.go)

**Perubahan:**
```go
ontHandler := NewONTHandler(ontService, metricsService, auditService)
//                                      ^^^^^^^^^^^^^^^ tambahan parameter
```

### 4. Frontend Entity (frontend/src/domain/entities/Ont.ts)

**Perubahan:**
```typescript
export interface Ont {
  id: string;
  oltId: string;
  oltName: string;
  portId: number;
  ontId: number;
  serialNumber: string;
  description: string;
  status: OntStatus;
  lastSeenAt: string | null;
  createdAt: string;
  updatedAt: string;
  // Fields baru untuk metrics
  distance?: number;
  rxPower?: number | null;
  txPower?: number | null;
}
```

## Cara Kerja Setelah Fix

1. **Monitoring Worker** (berjalan setiap 5 menit):
   - Mengumpulkan metrics dari OLT via SNMP
   - Menyimpan ke tabel `ont_metrics` (TimescaleDB hypertable)

2. **API Endpoint** (`GET /api/v1/onts`):
   - Mengambil semua ONT dari database
   - Untuk setiap ONT, query latest metrics dari `ont_metrics`
   - Menyertakan distance, rx_power, tx_power dalam response JSON

3. **Frontend** (OntListPage.tsx):
   - Menerima data dengan metrics dari API
   - Menampilkan nilai di kolom Distance, RX Power, TX Power
   - Jika tidak ada data, tetap tampilkan "-"

## Testing

### 1. Verifikasi Backend Build
```bash
cd backend
go build -o api cmd/api/main.go
```

### 2. Verifikasi Monitoring Worker
```bash
docker logs tikman-api | grep "Metrics collected"
```

Output yang diharapkan:
```
2026/08/17 02:21:05 [Worker] Metrics collected: serial=HG8245H rx_power=-24.82 tx_power=2.23 distance=983m
2026/08/17 02:21:05 [Worker] Metrics collected: serial=HG8245H5 rx_power=-27.96 tx_power=2.26 distance=796m
```

### 3. Verifikasi API Response (dengan authentication)
Login ke frontend: http://localhost:3000
- Username: `admin`
- Password: `admin123`

Kemudian check Network tab di browser, cari request ke `/api/v1/onts`

Response harus include:
```json
{
  "data": [
    {
      "id": "...",
      "serial_number": "HG8245H",
      "distance": 983,
      "rx_power": -24.82,
      "tx_power": 2.23,
      ...
    }
  ]
}
```

### 4. Verifikasi Frontend Display
Buka halaman ONT Monitoring:
- Kolom "Distance (m)" harus menampilkan angka (contoh: 983)
- Kolom "RX Power (dBm)" harus menampilkan angka (contoh: -24.82)
- Kolom "TX Power (dBm)" harus menampilkan angka (contoh: 2.23)

## Catatan Penting

### Kapan Metrics Muncul?
- Metrics hanya dikumpulkan untuk ONT dengan status **ONLINE**
- Worker berjalan setiap **5 menit**
- Jika ONT baru ditambahkan, tunggu maksimal 5 menit untuk metrics pertama

### Nilai NULL vs "-"
- `NULL` / `nil` pada rx_power/tx_power = ONT tidak melaporkan sinyal optik
- `-` di frontend = belum ada data metrics sama sekali

### Performance
- Query metrics untuk setiap ONT dilakukan di handler
- Untuk optimasi di masa depan, pertimbangkan:
  - Bulk query metrics dengan `WHERE ont_id IN (...)`
  - Caching metrics dengan TTL 30 detik
  - Pagination untuk mengurangi jumlah query

## Deployment

### Docker Compose
```bash
# Rebuild dan restart backend
docker-compose build api
docker-compose up -d

# Verifikasi logs
docker logs -f tikman-api
```

### Manual
```bash
# Backend
cd backend
go build -o api cmd/api/main.go
./api

# Frontend (jika ada perubahan)
cd frontend
npm run build
```

## Status
✅ Backend DTO updated
✅ Backend handler updated
✅ Backend router updated
✅ Frontend entity updated
✅ Build sukses
✅ Container running
✅ Monitoring worker collecting metrics
✅ Ready for testing

## Next Steps
1. Buka http://localhost:3000
2. Login dengan admin/admin123
3. Navigasi ke halaman ONT Monitoring
4. Verifikasi kolom Distance, RX Power, TX Power sudah menampilkan data

Jika masih menampilkan "-", tunggu 5 menit untuk siklus metrics collection berikutnya.
