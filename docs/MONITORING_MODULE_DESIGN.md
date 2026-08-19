# Monitoring Module Design Document

**Project:** TikMan - ZTE OLT Management System  
**Module:** Monitoring & Alerting  
**Version:** 1.0  
**Date:** 2026-08-15  
**Status:** Draft - Awaiting Approval

---

## 1. Overview

The Monitoring Module provides comprehensive real-time and historical monitoring of ZTE OLT devices and their connected ONTs (Optical Network Terminals). This module enables network operators to:

- Monitor ONT status (online/offline/LOS)
- Track signal quality metrics (RX/TX power, temperature)
- View traffic statistics and bandwidth usage
- Track OLT port status and utilization
- Receive alarms and event notifications
- View historical trends with time-series graphs
- Get proactive alerts for degraded service

---

## 2. Requirements

### 2.1 Functional Requirements

**FR-1: ONT Status Monitoring**
- Display online/offline/LOS status for each ONT
- Show last seen timestamp
- Track uptime/downtime duration
- Support bulk status queries (all ONTs on a port/OLT)

**FR-2: Signal Level Monitoring**
- RX optical power (dBm)
- TX optical power (dBm)
- ONT temperature (°C)
- Voltage levels
- Signal quality indicators

**FR-3: Traffic Statistics**
- Bandwidth usage (upload/download)
- Packet counters (unicast, multicast, broadcast)
- Error counters (CRC, FCS errors)
- Real-time throughput (Mbps)

**FR-4: OLT Port Monitoring**
- Port status (enabled/disabled)
- Port utilization percentage
- Connected ONT count per port
- Port-level alarms

**FR-5: Alarm & Event Tracking**
- Critical alarms (ONT offline, high temperature, signal loss)
- Warning alarms (signal degradation, high error rate)
- Event log (ONT registration, deregistration, reboots)
- Alarm acknowledgment and clearing

**FR-6: Real-time Dashboard**
- Auto-refresh every 30 seconds (configurable)
- Live status overview
- Critical alarm notifications
- Quick drill-down to problem areas

**FR-7: Historical Data & Graphs**
- Time-series graphs for signal levels
- Traffic trend graphs (hourly, daily, weekly)
- Customizable time ranges (1h, 24h, 7d, 30d)
- Export data to CSV

**FR-8: Alerting & Notifications**
- Email notifications for critical events
- Webhook integration for external systems
- In-app notification center
- Configurable alert thresholds
- Alert suppression for maintenance windows

### 2.2 Non-Functional Requirements

**NFR-1: Performance**
- Support monitoring 1000+ ONTs across multiple OLTs
- Dashboard load time < 2 seconds
- API response time < 500ms for real-time queries
- Polling interval: 1-5 minutes (configurable per metric type)

**NFR-2: Scalability**
- Horizontal scaling via worker pool
- Time-series data aggregation (5min → 1hour → 1day rollups)
- Efficient storage with data retention policies

**NFR-3: Reliability**
- Polling failure retry mechanism
- Graceful degradation if OLT unreachable
- No data loss during service restarts

**NFR-4: Data Retention**
- Raw metrics: 7 days
- 5-minute aggregates: 30 days
- 1-hour aggregates: 90 days
- 1-day aggregates: 1 year
- Alarms: 90 days
- Events: 1 year

---

## 3. Architecture

### 3.1 System Components

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (React)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Dashboard   │  │   Graphs     │  │   Alarms     │      │
│  │   (Live)     │  │ (Historical) │  │ (Real-time)  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Backend API (Go/Gin)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  Query API   │  │  Alert API   │  │  Config API  │      │
│  │ (REST/SSE)   │  │   (CRUD)     │  │  (Polling)   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Monitoring Worker Pool                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ SNMP Poller  │  │ SSH Poller   │  │Alert Engine  │      │
│  │   (Metrics)  │  │  (Commands)  │  │(Thresholds)  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Data Storage                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  PostgreSQL  │  │  TimescaleDB │  │    Redis     │      │
│  │ (Metadata)   │  │(Time-Series) │  │   (Cache)    │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    ZTE OLT Devices                           │
│         SNMP (UDP 161)  /  Telnet (TCP 23)                   │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 Data Flow

