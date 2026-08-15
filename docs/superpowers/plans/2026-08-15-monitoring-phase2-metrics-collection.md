# Monitoring Module Phase 2: Metrics Collection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement SNMP-based metrics collection for ONTs including signal levels, temperature, voltage, distance, and traffic statistics. Store metrics in TimescaleDB hypertable for time-series analysis.

**Architecture:** Worker-based metrics collector using gosnmp library to query ZTE C300 OLT SNMP OIDs. Metrics stored in ont_metrics table with automatic aggregation to 5-minute intervals via continuous aggregates.

**Tech Stack:** Go, gosnmp, GORM, TimescaleDB, React, TypeScript, Ant Design

## Global Constraints

- ZTE C300 OLT SNMP OIDs must match specification
- Polling interval: 5 minutes for metrics (different from 30s status polling)
- All metrics stored with timestamp for time-series queries
- ONT metrics API must support time range filtering
- Frontend must display real-time signal levels with color indicators
- All tests must pass before committing

---

## Task 1: SNMP Metrics Query Implementation

**Files:**
- Modify: `backend/internal/connectivity/snmp.go`
- Modify: `backend/internal/worker/monitoring_worker.go`

**Interfaces:**
- Consumes: Existing SNMP connection setup, ONT model with PortID/ONTID
- Produces: `QueryONTMetrics(ipAddress, community string, port, ontID int) (*ONTMetrics, error)` function

- [ ] **Step 1: Add ONTMetrics struct to snmp.go**

```go
type ONTMetrics struct {
    RxPower     float64 // in dBm
    TxPower     float64 // in dBm
    Temperature float64 // in Celsius
    Voltage     float64 // in Volts
    Distance    int     // in meters
    RxBytes     uint64
    TxBytes     uint64
}
```

- [ ] **Step 2: Add SNMP OID constants**

```go
const (
    // ZTE C300 OLT SNMP OIDs for ONT metrics
    // Format: OID.{rack}.{shelf}.{slot}.{port}.{ont_id}
    OID_ONT_RX_POWER     = ".1.3.6.1.4.1.3902.1012.3.28.1.1.5"  // × 0.01 dBm
    OID_ONT_TX_POWER     = ".1.3.6.1.4.1.3902.1012.3.28.1.1.6"  // × 0.01 dBm
    OID_ONT_TEMPERATURE  = ".1.3.6.1.4.1.3902.1012.3.28.1.1.7"  // in Celsius
    OID_ONT_VOLTAGE      = ".1.3.6.1.4.1.3902.1012.3.28.1.1.8"  // × 0.01 V
    OID_ONT_DISTANCE     = ".1.3.6.1.4.1.3902.1012.3.28.1.1.9"  // in meters
    OID_ONT_RX_BYTES     = ".1.3.6.1.4.1.3902.1012.3.50.13.1.4" // counter64
    OID_ONT_TX_BYTES     = ".1.3.6.1.4.1.3902.1012.3.50.13.1.5" // counter64
)
```

- [ ] **Step 3: Implement QueryONTMetrics function**

```go
func QueryONTMetrics(ipAddress, community string, port, ontID int) (*ONTMetrics, error) {
    client := &gosnmp.GoSNMP{
        Target:    ipAddress,
        Port:      161,
        Community: community,
        Version:   gosnmp.Version2c,
        Timeout:   time.Second * 5,
        Retries:   1,
    }

    err := client.Connect()
    if err != nil {
        return nil, fmt.Errorf("SNMP connect failed: %w", err)
    }
    defer client.Conn.Close()

    // Build OIDs with rack=1, shelf=1, slot=1
    oids := []string{
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_RX_POWER, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TX_POWER, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TEMPERATURE, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_VOLTAGE, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_DISTANCE, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_RX_BYTES, port, ontID),
        fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TX_BYTES, port, ontID),
    }

    result, err := client.Get(oids)
    if err != nil {
        return nil, fmt.Errorf("SNMP GET failed: %w", err)
    }

    if len(result.Variables) != 7 {
        return nil, fmt.Errorf("expected 7 values, got %d", len(result.Variables))
    }

    metrics := &ONTMetrics{
        RxPower:     float64(gosnmp.ToBigInt(result.Variables[0].Value).Int64()) * 0.01,
        TxPower:     float64(gosnmp.ToBigInt(result.Variables[1].Value).Int64()) * 0.01,
        Temperature: float64(gosnmp.ToBigInt(result.Variables[2].Value).Int64()),
        Voltage:     float64(gosnmp.ToBigInt(result.Variables[3].Value).Int64()) * 0.01,
        Distance:    int(gosnmp.ToBigInt(result.Variables[4].Value).Int64()),
        RxBytes:     uint64(gosnmp.ToBigInt(result.Variables[5].Value).Uint64()),
        TxBytes:     uint64(gosnmp.ToBigInt(result.Variables[6].Value).Uint64()),
    }

    return metrics, nil
}
```

