# ONT List Performance Optimization

## Problem

The ONT list endpoint (`GET /api/v1/onts`) was slow due to N+1 query problem when fetching metrics for each ONT.

**Before:**
- With 200 ONTs per request:
  - 1 query to list ONTs
  - 1 query to fetch OLT names
  - **200 individual queries** to fetch metrics (one per ONT)
  - **Total: 202 queries**

This resulted in:
- Response size: 106 kB
- Response time: 33-43 ms
- High database load

## Solution

Added batch metrics fetching to reduce queries from 200+ to just 3.

**After:**
- With 200 ONTs per request:
  - 1 query to list ONTs
  - 1 query to fetch OLT names
  - **1 batch query** to fetch all metrics at once
  - **Total: 3 queries**

### Implementation

**1. Added `GetLatestMetricsBatch()` to MetricsService**

Location: `backend/internal/services/metrics_service.go`

```go
func (s *MetricsService) GetLatestMetricsBatch(ontIDs []uuid.UUID) (map[uuid.UUID]*ONTMetricsRow, error)
```

Uses window function (compatible with both PostgreSQL and SQLite):
```sql
WITH ranked_metrics AS (
    SELECT 
        time, ont_id, rx_power, tx_power, temperature, voltage, tx_bias_current,
        distance, rx_bytes, tx_bytes, rx_packets, tx_packets, rx_errors, tx_errors,
        ROW_NUMBER() OVER (PARTITION BY ont_id ORDER BY time DESC) as rn
    FROM ont_metrics
    WHERE ont_id IN ?
)
SELECT * FROM ranked_metrics WHERE rn = 1
```

**2. Updated ONT Handler to use batch fetch**

Location: `backend/internal/api/ont_handler.go`

Changed from:
```go
for i, ont := range onts {
    metrics, _ := h.metricsService.GetLatestMetrics(ont.ID)  // N queries
    // ...
}
```

To:
```go
metricsMap, _ := h.metricsService.GetLatestMetricsBatch(ontIDs)  // 1 query
for i, ont := range onts {
    metrics := metricsMap[ont.ID]
    // ...
}
```

## Performance Results

**Test Results (50 ONTs, SQLite):**
- Batch fetch: 505.5µs
- Loop fetch: 835.2µs
- **Speedup: 1.65x**

**Expected Production Impact (200 ONTs, PostgreSQL):**
- Estimated speedup: **3-5x**
- Response time: ~10-15ms (down from 33-43ms)
- Database load: **98.5% reduction** in queries (202 → 3)

## Testing

Test file: `backend/internal/services/metrics_service_batch_test.go`

Run tests:
```bash
cd backend
go test ./internal/services -v -run TestGetLatestMetricsBatch
go test ./internal/services -v -run TestONTListPerformance
```

All tests pass ✅

## Verification

Build verification:
```bash
cd backend
go build -o /tmp/tikman-api cmd/api/main.go
```

No compilation errors ✅

## Files Changed

1. `backend/internal/services/metrics_service.go`
   - Added `GetLatestMetricsBatch()` method

2. `backend/internal/api/ont_handler.go`
   - Updated `List()` handler to use batch fetch

3. `backend/internal/services/metrics_service_batch_test.go` (new)
   - Comprehensive tests for batch fetching
   - Performance comparison test

## Deployment Notes

- Backward compatible - no schema changes required
- No frontend changes needed
- Safe to deploy immediately
- Monitor database query count after deployment to verify improvement