**Real-time Monitoring Flow:**
1. Worker polls OLT via SNMP (every 1-2 minutes for metrics)
2. Parse response and store in TimescaleDB
3. Check alert thresholds, trigger if exceeded
4. Cache latest values in Redis (TTL: 5 minutes)
5. Frontend queries API → API reads from Redis cache
6. SSE (Server-Sent Events) pushes updates to connected clients

**Historical Query Flow:**
1. Frontend requests graph data (time range + metric)
2. API queries TimescaleDB with aggregation
3. Return aggregated data points
4. Frontend renders chart with Chart.js or Recharts

**Alarm Flow:**
1. Alert Engine detects threshold breach
2. Create alarm record in PostgreSQL
3. Send notification (email/webhook) asynchronously
4. Push to frontend via SSE
5. User acknowledges alarm → update status

---

## 4. Database Schema

### 4.1 PostgreSQL (Metadata & Alarms)

```sql
-- ONTs table (extends existing OLT management)
CREATE TABLE onts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    olt_id UUID NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    port_id INT NOT NULL,              -- PON port number (0-15)
    ont_id INT NOT NULL,               -- ONT ID on port (0-127)
    serial_number VARCHAR(20) NOT NULL,
    description VARCHAR(255),
    status VARCHAR(20),                -- online, offline, los, unknown
    last_seen_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(olt_id, port_id, ont_id)
);

-- Alarms table
CREATE TABLE alarms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    olt_id UUID REFERENCES olts(id) ON DELETE CASCADE,
    ont_id UUID REFERENCES onts(id) ON DELETE CASCADE,
    severity VARCHAR(20) NOT NULL,     -- critical, warning, info
    type VARCHAR(50) NOT NULL,         -- ont_offline, signal_low, high_temperature
    message TEXT NOT NULL,
    metric_value DECIMAL(10,2),
    threshold_value DECIMAL(10,2),
    status VARCHAR(20) DEFAULT 'active', -- active, acknowledged, cleared
    occurred_at TIMESTAMP NOT NULL,
    acknowledged_at TIMESTAMP,
    acknowledged_by UUID REFERENCES users(id),
    cleared_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_alarms_status ON alarms(status);
CREATE INDEX idx_alarms_occurred_at ON alarms(occurred_at DESC);

-- Alert rules (user-configurable thresholds)
CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    metric_type VARCHAR(50) NOT NULL,  -- rx_power, temperature, error_rate
    condition VARCHAR(20) NOT NULL,    -- below, above, equals
    threshold DECIMAL(10,2) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    notify_email BOOLEAN DEFAULT FALSE,
    notify_webhook BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Notification settings
CREATE TABLE notification_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    email_enabled BOOLEAN DEFAULT FALSE,
    email_address VARCHAR(255),
    webhook_enabled BOOLEAN DEFAULT FALSE,
    webhook_url VARCHAR(500),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id)
);
```

### 4.2 TimescaleDB (Time-Series Metrics)