- [ ] **Step 4: Add unit test for QueryONTMetrics**

Create `backend/internal/connectivity/snmp_metrics_test.go`:

```go
func TestQueryONTMetrics(t *testing.T) {
    // This is an integration test - requires actual OLT or mock SNMP server
    t.Skip("Integration test - requires OLT access")
}
```

- [ ] **Step 5: Commit SNMP metrics query implementation**

```bash
cd backend
go test ./internal/connectivity/... -v
git add internal/connectivity/snmp.go internal/connectivity/snmp_metrics_test.go
git commit -m "feat(monitoring): add SNMP ONT metrics query function"
```

---

## Task 2: Metrics Storage Service

**Files:**
- Create: `backend/internal/services/metrics_service.go`
- Modify: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: ONTMetrics from Task 1, ont_metrics table schema
- Produces: `MetricsService.StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics) error`

- [ ] **Step 1: Create metrics service with StoreMetrics**

```go
package services

import (
    "time"
    "github.com/google/uuid"
    "github.com/tikman/olt-provisioning/internal/connectivity"
    "gorm.io/gorm"
)

type MetricsService struct {
    db *gorm.DB
}

func NewMetricsService(db *gorm.DB) *MetricsService {
    return &MetricsService{db: db}
}

// StoreMetrics saves ONT metrics to ont_metrics hypertable
func (s *MetricsService) StoreMetrics(ontID uuid.UUID, metrics *connectivity.ONTMetrics) error {
    query := `
        INSERT INTO ont_metrics (time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
    
    return s.db.Exec(query,
        time.Now(),
        ontID,
        metrics.RxPower,
        metrics.TxPower,
        metrics.Temperature,
        metrics.Voltage,
        metrics.Distance,
        metrics.RxBytes,
        metrics.TxBytes,
    ).Error
}
```

- [ ] **Step 2: Add GetLatestMetrics method**

```go
type ONTMetricsRow struct {
    Time        time.Time
    ONTID       uuid.UUID
    RxPower     float64
    TxPower     float64
    Temperature float64
    Voltage     float64
    Distance    int
    RxBytes     uint64
    TxBytes     uint64
}

func (s *MetricsService) GetLatestMetrics(ontID uuid.UUID) (*ONTMetricsRow, error) {
    var metrics ONTMetricsRow
    
    err := s.db.Raw(`
        SELECT time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes
        FROM ont_metrics
        WHERE ont_id = $1
        ORDER BY time DESC
        LIMIT 1
    `, ontID).Scan(&metrics).Error
    
    if err != nil {
        return nil, err
    }
    
    return &metrics, nil
}
```

- [ ] **Step 3: Add GetMetricsHistory method with time range**

```go
func (s *MetricsService) GetMetricsHistory(ontID uuid.UUID, startTime, endTime time.Time) ([]ONTMetricsRow, error) {
    var metrics []ONTMetricsRow
    
    err := s.db.Raw(`
        SELECT time, ont_id, rx_power, tx_power, temperature, voltage, distance, rx_bytes, tx_bytes
        FROM ont_metrics
        WHERE ont_id = $1 AND time >= $2 AND time <= $3
        ORDER BY time DESC
    `, ontID, startTime, endTime).Scan(&metrics).Error
    
    return metrics, err
}
```

- [ ] **Step 4: Write unit tests for MetricsService**

Create `backend/internal/services/metrics_service_test.go`:

```go
func TestMetricsService_StoreMetrics(t *testing.T) {
    db := setupTestDB(t)
    defer db.Exec("DROP TABLE IF EXISTS ont_metrics")
    
    // Create hypertable
    db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb")
    db.Exec(`CREATE TABLE ont_metrics (
        time TIMESTAMPTZ NOT NULL,
        ont_id UUID NOT NULL,
        rx_power DECIMAL(6,2),
        tx_power DECIMAL(6,2),
        temperature DECIMAL(5,2),
        voltage DECIMAL(5,2),
        distance INT,
        rx_bytes BIGINT,
        tx_bytes BIGINT
    )`)
    db.Exec("SELECT create_hypertable('ont_metrics', 'time', if_not_exists => TRUE)")
    
    service := NewMetricsService(db)
    ontID := uuid.New()
    
    metrics := &connectivity.ONTMetrics{
        RxPower:     -25.5,
        TxPower:     2.3,
        Temperature: 45.2,
        Voltage:     3.3,
        Distance:    500,
        RxBytes:     1000000,
        TxBytes:     2000000,
    }
    
    err := service.StoreMetrics(ontID, metrics)
    assert.NoError(t, err)
    
    // Verify stored
    latest, err := service.GetLatestMetrics(ontID)
    assert.NoError(t, err)
    assert.Equal(t, -25.5, latest.RxPower)
}
```

- [ ] **Step 5: Initialize MetricsService in main.go**

Add after ontService initialization:

```go
metricsService := services.NewMetricsService(db)
```

- [ ] **Step 6: Run tests and commit**

```bash
cd backend
go test ./internal/services/... -v
git add internal/services/metrics_service.go internal/services/metrics_service_test.go cmd/api/main.go
git commit -m "feat(monitoring): add metrics storage service"
```

---

## Task 3: Metrics Collection Worker

**Files:**
- Modify: `backend/internal/worker/monitoring_worker.go`
- Modify: `backend/cmd/api/main.go`

**Interfaces:**
- Consumes: QueryONTMetrics from Task 1, MetricsService from Task 2
- Produces: Background metrics collector running every 5 minutes

- [ ] **Step 1: Add MetricsService to MonitoringWorker struct**

```go
type MonitoringWorker struct {
    db             *gorm.DB
    oltService     *services.OLTService
    ontService     *services.ONTService
    metricsService *services.MetricsService
    statusInterval time.Duration
    metricsInterval time.Duration
    ctx            context.Context
    cancel         context.CancelFunc
    wg             sync.WaitGroup
}
```

- [ ] **Step 2: Update NewMonitoringWorker constructor**

```go
func NewMonitoringWorker(
    db *gorm.DB,
    oltService *services.OLTService,
    ontService *services.ONTService,
    metricsService *services.MetricsService,
    statusInterval time.Duration,
    metricsInterval time.Duration,
) *MonitoringWorker {
    ctx, cancel := context.WithCancel(context.Background())
    return &MonitoringWorker{
        db:              db,
        oltService:      oltService,
        ontService:      ontService,
        metricsService:  metricsService,
        statusInterval:  statusInterval,
        metricsInterval: metricsInterval,
        ctx:             ctx,
        cancel:          cancel,
    }
}
```

- [ ] **Step 3: Add metrics polling loop to Start method**

```go
func (w *MonitoringWorker) Start() {
    log.Printf("[Worker] Monitoring worker starting (status: %v, metrics: %v)", 
        w.statusInterval, w.metricsInterval)

    w.wg.Add(2)
    go w.statusPollLoop()
    go w.metricsPollLoop()
}
```

- [ ] **Step 4: Rename existing pollLoop to statusPollLoop**

```go
func (w *MonitoringWorker) statusPollLoop() {
    defer w.wg.Done()

    ticker := time.NewTicker(w.statusInterval)
    defer ticker.Stop()

    // Run first poll immediately
    w.pollAllONTsStatus()

    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            w.pollAllONTsStatus()
        }
    }
}
```

- [ ] **Step 5: Rename pollAllONTs to pollAllONTsStatus**

```go
func (w *MonitoringWorker) pollAllONTsStatus() {
    // Existing implementation unchanged
    // Just rename from pollAllONTs
}
```

- [ ] **Step 6: Implement metricsPollLoop**

```go
func (w *MonitoringWorker) metricsPollLoop() {
    defer w.wg.Done()

    ticker := time.NewTicker(w.metricsInterval)
    defer ticker.Stop()

    // Run first poll after 10 seconds (let status poll complete first)
    time.Sleep(10 * time.Second)
    w.pollAllONTsMetrics()

    for {
        select {
        case <-w.ctx.Done():
            return
        case <-ticker.C:
            w.pollAllONTsMetrics()
        }
    }
}
```

- [ ] **Step 7: Implement pollAllONTsMetrics**

```go
func (w *MonitoringWorker) pollAllONTsMetrics() {
    start := time.Now()

    olts, err := w.oltService.List()
    if err != nil {
        log.Printf("[Worker] Failed to list OLTs for metrics: %v", err)
        return
    }

    if len(olts) == 0 {
        return
    }

    log.Printf("[Worker] Collecting metrics from %d OLTs...", len(olts))

    var totalONTs int
    var successCount int

    for _, olt := range olts {
        onts, _, err := w.ontService.List(&olt.ID, nil, 1000, 0)
        if err != nil {
            log.Printf("[Worker] Failed to list ONTs for metrics on OLT %s: %v", olt.Name, err)
            continue
        }

        totalONTs += len(onts)

        for _, ont := range onts {
            // Only collect metrics from online ONTs
            if ont.Status != models.ONTStatusOnline {
                continue
            }

            if err := w.collectONTMetrics(&olt, &ont); err != nil {
                log.Printf("[Worker] Failed to collect metrics for ONT %s: %v", ont.SerialNumber, err)
            } else {
                successCount++
            }
        }
    }

    duration := time.Since(start)
    log.Printf("[Worker] Metrics collection completed: %d/%d ONTs successful (duration: %v)",
        successCount, totalONTs, duration)
}
```

- [ ] **Step 8: Implement collectONTMetrics**

```go
func (w *MonitoringWorker) collectONTMetrics(olt *models.OLT, ont *models.ONT) error {
    metrics, err := connectivity.QueryONTMetrics(
        olt.IPAddress,
        olt.SNMPCommunity,
        ont.PortID,
        ont.ONTID,
    )
    if err != nil {
        return fmt.Errorf("SNMP query failed: %w", err)
    }

    if err := w.metricsService.StoreMetrics(ont.ID, metrics); err != nil {
        return fmt.Errorf("store metrics failed: %w", err)
    }

    return nil
}
```

- [ ] **Step 9: Update main.go to pass MetricsService and intervals**

```go
// Start monitoring worker (30s status, 5min metrics)
monitoringWorker := worker.NewMonitoringWorker(
    db,
    oltService,
    ontService,
    metricsService,
    30*time.Second,  // Status polling
    5*time.Minute,   // Metrics polling
)
monitoringWorker.Start()
log.Info("Monitoring worker started",
    zap.Duration("statusInterval", 30*time.Second),
    zap.Duration("metricsInterval", 5*time.Minute))