```sql
-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ONT metrics hypertable
CREATE TABLE ont_metrics (
    time TIMESTAMPTZ NOT NULL,
    ont_id UUID NOT NULL,
    rx_power DECIMAL(6,2),             -- dBm
    tx_power DECIMAL(6,2),             -- dBm
    temperature DECIMAL(5,2),          -- Celsius
    voltage DECIMAL(5,2),              -- Volts
    rx_bytes BIGINT,
    tx_bytes BIGINT,
    rx_packets BIGINT,
    tx_packets BIGINT,
    rx_errors BIGINT,
    tx_errors BIGINT
);

SELECT create_hypertable('ont_metrics', 'time');

-- Create indexes for efficient queries
CREATE INDEX idx_ont_metrics_ont_time ON ont_metrics(ont_id, time DESC);

-- Continuous aggregates for 5-minute rollups
CREATE MATERIALIZED VIEW ont_metrics_5min
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('5 minutes', time) AS bucket,
    ont_id,
    AVG(rx_power) AS avg_rx_power,
    MIN(rx_power) AS min_rx_power,
    MAX(rx_power) AS max_rx_power,
    AVG(tx_power) AS avg_tx_power,
    AVG(temperature) AS avg_temperature,
    MAX(temperature) AS max_temperature,
    SUM(rx_bytes) AS total_rx_bytes,
    SUM(tx_bytes) AS total_tx_bytes
FROM ont_metrics
GROUP BY bucket, ont_id;

-- Continuous aggregates for 1-hour rollups
CREATE MATERIALIZED VIEW ont_metrics_1hour
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 hour', bucket) AS bucket,
    ont_id,
    AVG(avg_rx_power) AS avg_rx_power,
    MIN(min_rx_power) AS min_rx_power,
    MAX(max_rx_power) AS max_rx_power,
    AVG(avg_temperature) AS avg_temperature,
    MAX(max_temperature) AS max_temperature,
    SUM(total_rx_bytes) AS total_rx_bytes,
    SUM(total_tx_bytes) AS total_tx_bytes
FROM ont_metrics_5min
GROUP BY bucket, ont_id;

-- Data retention policies
SELECT add_retention_policy('ont_metrics', INTERVAL '7 days');
SELECT add_retention_policy('ont_metrics_5min', INTERVAL '30 days');
SELECT add_retention_policy('ont_metrics_1hour', INTERVAL '90 days');
```

### 4.3 Redis Cache Structure

```
# Latest ONT status (TTL: 5 minutes)
ont:status:{ont_id} → JSON {status, last_seen, rx_power, tx_power, temp}

# Latest OLT port summary (TTL: 5 minutes)
olt:port:{olt_id}:{port_id} → JSON {ont_count, online_count, offline_count}

# Active alarms count (TTL: 1 minute)
alarms:active → COUNT

# Polling job queue
queue:poll:snmp → LIST of olt_ids
```

---

## 5. API Endpoints

### 5.1 ONT Management

```
GET    /api/v1/onts                    # List all ONTs with filters
GET    /api/v1/onts/:id                # Get ONT details
POST   /api/v1/onts                    # Create ONT
PUT    /api/v1/onts/:id                # Update ONT
DELETE /api/v1/onts/:id                # Delete ONT
GET    /api/v1/onts/:id/metrics        # Get latest metrics
GET    /api/v1/onts/:id/metrics/history # Get historical data
```

### 5.2 Monitoring Queries

```
GET    /api/v1/monitoring/dashboard    # Dashboard summary (all OLTs)
GET    /api/v1/monitoring/olts/:id     # OLT-specific dashboard
GET    /api/v1/monitoring/onts/:id     # ONT real-time status
GET    /api/v1/monitoring/stream       # SSE endpoint for live updates
```

### 5.3 Alarms

```
GET    /api/v1/alarms                  # List alarms with filters
GET    /api/v1/alarms/:id              # Get alarm details
POST   /api/v1/alarms/:id/acknowledge  # Acknowledge alarm
POST   /api/v1/alarms/:id/clear        # Clear alarm
GET    /api/v1/alarms/stats            # Alarm statistics
```

### 5.4 Alert Rules

```
GET    /api/v1/alert-rules             # List rules
POST   /api/v1/alert-rules             # Create rule
PUT    /api/v1/alert-rules/:id         # Update rule
DELETE /api/v1/alert-rules/:id         # Delete rule
POST   /api/v1/alert-rules/:id/test    # Test rule
```

### 5.5 Notifications

```
GET    /api/v1/notifications/settings  # Get user notification settings
PUT    /api/v1/notifications/settings  # Update settings
POST   /api/v1/notifications/test      # Send test notification
```

---

## 6. ZTE OLT Integration

### 6.1 SNMP OIDs (Primary Method)

ZTE C300/C320 SNMP OIDs for monitoring:

```
# ONT Status
.1.3.6.1.4.1.3902.1012.3.28.1.1.3.{rack}.{shelf}.{slot}.{port}.{ont_id}  # ONT state (1=online, 2=offline, 3=LOS)

# Signal Levels
.1.3.6.1.4.1.3902.1012.3.28.2.1.5.{rack}.{shelf}.{slot}.{port}.{ont_id}  # RX power (dBm * 10)
.1.3.6.1.4.1.3902.1012.3.28.2.1.3.{rack}.{shelf}.{slot}.{port}.{ont_id}  # TX power (dBm * 10)
.1.3.6.1.4.1.3902.1012.3.28.2.1.7.{rack}.{shelf}.{slot}.{port}.{ont_id}  # Temperature (°C)

# Traffic Counters
.1.3.6.1.4.1.3902.1012.3.28.2.1.21.{rack}.{shelf}.{slot}.{port}.{ont_id} # RX bytes
.1.3.6.1.4.1.3902.1012.3.28.2.1.22.{rack}.{shelf}.{slot}.{port}.{ont_id} # TX bytes

# Port Status
.1.3.6.1.4.1.3902.1012.3.28.1.1.1.{rack}.{shelf}.{slot}.{port}           # Port admin status
```

### 6.2 Telnet Commands (Fallback/Detail)

```bash
# Show ONT status
show gpon onu state gpon-olt_1/1/1

# Show ONT optical info
show gpon onu detail-info gpon-onu_1/1/1:1

# Show ONT traffic
show interface gpon-onu_1/1/1:1 traffic
```

### 6.3 Polling Strategy

**Tier 1: Critical Metrics (Every 1 minute)**
- ONT online/offline status
- Active alarms

**Tier 2: Performance Metrics (Every 5 minutes)**
- RX/TX power
- Temperature
- Error counters

**Tier 3: Traffic Statistics (Every 5 minutes)**
- Bandwidth usage
- Packet counters

**On-Demand (User request)**
- Detailed diagnostics via Telnet
- Configuration verification

---

## 7. Worker Implementation

### 7.1 Worker Pool Architecture

```go
type WorkerPool struct {
    numWorkers int
    jobs       chan MonitoringJob
    results    chan MonitoringResult
    wg         sync.WaitGroup
}

type MonitoringJob struct {
    OLTID     uuid.UUID
    OLT       *models.OLT
    JobType   string // "snmp_poll", "alarm_check"
    Priority  int
}

type MonitoringResult struct {
    OLTID     uuid.UUID
    Success   bool
    Metrics   []ONTMetric
    Errors    []error
}
```

### 7.2 Scheduler

```go
// Cron-like scheduler
func StartScheduler() {
    ticker1min := time.NewTicker(1 * time.Minute)
    ticker5min := time.NewTicker(5 * time.Minute)
    
    go func() {
        for {
            select {
            case <-ticker1min.C:
                ScheduleStatusPolls()
            case <-ticker5min.C:
                ScheduleMetricsPolls()
            }
        }
    }()
}
```

---

## 8. Frontend Components

### 8.1 Dashboard Page

**Components:**
- `MonitoringDashboard` - Main container
- `OLTStatusCard` - Per-OLT summary (online/offline ONT counts)
- `ActiveAlarmsWidget` - Critical alarms list
- `ThroughputChart` - Real-time traffic graph
- `SignalQualityHeatmap` - Visual signal strength across ONTs

### 8.2 ONT Detail Page

**Components:**
- `ONTStatusBadge` - Online/offline indicator
- `SignalMetricsCard` - RX/TX/Temp gauges
- `TrafficStatsCard` - Bandwidth usage
- `MetricsChart` - Historical trends (24h/7d/30d)
- `AlarmHistory` - ONT-specific alarms

### 8.3 Alarms Page

**Components:**
- `AlarmTable` - Filterable alarm list
- `AlarmFilters` - Severity/status/time filters
- `AlarmDetailModal` - Detailed alarm info
- `BulkAcknowledge` - Multi-select actions

### 8.4 Alert Rules Page

**Components:**
- `RulesList` - All configured rules
- `RuleEditor` - Create/edit rule form
- `RuleTestModal` - Test rule against current data

---

## 9. Technology Stack

### Backend
- **Go 1.21+** - Core API and workers
- **Gin** - HTTP framework
- **GORM** - PostgreSQL ORM
- **gosnmp** - SNMP polling
- **go-redis** - Redis client
- **cron** - Job scheduling
- **zap** - Structured logging