```

- [ ] **Step 10: Build and commit**

```bash
cd backend
go build -o api cmd/api/main.go
git add internal/worker/monitoring_worker.go cmd/api/main.go
git commit -m "feat(monitoring): add metrics collection worker with 5min interval"
```

---

## Task 4: Metrics API Endpoints

**Files:**
- Create: `backend/internal/api/metrics_handler.go`
- Modify: `backend/internal/api/dto.go`
- Modify: `backend/internal/api/router.go`

**Interfaces:**
- Consumes: MetricsService from Task 2
- Produces: GET /api/v1/onts/:id/metrics and /api/v1/onts/:id/metrics/history endpoints

- [ ] **Step 1: Add metrics DTOs to dto.go**

```go
type ONTMetricsResponse struct {
    Time        time.Time `json:"time"`
    RxPower     float64   `json:"rx_power"`
    TxPower     float64   `json:"tx_power"`
    Temperature float64   `json:"temperature"`
    Voltage     float64   `json:"voltage"`
    Distance    int       `json:"distance"`
    RxBytes     uint64    `json:"rx_bytes"`
    TxBytes     uint64    `json:"tx_bytes"`
}

func ToONTMetricsResponse(metrics *services.ONTMetricsRow) ONTMetricsResponse {
    return ONTMetricsResponse{
        Time:        metrics.Time,
        RxPower:     metrics.RxPower,
        TxPower:     metrics.TxPower,
        Temperature: metrics.Temperature,
        Voltage:     metrics.Voltage,
        Distance:    metrics.Distance,
        RxBytes:     metrics.RxBytes,
        TxBytes:     metrics.TxBytes,
    }
}
```

- [ ] **Step 2: Create metrics_handler.go**

```go
package api

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/tikman/olt-provisioning/internal/services"
)

type MetricsHandler struct {
    metricsService *services.MetricsService
}

func NewMetricsHandler(metricsService *services.MetricsService) *MetricsHandler {
    return &MetricsHandler{
        metricsService: metricsService,
    }
}

// GetLatest handles GET /api/v1/onts/:id/metrics
func (h *MetricsHandler) GetLatest(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Code:  "INVALID_ID",
            Error: "Invalid ONT ID format",
        })
        return
    }

    metrics, err := h.metricsService.GetLatestMetrics(id)
    if err != nil {
        c.JSON(http.StatusNotFound, ErrorResponse{
            Code:  "NOT_FOUND",
            Error: "No metrics found for this ONT",
        })
        return
    }

    c.JSON(http.StatusOK, ToONTMetricsResponse(metrics))
}

// GetHistory handles GET /api/v1/onts/:id/metrics/history
func (h *MetricsHandler) GetHistory(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Code:  "INVALID_ID",
            Error: "Invalid ONT ID format",
        })
        return
    }

    // Parse time range (default: last 24 hours)
    endTime := time.Now()
    startTime := endTime.Add(-24 * time.Hour)

    if start := c.Query("start"); start != "" {
        if parsed, err := time.Parse(time.RFC3339, start); err == nil {
            startTime = parsed
        }
    }

    if end := c.Query("end"); end != "" {
        if parsed, err := time.Parse(time.RFC3339, end); err == nil {
            endTime = parsed
        }
    }

    metrics, err := h.metricsService.GetMetricsHistory(id, startTime, endTime)
    if err != nil {
        c.JSON(http.StatusInternalServerError, ErrorResponse{
            Code:  "QUERY_FAILED",
            Error: err.Error(),
        })
        return
    }

    responses := make([]ONTMetricsResponse, len(metrics))
    for i, m := range metrics {
        responses[i] = ToONTMetricsResponse(&m)
    }

    c.JSON(http.StatusOK, gin.H{
        "data":  responses,
        "start": startTime,
        "end":   endTime,
        "count": len(responses),
    })
}
```

- [ ] **Step 3: Register metrics routes in router.go**

Add after ONT routes:

```go
metricsService := services.NewMetricsService(db)
metricsHandler := NewMetricsHandler(metricsService)

onts := api.Group("/onts")
onts.Use(middleware.AuthMiddleware(sessionStore, logger))
{
    // ... existing ONT CRUD routes ...
    
    // Metrics routes
    onts.GET("/:id/metrics", metricsHandler.GetLatest)
    onts.GET("/:id/metrics/history", metricsHandler.GetHistory)
}
```

- [ ] **Step 4: Build and restart API**

```bash
cd backend
go build -o api cmd/api/main.go
docker-compose up -d --build api
```

- [ ] **Step 5: Test metrics endpoints**

```bash
# Test latest metrics
curl -s -b /tmp/tikman_cookies.txt http://localhost:8080/api/v1/onts/SOME_ONT_ID/metrics | jq

# Test history (last 1 hour)
START=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)
END=$(date -u +%Y-%m-%dT%H:%M:%SZ)
curl -s -b /tmp/tikman_cookies.txt \
  "http://localhost:8080/api/v1/onts/SOME_ONT_ID/metrics/history?start=$START&end=$END" | jq
```

- [ ] **Step 6: Commit metrics API**

```bash
git add internal/api/metrics_handler.go internal/api/dto.go internal/api/router.go
git commit -m "feat(monitoring): add ONT metrics API endpoints"
```

---

## Task 5: Frontend Metrics Display

**Files:**
- Modify: `frontend/src/domain/entities/Ont.ts`
- Modify: `frontend/src/domain/repositories/IOntRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/OntRepository.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/application/hooks/useOntMetrics.ts`
- Modify: `frontend/src/presentation/pages/OntListPage.tsx`

**Interfaces:**
- Consumes: Metrics API from Task 4
- Produces: Signal level indicators in ONT list table

- [ ] **Step 1: Add ONTMetrics interface to Ont.ts**

```typescript
export interface OntMetrics {
  time: string;
  rxPower: number;
  txPower: number;
  temperature: number;
  voltage: number;
  distance: number;
  rxBytes: number;
  txBytes: number;
}
```

- [ ] **Step 2: Add metrics methods to IOntRepository**

```typescript
export interface IOntRepository {
  // ... existing methods ...
  getLatestMetrics(id: string): Promise<OntMetrics>;
  getMetricsHistory(id: string, start?: string, end?: string): Promise<{
    data: OntMetrics[];
    start: string;
    end: string;
    count: number;
  }>;
}
```

- [ ] **Step 3: Implement metrics methods in OntRepository**

```typescript
async getLatestMetrics(id: string): Promise<OntMetrics> {
  const response = await apiClient.get<OntMetrics>(
    API_ENDPOINTS.ONT_LATEST_METRICS(id)
  );
  return response.data;
}