### Database
- **PostgreSQL 15+** - Metadata storage
- **TimescaleDB 2.11+** - Time-series extension
- **Redis 7+** - Caching and job queue

### Frontend
- **React 18+ with TypeScript**
- **Ant Design** - UI components
- **Recharts** - Charting library
- **React Query** - Data fetching
- **Server-Sent Events (SSE)** - Real-time updates

### Infrastructure
- **Docker & Docker Compose** - Containerization
- **Nginx** - Reverse proxy (production)

---

## 10. Implementation Phases

### Phase 1: Foundation (Week 1)
- Database schema setup (PostgreSQL + TimescaleDB)
- ONT CRUD API
- Basic SNMP poller (status only, 30-second interval)
- Simple dashboard showing ONT list with real-time status

### Phase 2: Metrics Collection (Week 2)
- Full SNMP polling (signal + traffic)
- Worker pool implementation with 30s/1m/5m tiers
- Redis caching layer
- Historical data storage in TimescaleDB
- Metrics API endpoints

### Phase 3: Visualization (Week 3)
- Real-time dashboard UI with auto-refresh
- Historical charts (signal levels, traffic)
- ONT detail page with live metrics
- SSE live updates (fallback: polling for mobile)

### Phase 4: Alerting (Week 4)
- Alert rules engine
- Alarm detection and storage
- Email notifications (HTML templates)
- Webhook integration (Discord/Slack/Generic)
- Alarms UI

### Phase 5: Optimization & Testing (Week 5)
- Performance tuning
- Continuous aggregates
- Data retention policies
- Load testing (1000+ ONTs)
- Documentation

### Phase 6: Mobile App (Future - Month 2+)
- JWT authentication for mobile
- React Native mobile app
- Push notifications (FCM)
- Offline mode with SQLite cache
- Biometric authentication
- App Store & Play Store deployment

---

## 11. Testing Strategy

### 11.1 Unit Tests
- SNMP OID parsing
- Alert threshold logic
- Data aggregation functions
- API handlers

### 11.2 Integration Tests
- End-to-end polling workflow
- Database writes and reads
- Alarm triggering
- Notification delivery

### 11.3 Performance Tests
- 1000 ONTs polling simulation
- Query performance on large datasets
- SSE connection scaling
- Redis cache hit rates

### 11.4 Manual Testing
- Test with real ZTE OLT (113.192.1.98)
- Verify SNMP OID correctness
- Test alarm thresholds
- UI responsiveness

---

## 12. Security Considerations

- **Authentication:** All monitoring APIs require valid session
- **Authorization:** RBAC enforcement (Admin, Technician, Viewer)
- **Rate Limiting:** Protect polling endpoints from abuse
- **Input Validation:** Sanitize time ranges and filters
- **Audit Trail:** Log all alarm acknowledgments
- **Webhook Security:** HMAC signature for outbound webhooks

---

## 13. Monitoring & Observability

- **Application Metrics:** Worker job success/failure rates
- **Performance Metrics:** API latency, query times
- **Business Metrics:** Total ONTs monitored, alarm resolution time
- **Health Checks:** Worker liveness, database connectivity
- **Logging:** Structured logs for debugging polling issues

---

## 14. Design Decisions (Resolved)

### 14.1 Polling Frequency ✅
**Decision:** Aggressive polling for real-time monitoring

- **ONT Status:** Every 30 seconds (critical for knowing client device state)
- **Critical Metrics:** Every 1 minute (signal levels)
- **Performance Metrics:** Every 5 minutes (temperature, voltage)
- **Traffic Statistics:** Every 5 minutes (bandwidth, counters)

**Rationale:** User needs real-time visibility of client device status. 30-second polling provides near-realtime updates while keeping server load manageable. Worker pool will handle concurrent polling across multiple OLTs.

### 14.2 Webhook Format ✅
**Decision:** Support multiple webhook formats (Discord, Slack, Generic)