async getMetricsHistory(
  id: string,
  start?: string,
  end?: string
): Promise<{ data: OntMetrics[]; start: string; end: string; count: number }> {
  const response = await apiClient.get(API_ENDPOINTS.ONT_METRICS_HISTORY(id), {
    params: { start, end },
  });
  return response.data;
}
```

- [ ] **Step 4: Add metrics endpoints**

```typescript
export const API_ENDPOINTS = {
  // ... existing endpoints ...
  ONT_LATEST_METRICS: (id: string) => `/api/v1/onts/${id}/metrics`,
  ONT_METRICS_HISTORY: (id: string) => `/api/v1/onts/${id}/metrics/history`,
} as const;
```

- [ ] **Step 5: Create useOntMetrics hook**

```typescript
import { useQuery } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";

const ontRepository = new OntRepository();

export function useOntMetrics(id: string, enabled = true) {
  return useQuery({
    queryKey: ["onts", id, "metrics"],
    queryFn: () => ontRepository.getLatestMetrics(id),
    enabled: enabled && !!id,
    refetchInterval: 300000, // 5 minutes
  });
}

export function useOntMetricsHistory(
  id: string,
  start?: string,
  end?: string
) {
  return useQuery({
    queryKey: ["onts", id, "metrics-history", start, end],
    queryFn: () => ontRepository.getMetricsHistory(id, start, end),
    enabled: !!id,
  });
}
```

- [ ] **Step 6: Add signal level column to OntListPage**

Add after Status column:

```typescript
{
  title: "Signal (Rx/Tx)",
  key: "signal",
  render: (_: unknown, record: Ont) => {
    const { data: metrics } = useOntMetrics(record.id, record.status === "online");
    
    if (!metrics) return <span>-</span>;
    
    const getRxColor = (power: number) => {
      if (power >= -25) return "green";
      if (power >= -27) return "orange";
      return "red";
    };
    
    const getTxColor = (power: number) => {
      if (power >= 0 && power <= 4) return "green";
      return "red";
    };
    
    return (
      <Space direction="vertical" size={0}>
        <span>
          Rx: <Tag color={getRxColor(metrics.rxPower)}>{metrics.rxPower.toFixed(2)} dBm</Tag>
        </span>
        <span>
          Tx: <Tag color={getTxColor(metrics.txPower)}>{metrics.txPower.toFixed(2)} dBm</Tag>
        </span>
      </Space>
    );
  },
},
```

- [ ] **Step 7: Add temperature and distance to detail modal**

Update handleViewDetail:

```typescript
const handleViewDetail = (ont: Ont) => {
  const { data: metrics } = useOntMetrics(ont.id, ont.status === "online");
  
  Modal.info({
    title: "ONT Details",
    width: 700,
    content: (
      <div>
        <p><strong>Serial Number:</strong> {ont.serialNumber}</p>
        {/* ... existing fields ... */}
        {metrics && (
          <>
            <Divider />
            <h4>Signal Metrics</h4>
            <p><strong>Rx Power:</strong> {metrics.rxPower.toFixed(2)} dBm</p>
            <p><strong>Tx Power:</strong> {metrics.txPower.toFixed(2)} dBm</p>
            <p><strong>Temperature:</strong> {metrics.temperature.toFixed(1)} °C</p>
            <p><strong>Voltage:</strong> {metrics.voltage.toFixed(2)} V</p>
            <p><strong>Distance:</strong> {metrics.distance} m</p>
            <p><strong>Last Updated:</strong> {new Date(metrics.time).toLocaleString()}</p>
          </>
        )}
      </div>
    ),
  });
};
```

- [ ] **Step 8: Build and test frontend**

```bash
cd frontend
npm run build
docker-compose up -d --build frontend
```

- [ ] **Step 9: Verify metrics display in browser**

Open http://localhost:3000, navigate to ONT Monitoring page, verify signal levels appear

- [ ] **Step 10: Commit frontend metrics display**

```bash
git add frontend/src/
git commit -m "feat(monitoring): add signal level indicators to ONT list"
```

---

## Final Verification

- [ ] All backend tests pass: `cd backend && go test ./... -v`
- [ ] Frontend builds successfully: `cd frontend && npm run build`
- [ ] Docker containers running: `docker-compose ps`
- [ ] Monitoring worker logs show metrics collection every 5 minutes
- [ ] Metrics API returns data for online ONTs
- [ ] Frontend displays signal levels with color indicators
- [ ] Continuous aggregates working: Check ont_metrics_5min table has data