**Generic HTTP POST (Default):**
```json
{
  "event": "alarm.triggered",
  "timestamp": "2026-08-15T10:30:00Z",
  "severity": "critical",
  "alarm": {
    "id": "uuid",
    "type": "ont_offline",
    "message": "ONT offline: ZTEG12345678",
    "olt": {
      "id": "uuid",
      "name": "OLT-Central-01",
      "ip_address": "113.192.1.98"
    },
    "ont": {
      "id": "uuid",
      "serial_number": "ZTEG12345678",
      "port": "1/1/1",
      "ont_id": 5
    }
  },
  "signature": "sha256=..."
}
```

**Discord Webhook:**
```json
{
  "embeds": [{
    "title": "🔴 Critical Alarm",
    "description": "ONT offline: ZTEG12345678",
    "color": 15158332,
    "fields": [
      {"name": "OLT", "value": "OLT-Central-01 (113.192.1.98)", "inline": true},
      {"name": "Port", "value": "1/1/1:5", "inline": true}
    ],
    "timestamp": "2026-08-15T10:30:00Z"
  }]
}
```

**Slack Incoming Webhook:**
```json
{
  "text": "🔴 *Critical Alarm*",
  "blocks": [
    {
      "type": "section",
      "text": {"type": "mrkdwn", "text": "*ONT Offline*\nSerial: ZTEG12345678"}
    },
    {
      "type": "context",
      "elements": [
        {"type": "mrkdwn", "text": "OLT: OLT-Central-01 | Port: 1/1/1:5"}
      ]
    }
  ]
}
```

### 14.3 Email Templates ✅
**Decision:** HTML emails with inline styles and embedded metrics

**Features:**
- Responsive HTML (mobile-friendly)
- Inline CSS (no external stylesheets)
- Alarm severity color coding (red=critical, yellow=warning)
- Embedded metric values (signal levels, uptime)
- Direct link to dashboard for details
- Plain text fallback for old email clients

### 14.4 Mobile App ✅
**Decision:** Mobile app as Phase 6 (future requirement)

**Current Design (Mobile-Ready):**
- REST API already mobile-friendly
- Token-based auth will be added alongside session cookies
- Polling fallback for mobile (SSE may not work reliably)
- Responsive web UI works on mobile browsers

**Future Mobile App Stack:**
- **React Native** (share code with web React)
- **Push Notifications** (Firebase Cloud Messaging)
- **Offline Mode** (SQLite cache)
- **Biometric Auth** (Face ID / Fingerprint)

**Architecture Note:** Current API design supports mobile without changes. Phase 6 will add:
- JWT tokens for mobile auth
- Push notification service
- Mobile-specific optimized endpoints

### 14.5 Multi-Tenancy ✅
**Decision:** Not required currently

Single-tenant deployment. All users in the system can see all OLTs/ONTs based on RBAC roles. No organization/tenant isolation needed at this time.

---

## 15. Success Criteria

- ✅ Monitor 1000+ ONTs across 10+ OLTs
- ✅ Dashboard loads in < 2 seconds
- ✅ Alarms triggered within 2 minutes of issue
- ✅ Historical graphs render < 1 second
- ✅ 99.9% polling success rate
- ✅ Zero data loss during restarts
- ✅ Email notifications sent < 30 seconds
- ✅ All tests pass (unit + integration)

---

## 16. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SNMP OID mismatch (different ZTE firmware) | High | Test with real device, document OID mapping |
| TimescaleDB complexity | Medium | Start with PostgreSQL, migrate later if needed |
| Polling overload (too many ONTs) | High | Worker pool with rate limiting, prioritize critical checks |
| False alarm storm | Medium | Alarm suppression, cooldown periods |
| SSE connection limits | Medium | Fallback to polling for old browsers |

---

**Next Steps:**
1. ✅ **Design document reviewed and approved**
2. ✅ **All open questions resolved**
3. **Ready to start Phase 1 implementation**
4. **Estimated timeline: 5 weeks (Phase 1-5), Mobile app later**

---

**Prepared by:** Claude (Kiro AI)  
**Review Status:** ✅ Approved  
**Start Date:** 2026-08-15  
**Estimated Completion:** Phase 1-5 by 2026-09-19, Phase 6 (Mobile) TBD
