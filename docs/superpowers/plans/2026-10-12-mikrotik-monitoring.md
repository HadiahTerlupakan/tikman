# Monitoring Router MikroTik (Sub-proyek 1) — Rencana Implementasi

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** TikMan memantau router border MikroTik (RouterOS v7): trafik per interface/VLAN lewat SNMP, status sesi BGP dan ringkasan router lewat REST, dengan halaman MikroTik di UI.

**Architecture:** Paket baru `internal/mikrotik` (klien REST dengan sertifikat yang dipin, parser nilai RouterOS) dan satu fungsi walk IF-MIB di `internal/connectivity`. Logika polling ada di `internal/services` (router CRUD, antrian `router_poll_jobs`, polling, query), dijalankan oleh `worker` yang sudah ada di samping antrian OLT. API `/api/v1/routers` dan halaman `/routers` di frontend.

**Tech Stack:** Go 1.26, Gin, GORM, gosnmp v1.45 (sudah ada), `net/http` + `crypto/tls` (tanpa dependensi baru), PostgreSQL/TimescaleDB, React + TypeScript, Ant Design 5, React Query 5, recharts 3, dayjs (transitif lewat antd, sudah dipakai di `ONTEventTimeline.tsx`).

**Spec:** `docs/superpowers/specs/2026-10-12-mikrotik-monitoring-design.md`

## Global Constraints

- Tidak ada dependensi baru, di backend maupun frontend.
- Akses ke router read-only. Tidak ada kode yang menulis konfigurasi router.
- Kredensial (`snmp_community`, `api_password`) dienkripsi AES-256-GCM lewat `utils.Encrypt`/`utils.Decrypt` dengan `cfg.EncryptionKey`, dan tidak pernah dikembalikan oleh endpoint mana pun (`json:"-"`).
- SNMP v2c saja. REST HTTPS ke port default **8443**. SNMP port default **161**.
- TLS ke router diverifikasi terhadap fingerprint SHA-256 sertifikat (64 karakter hex huruf kecil), bukan CA.
- Timeout: request SNMP 2 detik, request REST 5 detik.
- Interval: `interfaces` 1 menit, `bgp` 1 menit, `discovery` 6 jam.
- Identitas interface = `(router_id, name)`. `ifIndex` disimpan tetapi bukan kunci.
- Interface dinamis (`dynamic=true`) tidak dicatat.
- Retensi: raw 7 hari, rollup 5 menit 30 hari, rollup per jam 1 tahun.
- Repo ini **publik**: fixture dan test tidak boleh memuat IP publik, nama router, AS, serial number, atau community asli. Pakai 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24, 198.18.0.0/15 (untuk subnet /23 yang tidak muat di satu rentang dokumentasi), dan AS 64496–64511.
- Aturan CLAUDE.md berlaku: file ≤ 350 baris, fungsi ≤ 50 baris, nesting ≤ 3, komentar menjelaskan *kenapa*, tanpa TODO, tanpa `console.log`/`fmt.Println`.
- Migrasi berikutnya bernomor **56**.

## Review Focus

1. **Sesi BGP yang putus lalu naik lagi di antara dua poll.** Kejadian 2026-10-11 hanya 4 detik (putus 16:06:39, naik 16:06:43), jadi poll per menit melihat Established → Established. Harapan: tetap tercatat dua event (putus dan naik) dari perubahan `last-started`. Diuji di Task 5.
2. **Nama koneksi yang menjadi awalan koneksi lain** (`peer-ILIX-RS1` vs sesi `peer-ILIX-RS1-NEW-1`). Harapan: sesi RS1-NEW tidak menempel ke RS1. Diuji di Task 5.
3. **Waktu router dalam zona lokal router.** `last-started` tidak membawa zona. Harapan: dikonversi memakai `gmt-offset` dari `/system/clock`, bukan dianggap UTC. Diuji di Task 2 dan Task 8.
4. **Router mengembalikan daftar interface kosong** (REST error sebagian, atau user tanpa hak baca). Harapan: discovery gagal dengan error, bukan menghapus semua interface beserta grafiknya. Diuji di Task 8.
5. **Rentang grafik pendek tetapi jauh ke belakang** (1 jam, 10 hari lalu). Harapan: dibaca dari rollup yang masih menyimpan data, bukan tabel raw yang sudah dibuang retensi. Diuji di Task 10.

---

## Struktur file

Backend (dibuat):
- `backend/internal/models/mikrotik.go` — enam model dan konstantanya.
- `backend/migrations/56_mikrotik_monitoring.sql` — FK, hypertable, rollup.
- `backend/internal/database/migrations_mikrotik_postgres_test.go`
- `backend/internal/mikrotik/parse.go` + `parse_test.go` — parser durasi/angka/bool/waktu RouterOS.
- `backend/internal/mikrotik/types.go` — bentuk JSON REST.
- `backend/internal/mikrotik/client.go` + `client_test.go` — klien REST dengan pin fingerprint.
- `backend/internal/connectivity/snmp_ifxtable.go` — walk IF-MIB (dikecualikan dari batas baris, kode jaringan).
- `backend/internal/services/router_bgp.go` + `router_bgp_test.go` — merge connection/session, pencocokan interface, diff event.
- `backend/internal/services/router_rates.go` + `router_rates_test.go`
- `backend/internal/services/router_service.go` + `router_service_test.go` — CRUD.
- `backend/internal/services/router_connection.go` + `router_connection_test.go` — uji koneksi, rahasia, terima sertifikat.
- `backend/internal/services/router_poll_jobs.go` + `router_poll_jobs_test.go` + `router_poll_jobs_postgres_test.go`
- `backend/internal/services/router_poll.go` (interfaces + BGP) dan `router_discovery.go` + `router_poll_test.go` + `router_discovery_test.go`
- `backend/internal/services/router_query.go` + `router_query_test.go`
- `backend/cmd/worker/router_jobs.go`
- `backend/internal/api/router_dto.go`, `router_handler.go`, `router_handler_reads.go`, `router_routes_mikrotik.go`, `router_handler_test.go`

Backend (diubah): `internal/models/models.go`, `cmd/worker/jobs.go`, `cmd/worker/main.go`, `internal/api/router.go`, `internal/api/router_handlers.go`, `internal/api/router_rbac_test.go`.

Frontend (dibuat):
- `src/domain/entities/MikrotikRouter.ts`
- `src/infrastructure/repositories/MikrotikRouterRepository.ts`
- `src/application/hooks/useMikrotikRouters.ts`
- `src/presentation/components/routers/routerTraffic.ts` + `routerTraffic.test.ts`
- `src/presentation/components/routers/RouterStatusTag.tsx`, `TrafficRangePicker.tsx`, `TrafficChart.tsx`, `BgpSessionTable.tsx`, `BgpEventList.tsx`, `UpstreamTab.tsx`, `InterfacesTab.tsx`, `InterfaceTrafficModal.tsx`, `RouterInfoTab.tsx`, `RouterFormModal.tsx`
- `src/presentation/pages/RoutersPage.tsx`, `RouterDetailPage.tsx`

Frontend (diubah): `src/domain/entities/index.ts`, `src/infrastructure/repositories/index.ts`, `src/infrastructure/http/endpoints.ts`, `src/presentation/components/layout/navigationRoutes.tsx` + test, `src/presentation/routes/index.tsx`.

Docs (diubah): `docs/operator_guide.md`, `CLAUDE.md`.

---

### Task 1: Model dan migrasi

**Files:**
- Create: `backend/internal/models/mikrotik.go`
- Modify: `backend/internal/models/models.go` (daftar `AutoMigrate`)
- Create: `backend/migrations/56_mikrotik_monitoring.sql`
- Test: `backend/internal/database/migrations_mikrotik_postgres_test.go`

**Interfaces:**
- Produces: `models.MikrotikRouter`, `models.RouterInterface`, `models.RouterInterfaceMetric`, `models.RouterBGPSession`, `models.RouterBGPEvent`, `models.RouterPollJob`; tipe `RouterStatus` (`RouterStatusUnknown|Up|Down`), `RouterAPIStatus` (`RouterAPIStatusUnknown|OK|Error|CertificateChanged`), `BGPState` (`BGPStateEstablished|Down`), `RouterPollKind` (`RouterPollKindInterfaces|BGP|Discovery`).

- [ ] **Step 1: Tulis test migrasi yang gagal**

`backend/internal/database/migrations_mikrotik_postgres_test.go`:

```go
package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deleting a router has to take its interfaces, sessions, events and queue rows
// with it. SQLite builds no foreign keys, so only Postgres can show this.
func TestDeletingARouterRemovesWhatHangsOffIt(t *testing.T) {
	db := freshPostgres(t)

	routerID := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO mikrotik_routers
		(id, name, host, snmp_community, snmp_port, api_username, api_password, api_port,
		 tls_fingerprint, enabled, status, api_status, created_at, updated_at)
		VALUES (?, 'border', '192.0.2.1', 'x', 161, 'tikman', 'x', 8443, ?, true,
		 'unknown', 'unknown', now(), now())`,
		routerID, "0000000000000000000000000000000000000000000000000000000000000000").Error)
	require.NoError(t, db.Exec(`INSERT INTO router_interfaces
		(id, router_id, name, comment, type, disabled, oper_status, addresses, created_at, updated_at)
		VALUES (?, ?, 'vlan-ix', '', 'vlan', false, 'up', '', now(), now())`, uuid.New(), routerID).Error)
	require.NoError(t, db.Exec(`INSERT INTO router_poll_jobs (router_id, kind, due_at)
		VALUES (?, 'bgp', now())`, routerID).Error)

	require.NoError(t, db.Exec(`DELETE FROM mikrotik_routers WHERE id = ?`, routerID).Error)

	for _, table := range []string{"router_interfaces", "router_poll_jobs"} {
		var left int64
		require.NoError(t, db.Raw("SELECT count(*) FROM "+table+" WHERE router_id = ?", routerID).Scan(&left).Error)
		assert.Zero(t, left, table)
	}
}

func TestRouterInterfaceMetricsIsAHypertableWithRollups(t *testing.T) {
	db := freshPostgres(t)

	var hypertables int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.hypertables
		WHERE hypertable_name = 'router_interface_metrics'`).Scan(&hypertables).Error)
	assert.EqualValues(t, 1, hypertables)

	var views int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM timescaledb_information.continuous_aggregates
		WHERE view_name IN ('router_interface_metrics_5min', 'router_interface_metrics_hourly')`).Scan(&views).Error)
	assert.EqualValues(t, 2, views)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && TEST_POSTGRES_DSN="$TEST_POSTGRES_DSN" go test ./internal/database/ -run 'TestDeletingARouter|TestRouterInterfaceMetrics' -v`
Expected: FAIL, `relation "mikrotik_routers" does not exist`.

Bila test lain di paket ini gagal karena `pg_trgm`/continuous aggregate lama, itu masalah database test basi (lihat memory *Test Postgres stale schema*): buat ulang database `tikman-testpg`, jangan ubah kode.

- [ ] **Step 3: Tulis model**

`backend/internal/models/mikrotik.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RouterStatus is what SNMP last said about a router.
type RouterStatus string

const (
	RouterStatusUnknown RouterStatus = "unknown"
	RouterStatusUp      RouterStatus = "up"
	RouterStatusDown    RouterStatus = "down"
)

// RouterAPIStatus is what the REST API last said, kept apart from RouterStatus
// because either can work while the other does not.
type RouterAPIStatus string

const (
	RouterAPIStatusUnknown RouterAPIStatus = "unknown"
	RouterAPIStatusOK      RouterAPIStatus = "ok"
	RouterAPIStatusError   RouterAPIStatus = "error"
	// RouterAPIStatusCertificateChanged stops REST polling until an admin
	// accepts the new certificate: trusting it silently would defeat the pin.
	RouterAPIStatusCertificateChanged RouterAPIStatus = "certificate_changed"
)

// BGPState is a configured BGP connection's state as TikMan reports it.
type BGPState string

const (
	BGPStateEstablished BGPState = "established"
	BGPStateDown        BGPState = "down"
)

// RouterPollKind is one tier of polling work for a router.
type RouterPollKind string

const (
	RouterPollKindInterfaces RouterPollKind = "interfaces"
	RouterPollKindBGP        RouterPollKind = "bgp"
	RouterPollKindDiscovery  RouterPollKind = "discovery"
)

// MikrotikRouter is a RouterOS v7 router read over SNMP (traffic) and REST
// (BGP, inventory, resources).
//
// The tags must match migrations/56_mikrotik_monitoring.sql: AutoMigrate runs
// first, so a disagreeing tag wins and the migration silently does nothing.
type MikrotikRouter struct {
	ID     uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Name   string     `gorm:"type:varchar(255);not null" json:"name"`
	Host   string     `gorm:"type:varchar(255);not null" json:"host"`
	SiteID *uuid.UUID `gorm:"type:uuid" json:"site_id"`

	SNMPCommunity string `gorm:"type:text;not null" json:"-"`
	SNMPPort      int    `gorm:"not null" json:"snmp_port"`
	APIUsername   string `gorm:"type:varchar(64);not null" json:"api_username"`
	APIPassword   string `gorm:"type:text;not null" json:"-"`
	APIPort       int    `gorm:"not null" json:"api_port"`
	// No default on Enabled: GORM replaces a false zero value with the column
	// default on insert, which would make "add disabled" impossible.
	Enabled bool `gorm:"not null" json:"enabled"`

	TLSFingerprint       string  `gorm:"type:varchar(64);not null" json:"tls_fingerprint"`
	PresentedFingerprint *string `gorm:"type:varchar(64)" json:"presented_fingerprint"`

	Status    RouterStatus    `gorm:"type:varchar(16);not null" json:"status"`
	APIStatus RouterAPIStatus `gorm:"type:varchar(32);not null" json:"api_status"`
	APIError  *string         `gorm:"type:text" json:"api_error"`

	CPULoad       *int64     `json:"cpu_load"`
	MemoryTotal   *int64     `json:"memory_total"`
	MemoryFree    *int64     `json:"memory_free"`
	UptimeSeconds *int64     `json:"uptime_seconds"`
	ROSVersion    *string    `gorm:"type:varchar(64)" json:"ros_version"`
	BoardName     *string    `gorm:"type:varchar(64)" json:"board_name"`
	LastSeenAt    *time.Time `json:"last_seen_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *MikrotikRouter) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for MikrotikRouter.
func (MikrotikRouter) TableName() string { return "mikrotik_routers" }

// RouterInterface is one interface of a router, VLANs included.
//
// Its identity is the name, not ifIndex: RouterOS renumbers ifIndex when
// interfaces are added or removed, and keying on it would swap two VLANs'
// graphs without anything noticing.
type RouterInterface struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_router_interfaces_router_name" json:"router_id"`
	Name       string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_router_interfaces_router_name" json:"name"`
	Comment    string    `gorm:"type:text;not null" json:"comment"`
	Type       string    `gorm:"type:varchar(32);not null" json:"type"`
	VLANID     *int      `json:"vlan_id"`
	ParentName *string   `gorm:"type:varchar(255)" json:"parent_name"`
	IfIndex    *int      `json:"if_index"`
	SpeedMbps  *int64    `json:"speed_mbps"`
	Disabled   bool      `gorm:"not null" json:"disabled"`
	OperStatus string    `gorm:"type:varchar(8);not null" json:"oper_status"`
	// Comma-separated CIDRs from /ip/address. Text rather than cidr[] so the
	// SQLite test database can hold it too.
	Addresses string `gorm:"type:text;not null" json:"addresses"`

	LastInBps    *float64   `json:"last_in_bps"`
	LastOutBps   *float64   `json:"last_out_bps"`
	LastPolledAt *time.Time `json:"last_polled_at"`

	// Counter64 values stored as int64. The conversion keeps every bit, so
	// uint64(stored) gives back the exact counter even past 2^63.
	PrevInOctets  *int64     `json:"-"`
	PrevOutOctets *int64     `json:"-"`
	PrevSampleAt  *time.Time `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (i *RouterInterface) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterInterface.
func (RouterInterface) TableName() string { return "router_interfaces" }

// RouterInterfaceMetric is one per-minute traffic sample. The table is a
// TimescaleDB hypertable; it has no primary key because a hypertable's unique
// keys must include the time column and nothing reads a sample by key.
type RouterInterfaceMetric struct {
	Time        time.Time `gorm:"not null;index:idx_router_interface_metrics_if_time,priority:2" json:"time"`
	InterfaceID uuid.UUID `gorm:"type:uuid;not null;index:idx_router_interface_metrics_if_time,priority:1" json:"interface_id"`
	InBps       float64   `gorm:"not null" json:"in_bps"`
	OutBps      float64   `gorm:"not null" json:"out_bps"`
}

// TableName returns the table name for RouterInterfaceMetric.
func (RouterInterfaceMetric) TableName() string { return "router_interface_metrics" }

// RouterBGPSession is one configured BGP connection and the state of its
// running session.
type RouterBGPSession struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_router_bgp_sessions_router_name" json:"router_id"`
	Name          string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_router_bgp_sessions_router_name" json:"name"`
	RemoteAddress string     `gorm:"type:varchar(64);not null" json:"remote_address"`
	RemoteAS      int64      `gorm:"not null" json:"remote_as"`
	LocalAddress  string     `gorm:"type:varchar(64);not null" json:"local_address"`
	State         BGPState   `gorm:"type:varchar(16);not null" json:"state"`
	UptimeSeconds int64      `gorm:"not null" json:"uptime_seconds"`
	PrefixCount   int64      `gorm:"not null" json:"prefix_count"`
	LastStartedAt *time.Time `json:"last_started_at"`
	LastStoppedAt *time.Time `json:"last_stopped_at"`
	InterfaceID   *uuid.UUID `gorm:"type:uuid" json:"interface_id"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *RouterBGPSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterBGPSession.
func (RouterBGPSession) TableName() string { return "router_bgp_sessions" }

// RouterBGPEvent records a session changing state.
type RouterBGPEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	RouterID    uuid.UUID `gorm:"type:uuid;not null;index:idx_router_bgp_events_router_time,priority:1" json:"router_id"`
	SessionName string    `gorm:"type:varchar(255);not null" json:"session_name"`
	FromState   BGPState  `gorm:"type:varchar(16);not null" json:"from_state"`
	ToState     BGPState  `gorm:"type:varchar(16);not null" json:"to_state"`
	OccurredAt  time.Time `gorm:"not null;index:idx_router_bgp_events_router_time,priority:2" json:"occurred_at"`
}

func (e *RouterBGPEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name for RouterBGPEvent.
func (RouterBGPEvent) TableName() string { return "router_bgp_events" }

// RouterPollJob is one router's schedule for one tier, and the row a worker
// claims to take that work. Same shape as OLTPollJob, keyed on the router.
type RouterPollJob struct {
	RouterID uuid.UUID      `gorm:"type:uuid;not null;primaryKey" json:"router_id"`
	Kind     RouterPollKind `gorm:"type:varchar(16);not null;primaryKey" json:"kind"`

	DueAt    time.Time  `gorm:"not null;index" json:"due_at"`
	LockedBy *string    `gorm:"type:varchar(64)" json:"locked_by"`
	LockedAt *time.Time `json:"locked_at"`

	LastRunAt           *time.Time `json:"last_run_at"`
	LastDurationMs      *int64     `json:"last_duration_ms"`
	LastError           *string    `json:"last_error"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
}

// TableName returns the table name for RouterPollJob.
func (RouterPollJob) TableName() string { return "router_poll_jobs" }
```

Di `backend/internal/models/models.go`, tambahkan ke akhir daftar `db.AutoMigrate(`:

```go
		&CSWait{},
		&MikrotikRouter{},
		&RouterInterface{},
		&RouterInterfaceMetric{},
		&RouterBGPSession{},
		&RouterBGPEvent{},
		&RouterPollJob{},
	)
```

- [ ] **Step 4: Tulis migrasi**

`backend/migrations/56_mikrotik_monitoring.sql`:

```sql
-- MikroTik border routers. AutoMigrate builds the tables from
-- internal/models/mikrotik.go; this adds what tags cannot express: foreign
-- keys, the hypertable and its rollups.

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_site_fk;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_site_fk
    FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL;

ALTER TABLE router_interfaces DROP CONSTRAINT IF EXISTS router_interfaces_router_fk;
ALTER TABLE router_interfaces ADD CONSTRAINT router_interfaces_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_bgp_sessions DROP CONSTRAINT IF EXISTS router_bgp_sessions_router_fk;
ALTER TABLE router_bgp_sessions ADD CONSTRAINT router_bgp_sessions_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- A session whose interface disappears in discovery keeps its row; the next
-- match fills the column again.
ALTER TABLE router_bgp_sessions DROP CONSTRAINT IF EXISTS router_bgp_sessions_interface_fk;
ALTER TABLE router_bgp_sessions ADD CONSTRAINT router_bgp_sessions_interface_fk
    FOREIGN KEY (interface_id) REFERENCES router_interfaces(id) ON DELETE SET NULL;

ALTER TABLE router_bgp_events DROP CONSTRAINT IF EXISTS router_bgp_events_router_fk;
ALTER TABLE router_bgp_events ADD CONSTRAINT router_bgp_events_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

ALTER TABLE router_poll_jobs DROP CONSTRAINT IF EXISTS router_poll_jobs_router_fk;
ALTER TABLE router_poll_jobs ADD CONSTRAINT router_poll_jobs_router_fk
    FOREIGN KEY (router_id) REFERENCES mikrotik_routers(id) ON DELETE CASCADE;

-- No foreign key from the metrics to router_interfaces. A cascading delete into
-- compressed chunks is the expensive path TimescaleDB handles worst, and an
-- orphaned sample is harmless: every read filters by interface_id, and
-- retention drops it within the week.
SELECT create_hypertable('router_interface_metrics', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

ALTER TABLE router_interface_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'interface_id',
    timescaledb.compress_orderby = 'time DESC'
);

-- Two days, as for ont_metrics: the rollups refresh from the last hours, and
-- compressing what they still read would slow every refresh.
SELECT add_compression_policy('router_interface_metrics', INTERVAL '2 days', if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics', INTERVAL '7 days', if_not_exists => TRUE);

CREATE MATERIALIZED VIEW IF NOT EXISTS router_interface_metrics_5min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    interface_id,
    AVG(in_bps) AS avg_in_bps,
    MAX(in_bps) AS max_in_bps,
    AVG(out_bps) AS avg_out_bps,
    MAX(out_bps) AS max_out_bps
FROM router_interface_metrics
GROUP BY bucket, interface_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_interface_metrics_5min',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics_5min', INTERVAL '30 days', if_not_exists => TRUE);

CREATE MATERIALIZED VIEW IF NOT EXISTS router_interface_metrics_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    interface_id,
    AVG(in_bps) AS avg_in_bps,
    MAX(in_bps) AS max_in_bps,
    AVG(out_bps) AS avg_out_bps,
    MAX(out_bps) AS max_out_bps
FROM router_interface_metrics
GROUP BY bucket, interface_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('router_interface_metrics_hourly',
    start_offset => INTERVAL '6 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);
SELECT add_retention_policy('router_interface_metrics_hourly', INTERVAL '1 year', if_not_exists => TRUE);
```

- [ ] **Step 5: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/database/ -run 'TestEveryMigrationAppliesToAFreshSchema|TestDeletingARouter|TestRouterInterfaceMetrics' -v`
Expected: PASS ketiganya.

Run: `cd backend && go build ./... && go test ./internal/models/ ./internal/api/ -count=1`
Expected: PASS. Test API memakai `models.AutoMigrate` di SQLite, jadi ini membuktikan model baru bisa dibuat di SQLite juga.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/mikrotik.go backend/internal/models/models.go \
  backend/migrations/56_mikrotik_monitoring.sql \
  backend/internal/database/migrations_mikrotik_postgres_test.go
git commit -m "feat(mikrotik): add router, interface, BGP and poll job tables"
```

---

### Task 2: Parser nilai RouterOS

**Files:**
- Create: `backend/internal/mikrotik/parse.go`
- Test: `backend/internal/mikrotik/parse_test.go`

**Interfaces:**
- Produces: `mikrotik.ParseDuration(s string) (time.Duration, error)`, `mikrotik.ParseInt(s string) (int64, error)`, `mikrotik.ParseBool(s string) bool`, `mikrotik.ParseOffset(s string) (*time.Location, error)`, `mikrotik.ParseTime(s string, loc *time.Location) (time.Time, error)`.

- [ ] **Step 1: Tulis test yang gagal**

`backend/internal/mikrotik/parse_test.go`:

```go
package mikrotik

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"4w6d1h44m21s", 34*24*time.Hour + time.Hour + 44*time.Minute + 21*time.Second},
		{"15h53m9s990ms", 15*time.Hour + 53*time.Minute + 9*time.Second + 990*time.Millisecond},
		{"1d14h52m22s720ms", 38*time.Hour + 52*time.Minute + 22*time.Second + 720*time.Millisecond},
		{"1m30s", 90 * time.Second},
		{"0s", 0},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestParseDurationRejectsWhatIsNotADuration(t *testing.T) {
	for _, in := range []string{"", "infinity", "12", "5x", "h5"} {
		_, err := ParseDuration(in)
		assert.Error(t, err, in)
	}
}

func TestParseInt(t *testing.T) {
	got, err := ParseInt("1093192")
	require.NoError(t, err)
	assert.EqualValues(t, 1093192, got)

	_, err = ParseInt("")
	assert.Error(t, err)
	_, err = ParseInt("12k")
	assert.Error(t, err)
}

func TestParseBool(t *testing.T) {
	assert.True(t, ParseBool("true"))
	assert.True(t, ParseBool("yes"))
	assert.False(t, ParseBool("false"))
	assert.False(t, ParseBool(""))
}

// last-started carries no zone. A router on WIB that says 16:06:43 means
// 09:06:43 UTC, and reading it as UTC would place every event seven hours late.
func TestParseTimeUsesTheRoutersOffset(t *testing.T) {
	loc, err := ParseOffset("+07:00")
	require.NoError(t, err)

	got, err := ParseTime("2026-10-11 16:06:43", loc)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 11, 9, 6, 43, 0, time.UTC), got.UTC())
}

func TestParseOffset(t *testing.T) {
	for in, seconds := range map[string]int{"+07:00": 7 * 3600, "-03:30": -(3*3600 + 1800), "+00:00": 0} {
		loc, err := ParseOffset(in)
		require.NoError(t, err, in)
		_, offset := time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Zone()
		assert.Equal(t, seconds, offset, in)
	}
	for _, in := range []string{"", "7", "+7", "WIB"} {
		_, err := ParseOffset(in)
		assert.Error(t, err, in)
	}
}

func TestParseTimeRejectsEmpty(t *testing.T) {
	_, err := ParseTime("", time.UTC)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/mikrotik/ -v`
Expected: FAIL, `undefined: ParseDuration` (paket belum ada).

- [ ] **Step 3: Implementasi**

`backend/internal/mikrotik/parse.go`:

```go
// Package mikrotik reads RouterOS v7 routers over their REST API.
package mikrotik

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// routerTimeLayout is how RouterOS 7.10+ prints a timestamp such as
// last-started. It carries no zone: the router's clock zone applies.
const routerTimeLayout = "2006-01-02 15:04:05"

var durationUnits = map[string]time.Duration{
	"w":  7 * 24 * time.Hour,
	"d":  24 * time.Hour,
	"h":  time.Hour,
	"m":  time.Minute,
	"s":  time.Second,
	"ms": time.Millisecond,
	"us": time.Microsecond,
}

// ParseDuration reads a RouterOS duration such as "4w6d1h44m21s" or
// "15h53m9s990ms". time.ParseDuration rejects both: it knows neither weeks nor
// days.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var total time.Duration
	for rest := s; rest != ""; {
		digits := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
		if digits <= 0 {
			return 0, fmt.Errorf("duration %q: expected a number followed by a unit", s)
		}
		n, err := strconv.ParseInt(rest[:digits], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("duration %q: %w", s, err)
		}
		rest = rest[digits:]
		unitEnd := strings.IndexFunc(rest, func(r rune) bool { return r >= '0' && r <= '9' })
		if unitEnd < 0 {
			unitEnd = len(rest)
		}
		unit, ok := durationUnits[rest[:unitEnd]]
		if !ok {
			return 0, fmt.Errorf("duration %q: unknown unit %q", s, rest[:unitEnd])
		}
		total += time.Duration(n) * unit
		rest = rest[unitEnd:]
	}
	return total, nil
}

// ParseInt reads a number the REST API sent as a string, as it sends every
// value.
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// ParseBool reads a RouterOS flag. Anything but "true" or "yes" is false, which
// is also what an absent flag means.
func ParseBool(s string) bool {
	return s == "true" || s == "yes"
}

// ParseOffset reads /system/clock's gmt-offset, such as "+07:00".
func ParseOffset(s string) (*time.Location, error) {
	if len(s) != len("+07:00") || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return nil, fmt.Errorf("gmt-offset %q: want ±HH:MM", s)
	}
	hours, errH := strconv.Atoi(s[1:3])
	minutes, errM := strconv.Atoi(s[4:6])
	if errH != nil || errM != nil {
		return nil, fmt.Errorf("gmt-offset %q: want ±HH:MM", s)
	}
	seconds := hours*3600 + minutes*60
	if s[0] == '-' {
		seconds = -seconds
	}
	return time.FixedZone(s, seconds), nil
}

// ParseTime reads a RouterOS timestamp in the router's own zone.
func ParseTime(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	return time.ParseInLocation(routerTimeLayout, s, loc)
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/mikrotik/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/mikrotik/parse.go backend/internal/mikrotik/parse_test.go
git commit -m "feat(mikrotik): parse RouterOS durations, numbers and timestamps"
```

---

### Task 3: Klien REST dengan sertifikat yang dipin

**Files:**
- Create: `backend/internal/mikrotik/types.go`
- Create: `backend/internal/mikrotik/client.go`
- Test: `backend/internal/mikrotik/client_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces:
  - `mikrotik.ErrCertificateChanged`, `mikrotik.ErrUnauthorized`
  - `mikrotik.Fingerprint(der []byte) string`
  - `mikrotik.FetchFingerprint(host string, port int) (string, error)`
  - `mikrotik.NewClient(host string, port int, username, password, fingerprint string) *Client`
  - Methods on `*Client`: `SystemResource() (SystemResource, error)`, `Clock() (Clock, error)`, `Interfaces() ([]Interface, error)`, `VLANs() ([]VLAN, error)`, `Addresses() ([]Address, error)`, `BGPConnections() ([]BGPConnection, error)`, `BGPSessions() ([]BGPSession, error)`
  - Struct `SystemResource{Uptime, Version, CPULoad, FreeMemory, TotalMemory, BoardName string}`, `Clock{GMTOffset string}`, `Interface{Name, Type, Comment, Disabled, Running, Dynamic string}`, `VLAN{Name, VLANID, Interface, Disabled string}`, `Address{Address, Interface, Disabled, Invalid string}`, `BGPConnection{Name, RemoteAddress, RemoteAS, Disabled string}`, `BGPSession{Name, RemoteAddress, RemoteAS, LocalAddress, Established, Uptime, PrefixCount, LastStarted, LastStopped string}`.

- [ ] **Step 1: Tulis test yang gagal**

`backend/internal/mikrotik/client_test.go`:

```go
package mikrotik

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A trimmed, documentation-range copy of what /rest/routing/bgp/session
// returns. Every value is a string, as RouterOS sends it.
const sessionsJSON = `[
 {".id":"*1","name":"peer-transit-1","remote.address":"198.51.100.1","remote.as":"64500",
  "local.address":"198.51.100.0","established":"true","uptime":"15h53m9s990ms",
  "prefix-count":"1093192","last-started":"2026-10-11 16:06:43","last-stopped":"2026-10-11 16:06:39"},
 {".id":"*2","name":"peer-ix-rs1-new-1","remote.address":"203.0.113.166","remote.as":"0",
  "local.address":"203.0.113.140","prefix-count":"0"}
]`

func tlsRouter(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string, int) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return server, host, port
}

func TestClientReadsSessionsFromAPinnedRouter(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "tikman" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "/rest/routing/bgp/session", r.URL.Path)
		_, _ = w.Write([]byte(sessionsJSON))
	})

	client := NewClient(host, port, "tikman", "secret", Fingerprint(server.Certificate().Raw))
	sessions, err := client.BGPSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 2)
	assert.Equal(t, "peer-transit-1", sessions[0].Name)
	assert.Equal(t, "198.51.100.1", sessions[0].RemoteAddress)
	assert.Equal(t, "true", sessions[0].Established)
	assert.Equal(t, "1093192", sessions[0].PrefixCount)
	assert.Equal(t, "2026-10-11 16:06:39", sessions[0].LastStopped)
	assert.Empty(t, sessions[1].Established)
}

// The router's certificate is self-signed, so the pin is the only thing
// standing between the stored password and whoever answers on that address.
func TestClientRefusesARouterWhoseCertificateChanged(t *testing.T) {
	called := false
	_, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	})

	pinned := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := NewClient(host, port, "tikman", "secret", pinned).SystemResource()

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.False(t, called, "credentials must not reach a router that failed the pin")
}

func TestClientReportsRejectedCredentials(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := NewClient(host, port, "tikman", "wrong", Fingerprint(server.Certificate().Raw)).SystemResource()

	assert.True(t, errors.Is(err, ErrUnauthorized), "got %v", err)
}

func TestClientReportsAnHTTPErrorWithItsBody(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":400,"message":"no such command"}`))
	})

	_, err := NewClient(host, port, "u", "p", Fingerprint(server.Certificate().Raw)).Clock()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
	assert.Contains(t, err.Error(), "no such command")
}

func TestFetchFingerprintReturnsWhatTheRouterPresents(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {})

	got, err := FetchFingerprint(host, port)

	require.NoError(t, err)
	assert.Equal(t, Fingerprint(server.Certificate().Raw), got)
	assert.Len(t, got, 64)
}

func TestFetchFingerprintFailsWhenNothingListens(t *testing.T) {
	_, err := FetchFingerprint("127.0.0.1", 1)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/mikrotik/ -run 'TestClient|TestFetchFingerprint' -v`
Expected: FAIL, `undefined: NewClient`.

- [ ] **Step 3: Implementasi tipe**

`backend/internal/mikrotik/types.go`:

```go
package mikrotik

// The REST API sends every value as a string, flags included ("true"), and
// keeps RouterOS's dotted property names ("remote.address"). These structs
// mirror that; parse.go turns the strings into values.

// SystemResource is /rest/system/resource.
type SystemResource struct {
	Uptime      string `json:"uptime"`
	Version     string `json:"version"`
	CPULoad     string `json:"cpu-load"`
	FreeMemory  string `json:"free-memory"`
	TotalMemory string `json:"total-memory"`
	BoardName   string `json:"board-name"`
}

// Clock is /rest/system/clock. Only the offset is read: it is what turns the
// zone-less timestamps elsewhere into instants.
type Clock struct {
	GMTOffset string `json:"gmt-offset"`
}

// Interface is one row of /rest/interface.
type Interface struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Comment  string `json:"comment"`
	Disabled string `json:"disabled"`
	Running  string `json:"running"`
	Dynamic  string `json:"dynamic"`
}

// VLAN is one row of /rest/interface/vlan.
type VLAN struct {
	Name      string `json:"name"`
	VLANID    string `json:"vlan-id"`
	Interface string `json:"interface"`
	Disabled  string `json:"disabled"`
}

// Address is one row of /rest/ip/address.
type Address struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Disabled  string `json:"disabled"`
	Invalid   string `json:"invalid"`
}

// BGPConnection is one configured peer, from /rest/routing/bgp/connection.
type BGPConnection struct {
	Name          string `json:"name"`
	RemoteAddress string `json:"remote.address"`
	RemoteAS      string `json:"remote.as"`
	Disabled      string `json:"disabled"`
}

// BGPSession is one running session, from /rest/routing/bgp/session.
type BGPSession struct {
	Name          string `json:"name"`
	RemoteAddress string `json:"remote.address"`
	RemoteAS      string `json:"remote.as"`
	LocalAddress  string `json:"local.address"`
	Established   string `json:"established"`
	Uptime        string `json:"uptime"`
	PrefixCount   string `json:"prefix-count"`
	LastStarted   string `json:"last-started"`
	LastStopped   string `json:"last-stopped"`
}
```

- [ ] **Step 4: Implementasi klien**

`backend/internal/mikrotik/client.go`:

```go
package mikrotik

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// requestTimeout bounds one REST call. The router answers these in tens of
// milliseconds; anything near this long means it is unreachable, and the job
// should give up rather than hold a worker.
const requestTimeout = 5 * time.Second

// errorBodyLimit is how much of a failed response is kept for the message.
const errorBodyLimit = 512

var (
	// ErrCertificateChanged means the router presented a certificate other
	// than the one pinned when it was added.
	ErrCertificateChanged = errors.New("router certificate changed since it was trusted")
	// ErrUnauthorized means the router rejected the username or password.
	ErrUnauthorized = errors.New("router rejected the API credentials")
)

// Client reads one router's REST API.
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewClient builds a client that only talks to a router presenting the
// certificate with the given SHA-256 fingerprint.
func NewClient(host string, port int, username, password, fingerprint string) *Client {
	return &Client{
		baseURL:  "https://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/rest",
		username: username,
		password: password,
		http: &http.Client{
			Timeout:   requestTimeout,
			Transport: &http.Transport{TLSClientConfig: pinnedTLS(fingerprint)},
		},
	}
}

// pinnedTLS trusts exactly one certificate. The router's is self-signed, so
// there is no CA to verify against; the pin replaces that check rather than
// dropping it.
func pinnedTLS(fingerprint string) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // verification is VerifyPeerCertificate's pin below
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 || Fingerprint(raw[0]) != strings.ToLower(fingerprint) {
				return ErrCertificateChanged
			}
			return nil
		},
	}
}

// Fingerprint is the lowercase hex SHA-256 of a DER certificate, the form the
// operator compares against Winbox.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// FetchFingerprint reads the certificate a router presents, for an operator to
// confirm before it is pinned. Nothing is sent over the connection.
func FetchFingerprint(host string, port int) (string, error) {
	dialer := &net.Dialer{Timeout: requestTimeout}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := tls.DialWithDialer(dialer, "tcp", address,
		&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // reading the certificate is the point
	if err != nil {
		return "", fmt.Errorf("TLS handshake with %s: %w", address, err)
	}
	defer func() { _ = conn.Close() }()

	certificates := conn.ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return "", fmt.Errorf("%s presented no certificate", address)
	}
	return Fingerprint(certificates[0].Raw), nil
}

func (c *Client) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrCertificateChanged) {
			return ErrCertificateChanged
		}
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// SystemResource reads uptime, version, CPU load, memory and board name.
func (c *Client) SystemResource() (SystemResource, error) {
	var out SystemResource
	return out, c.get("/system/resource", &out)
}

// Clock reads the router's clock settings.
func (c *Client) Clock() (Clock, error) {
	var out Clock
	return out, c.get("/system/clock", &out)
}

// Interfaces lists every interface, dynamic ones included.
func (c *Client) Interfaces() ([]Interface, error) {
	var out []Interface
	return out, c.get("/interface", &out)
}

// VLANs lists VLAN interfaces with their ID and parent.
func (c *Client) VLANs() ([]VLAN, error) {
	var out []VLAN
	return out, c.get("/interface/vlan", &out)
}

// Addresses lists IPv4 addresses and the interface each sits on.
func (c *Client) Addresses() ([]Address, error) {
	var out []Address
	return out, c.get("/ip/address", &out)
}

// BGPConnections lists configured BGP peers.
func (c *Client) BGPConnections() ([]BGPConnection, error) {
	var out []BGPConnection
	return out, c.get("/routing/bgp/connection", &out)
}

// BGPSessions lists running BGP sessions.
func (c *Client) BGPSessions() ([]BGPSession, error) {
	var out []BGPSession
	return out, c.get("/routing/bgp/session", &out)
}
```

- [ ] **Step 5: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/mikrotik/ -v -race`
Expected: PASS semua. Kalau `TestClientRefusesARouterWhoseCertificateChanged` gagal karena `errors.Is` tidak menemukan `ErrCertificateChanged`, cetak `err` dan periksa pembungkusnya di `crypto/tls` versi Go ini. Perbaiki deteksi di `get`, jangan longgarkan test-nya.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/mikrotik/
git commit -m "feat(mikrotik): REST client pinned to the router's certificate"
```

---

### Task 4: Walk counter IF-MIB

**Files:**
- Create: `backend/internal/connectivity/snmp_ifxtable.go`

**Interfaces:**
- Consumes: `newSNMPClientWithContext`, `bulkWalk`, `toInt64` (sudah ada di paket `connectivity`).
- Produces: `connectivity.IfCounters{IfIndex int; Name string; InOctets, OutOctets, SpeedMbps uint64; OperUp bool}` dan `connectivity.WalkIfCounters(ctx context.Context, host string, port int, community string) ([]IfCounters, error)`.

Kode ini membuat `&gosnmp.GoSNMP{}` lewat helper dan berbicara langsung ke jaringan, jadi termasuk pengecualian "network-bound SNMP code" di CLAUDE.md: tidak ada unit test. Kebenarannya dibuktikan lewat `go build`, `go vet`, dan langkah verifikasi langsung ke router di Task 14.

- [ ] **Step 1: Implementasi**

`backend/internal/connectivity/snmp_ifxtable.go`:

```go
package connectivity

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// IF-MIB columns read for every interface. ifXTable's 64-bit counters, not
// ifTable's 32-bit ones: a 32-bit octet counter wraps in under four seconds at
// 10 Gbps, so a one-minute poll could not tell how many times it had.
const (
	oidIfName        = "1.3.6.1.2.1.31.1.1.1.1"
	oidIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"
	oidIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10"
	oidIfHighSpeed   = "1.3.6.1.2.1.31.1.1.1.15"
	oidIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"
	ifOperStatusUp   = 1
)

// ifCounterRequestTimeout is the per-request SNMP timeout for a router. A
// router answers in milliseconds; the walk is bounded as a whole by the
// caller's context.
const ifCounterRequestTimeout = 2 * time.Second

// IfCounters is one interface's row of IF-MIB.
type IfCounters struct {
	IfIndex   int
	Name      string
	InOctets  uint64
	OutOctets uint64
	SpeedMbps uint64
	OperUp    bool
}

type ifColumn struct {
	oid string
	set func(row *IfCounters, pdu gosnmp.SnmpPDU)
}

var ifColumns = []ifColumn{
	{oidIfName, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.Name = pduText(pdu) }},
	{oidIfHCInOctets, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.InOctets = pduUint(pdu) }},
	{oidIfHCOutOctets, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.OutOctets = pduUint(pdu) }},
	{oidIfHighSpeed, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.SpeedMbps = pduUint(pdu) }},
	{oidIfOperStatus, func(row *IfCounters, pdu gosnmp.SnmpPDU) {
		status, _ := toInt64(pdu.Value)
		row.OperUp = status == ifOperStatusUp
	}},
}

// WalkIfCounters reads name, 64-bit octet counters, speed and operational
// status of every interface, ordered by ifIndex.
func WalkIfCounters(ctx context.Context, host string, port int, community string) ([]IfCounters, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout

	rows := map[int]*IfCounters{}
	for _, column := range ifColumns {
		err := bulkWalk(client, column.oid, func(pdu gosnmp.SnmpPDU) error {
			index, err := lastSubidentifier(pdu.Name)
			if err != nil {
				return err
			}
			row, ok := rows[index]
			if !ok {
				row = &IfCounters{IfIndex: index}
				rows[index] = row
			}
			column.set(row, pdu)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", column.oid, err)
		}
	}

	out := make([]IfCounters, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IfIndex < out[j].IfIndex })
	return out, nil
}

func lastSubidentifier(oid string) (int, error) {
	index, err := strconv.Atoi(oid[strings.LastIndex(oid, ".")+1:])
	if err != nil {
		return 0, fmt.Errorf("OID %s has no numeric index", oid)
	}
	return index, nil
}

func pduText(pdu gosnmp.SnmpPDU) string {
	if raw, ok := pdu.Value.([]byte); ok {
		return string(raw)
	}
	return fmt.Sprint(pdu.Value)
}

// pduUint reads a Counter64 without going through int64, which would turn a
// counter past 2^63 negative.
func pduUint(pdu gosnmp.SnmpPDU) uint64 {
	value := gosnmp.ToBigInt(pdu.Value)
	if value.Sign() < 0 || value.Cmp(new(big.Int).SetUint64(^uint64(0))) > 0 {
		return 0
	}
	return value.Uint64()
}
```

- [ ] **Step 2: Build dan vet**

Run: `cd backend && go build ./... && go vet ./internal/connectivity/ && gofmt -s -l internal/connectivity/`
Expected: tanpa output dari `gofmt`, build dan vet bersih.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/connectivity/snmp_ifxtable.go
git commit -m "feat(connectivity): walk IF-MIB 64-bit interface counters"
```

---

### Task 5: Logika BGP dan rate (murni)

**Files:**
- Create: `backend/internal/services/router_rates.go`, `router_rates_test.go`
- Create: `backend/internal/services/router_bgp.go`, `router_bgp_test.go`

**Interfaces:**
- Consumes: `mikrotik.ParseBool/ParseInt/ParseDuration/ParseTime`, struct `mikrotik.BGPConnection`, `mikrotik.BGPSession` (Task 2–3); model dari Task 1.
- Produces:
  - `services.InterfaceRate(prevOctets, curOctets uint64, prevAt, curAt time.Time) (float64, bool)`
  - `services.BGPPeerState{Name, RemoteAddress, LocalAddress string; RemoteAS, UptimeSeconds, PrefixCount int64; State models.BGPState; LastStartedAt, LastStoppedAt *time.Time}`
  - `services.MergeBGP(conns []mikrotik.BGPConnection, sessions []mikrotik.BGPSession, loc *time.Location) []BGPPeerState`
  - `services.MatchInterface(remote string, ifaces []models.RouterInterface) *uuid.UUID`
  - `services.DiffBGP(routerID uuid.UUID, stored []models.RouterBGPSession, current []BGPPeerState, now time.Time) []models.RouterBGPEvent`

- [ ] **Step 1: Tulis test rate yang gagal**

`backend/internal/services/router_rates_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInterfaceRate(t *testing.T) {
	start := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

	bps, ok := InterfaceRate(1_000, 750_001_000, start, start.Add(time.Minute))
	assert.True(t, ok)
	assert.InDelta(t, 100_000_000, bps, 0.001)

	// A counter that went backwards means the router rebooted or the counter
	// was reset. The difference is meaningless, and drawing it would put a
	// spike of exabits on the graph.
	_, ok = InterfaceRate(750_001_000, 5_000, start, start.Add(time.Minute))
	assert.False(t, ok)

	_, ok = InterfaceRate(1_000, 2_000, start, start)
	assert.False(t, ok, "no elapsed time gives no rate")

	// Past 2^63 the counter is still read correctly.
	high := uint64(1) << 63
	bps, ok = InterfaceRate(high, high+125, start, start.Add(time.Second))
	assert.True(t, ok)
	assert.InDelta(t, 1000, bps, 0.001)
}
```

- [ ] **Step 2: Implementasi rate**

`backend/internal/services/router_rates.go`:

```go
package services

import "time"

// InterfaceRate returns bits per second between two samples of an octet
// counter, and false when the pair cannot give an honest rate: no time passed,
// or the counter went backwards because the router rebooted.
func InterfaceRate(prevOctets, curOctets uint64, prevAt, curAt time.Time) (float64, bool) {
	elapsed := curAt.Sub(prevAt).Seconds()
	if elapsed <= 0 || curOctets < prevOctets {
		return 0, false
	}
	return float64(curOctets-prevOctets) * 8 / elapsed, true
}
```

Run: `cd backend && go test ./internal/services/ -run TestInterfaceRate -v`
Expected: PASS.

- [ ] **Step 3: Tulis test BGP yang gagal**

`backend/internal/services/router_bgp_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

var wib = time.FixedZone("+07:00", 7*3600)

// Shaped after the border router's real configuration, in documentation ranges:
// a transit, an IX, and four route-server connections sharing one VLAN, two of
// which never came up.
func borderConnections() []mikrotik.BGPConnection {
	return []mikrotik.BGPConnection{
		{Name: "peer-transit", RemoteAddress: "198.51.100.0/32", RemoteAS: "64500"},
		{Name: "peer-ix", RemoteAddress: "198.51.100.2/32", RemoteAS: "64501"},
		{Name: "peer-rs1", RemoteAddress: "198.18.0.16/32", RemoteAS: "64502"},
		{Name: "peer-rs2", RemoteAddress: "198.18.1.17/32", RemoteAS: "64502"},
		{Name: "peer-rs1-NEW", RemoteAddress: "198.18.0.166/32", RemoteAS: "64502"},
		{Name: "peer-old", RemoteAddress: "192.0.2.9/32", RemoteAS: "64503", Disabled: "true"},
	}
}

func borderSessions() []mikrotik.BGPSession {
	return []mikrotik.BGPSession{
		{Name: "peer-transit-1", RemoteAddress: "198.51.100.0", RemoteAS: "64500", LocalAddress: "198.51.100.1",
			Established: "true", Uptime: "15h53m9s990ms", PrefixCount: "1093192",
			LastStarted: "2026-10-11 16:06:43", LastStopped: "2026-10-11 16:06:39"},
		{Name: "peer-ix-1", RemoteAddress: "198.51.100.2", RemoteAS: "64501", LocalAddress: "198.51.100.3",
			Established: "true", Uptime: "15h53m11s540ms", PrefixCount: "43523",
			LastStarted: "2026-10-11 16:06:42", LastStopped: "2026-10-11 16:06:39"},
		{Name: "peer-rs1-1", RemoteAddress: "198.18.0.16", RemoteAS: "64502", LocalAddress: "198.18.0.140",
			Established: "true", Uptime: "1d14h52m22s720ms", PrefixCount: "9175", LastStarted: "2026-10-10 17:07:30"},
		{Name: "peer-rs2-1", RemoteAddress: "198.18.1.17", RemoteAS: "64502", LocalAddress: "198.18.0.140",
			Established: "true", Uptime: "1d14h52m22s710ms", PrefixCount: "8987", LastStarted: "2026-10-10 17:07:30"},
		{Name: "peer-rs1-NEW-1", RemoteAddress: "198.18.0.166", RemoteAS: "0", LocalAddress: "198.18.0.140", PrefixCount: "0"},
	}
}

func peerNamed(t *testing.T, peers []BGPPeerState, name string) BGPPeerState {
	t.Helper()
	for _, p := range peers {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no peer %q", name)
	return BGPPeerState{}
}

func TestMergeBGPReportsEveryConfiguredPeerOnce(t *testing.T) {
	peers := MergeBGP(borderConnections(), borderSessions(), wib)

	require.Len(t, peers, 5, "the disabled connection is configuration, not an outage")

	transit := peerNamed(t, peers, "peer-transit")
	assert.Equal(t, models.BGPStateEstablished, transit.State)
	assert.Equal(t, "198.51.100.0", transit.RemoteAddress, "the /32 is stripped")
	assert.EqualValues(t, 64500, transit.RemoteAS)
	assert.EqualValues(t, 1093192, transit.PrefixCount)
	assert.EqualValues(t, 15*3600+53*60+9, transit.UptimeSeconds)
	require.NotNil(t, transit.LastStoppedAt)
	assert.Equal(t, time.Date(2026, 10, 11, 9, 6, 39, 0, time.UTC), transit.LastStoppedAt.UTC())
}

// "peer-rs1" is a prefix of "peer-rs1-NEW-1". Matching sessions by prefix would
// hand the never-established NEW session to the healthy RS1, or the reverse.
func TestMergeBGPDoesNotConfuseAConnectionWithOneItPrefixes(t *testing.T) {
	peers := MergeBGP(borderConnections(), borderSessions(), wib)

	assert.Equal(t, models.BGPStateEstablished, peerNamed(t, peers, "peer-rs1").State)
	newPeer := peerNamed(t, peers, "peer-rs1-NEW")
	assert.Equal(t, models.BGPStateDown, newPeer.State)
	assert.Zero(t, newPeer.PrefixCount)
}

func TestMergeBGPMarksAConnectionWithNoSessionDown(t *testing.T) {
	conns := []mikrotik.BGPConnection{{Name: "peer-lonely", RemoteAddress: "192.0.2.5", RemoteAS: "64504"}}

	peers := MergeBGP(conns, nil, wib)

	require.Len(t, peers, 1)
	assert.Equal(t, models.BGPStateDown, peers[0].State)
}

func TestMergeBGPFallsBackToTheRemoteAddressForUnfamiliarSessionNames(t *testing.T) {
	conns := []mikrotik.BGPConnection{{Name: "upstream", RemoteAddress: "192.0.2.5/32", RemoteAS: "64504"}}
	sessions := []mikrotik.BGPSession{{Name: "upstream-session", RemoteAddress: "192.0.2.5", Established: "true"}}

	peers := MergeBGP(conns, sessions, wib)

	assert.Equal(t, models.BGPStateEstablished, peers[0].State)
}

func TestMatchInterfacePicksTheSubnetThePeerLivesIn(t *testing.T) {
	transitVLAN := models.RouterInterface{ID: uuid.New(), Name: "vlan-transit", Addresses: "198.51.100.1/31"}
	ixVLAN := models.RouterInterface{ID: uuid.New(), Name: "vlan-ix", Addresses: "198.51.100.3/31"}
	rsVLAN := models.RouterInterface{ID: uuid.New(), Name: "vlan-rs", Addresses: "198.18.0.140/23"}
	mgmt := models.RouterInterface{ID: uuid.New(), Name: "bridge-mgmt", Addresses: "192.0.2.1/28,192.0.2.33/29"}
	ifaces := []models.RouterInterface{transitVLAN, ixVLAN, rsVLAN, mgmt}

	assert.Equal(t, transitVLAN.ID, *MatchInterface("198.51.100.0", ifaces))
	assert.Equal(t, ixVLAN.ID, *MatchInterface("198.51.100.2", ifaces))
	// The second route server sits in the upper half of the /23.
	assert.Equal(t, rsVLAN.ID, *MatchInterface("198.18.1.17", ifaces))
	assert.Equal(t, mgmt.ID, *MatchInterface("192.0.2.35", ifaces), "the second address on an interface counts")
	assert.Nil(t, MatchInterface("192.0.2.200", ifaces), "a multihop peer is not guessed")
	assert.Nil(t, MatchInterface("not-an-ip", ifaces))
}

func TestMatchInterfacePrefersTheMostSpecificSubnet(t *testing.T) {
	wide := models.RouterInterface{ID: uuid.New(), Addresses: "192.0.2.0/24"}
	narrow := models.RouterInterface{ID: uuid.New(), Addresses: "192.0.2.8/30"}

	assert.Equal(t, narrow.ID, *MatchInterface("192.0.2.9", []models.RouterInterface{wide, narrow}))
}

func storedSession(name string, state models.BGPState, started *time.Time) models.RouterBGPSession {
	return models.RouterBGPSession{Name: name, State: state, LastStartedAt: started}
}

func at(s string) *time.Time {
	t, _ := mikrotik.ParseTime(s, wib)
	return &t
}

func TestDiffBGPRecordsAStateChangeAtTheRoutersTime(t *testing.T) {
	routerID := uuid.New()
	now := time.Date(2026, 10, 11, 9, 7, 30, 0, time.UTC)
	stored := []models.RouterBGPSession{storedSession("peer-transit", models.BGPStateEstablished, at("2026-10-10 10:00:00"))}
	current := []BGPPeerState{{Name: "peer-transit", State: models.BGPStateDown, LastStoppedAt: at("2026-10-11 16:06:39")}}

	events := DiffBGP(routerID, stored, current, now)

	require.Len(t, events, 1)
	assert.Equal(t, models.BGPStateEstablished, events[0].FromState)
	assert.Equal(t, models.BGPStateDown, events[0].ToState)
	assert.Equal(t, *at("2026-10-11 16:06:39"), events[0].OccurredAt)
	assert.Equal(t, routerID, events[0].RouterID)
}

// The outage of 2026-10-11 lasted four seconds. A poll a minute apart sees
// Established both times; only last-started moving tells that the session
// dropped and came back.
func TestDiffBGPCatchesADropShorterThanThePollInterval(t *testing.T) {
	stored := []models.RouterBGPSession{storedSession("peer-transit", models.BGPStateEstablished, at("2026-10-10 10:00:00"))}
	current := []BGPPeerState{{
		Name: "peer-transit", State: models.BGPStateEstablished,
		LastStoppedAt: at("2026-10-11 16:06:39"), LastStartedAt: at("2026-10-11 16:06:43"),
	}}

	events := DiffBGP(uuid.New(), stored, current, time.Now())

	require.Len(t, events, 2)
	assert.Equal(t, models.BGPStateDown, events[0].ToState)
	assert.Equal(t, *at("2026-10-11 16:06:39"), events[0].OccurredAt)
	assert.Equal(t, models.BGPStateEstablished, events[1].ToState)
	assert.Equal(t, *at("2026-10-11 16:06:43"), events[1].OccurredAt)
}

func TestDiffBGPRecordsThreeSessionsDroppingInTheSameSecond(t *testing.T) {
	started := at("2026-10-10 10:00:00")
	stored := []models.RouterBGPSession{
		storedSession("a", models.BGPStateEstablished, started),
		storedSession("b", models.BGPStateEstablished, started),
		storedSession("c", models.BGPStateEstablished, started),
	}
	stopped := at("2026-10-11 16:06:39")
	current := []BGPPeerState{
		{Name: "a", State: models.BGPStateDown, LastStoppedAt: stopped},
		{Name: "b", State: models.BGPStateDown, LastStoppedAt: stopped},
		{Name: "c", State: models.BGPStateDown, LastStoppedAt: stopped},
	}

	events := DiffBGP(uuid.New(), stored, current, time.Now())

	require.Len(t, events, 3)
	for _, e := range events {
		assert.Equal(t, *stopped, e.OccurredAt)
	}
}

func TestDiffBGPIsQuietOnFirstSightAndWhenNothingChanged(t *testing.T) {
	started := at("2026-10-10 10:00:00")
	stored := []models.RouterBGPSession{storedSession("steady", models.BGPStateEstablished, started)}
	current := []BGPPeerState{
		{Name: "steady", State: models.BGPStateEstablished, LastStartedAt: started},
		{Name: "brand-new", State: models.BGPStateDown},
	}

	assert.Empty(t, DiffBGP(uuid.New(), stored, current, time.Now()))
}

func TestDiffBGPFallsBackToNowWhenTheRouterGivesNoTime(t *testing.T) {
	now := time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC)
	stored := []models.RouterBGPSession{storedSession("p", models.BGPStateEstablished, nil)}
	current := []BGPPeerState{{Name: "p", State: models.BGPStateDown}}

	events := DiffBGP(uuid.New(), stored, current, now)

	require.Len(t, events, 1)
	assert.Equal(t, now, events[0].OccurredAt)
}
```

- [ ] **Step 4: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestMergeBGP|TestMatchInterface|TestDiffBGP' -v`
Expected: FAIL, `undefined: MergeBGP`.

- [ ] **Step 5: Implementasi**

`backend/internal/services/router_bgp.go`:

```go
package services

import (
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// BGPPeerState is one configured BGP connection merged with its running
// session.
type BGPPeerState struct {
	Name          string
	RemoteAddress string
	RemoteAS      int64
	LocalAddress  string
	State         models.BGPState
	UptimeSeconds int64
	PrefixCount   int64
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
}

// MergeBGP reports every enabled connection once.
//
// Connections, not sessions, are the list: a peer that never comes up has a
// connection and only a useless session (remote.as 0), and reporting from
// sessions would leave it out instead of showing it down.
func MergeBGP(conns []mikrotik.BGPConnection, sessions []mikrotik.BGPSession, loc *time.Location) []BGPPeerState {
	peers := make([]BGPPeerState, 0, len(conns))
	for _, conn := range conns {
		if mikrotik.ParseBool(conn.Disabled) {
			continue
		}
		peer := BGPPeerState{Name: conn.Name, RemoteAddress: hostAddress(conn.RemoteAddress), State: models.BGPStateDown}
		peer.RemoteAS, _ = mikrotik.ParseInt(conn.RemoteAS)
		if session := sessionFor(conn, sessions); session != nil {
			applySession(&peer, *session, loc)
		}
		peers = append(peers, peer)
	}
	return peers
}

// sessionFor finds a connection's running session. A connection can have more
// than one (listening and connecting); the established one is what counts.
func sessionFor(conn mikrotik.BGPConnection, sessions []mikrotik.BGPSession) *mikrotik.BGPSession {
	var found *mikrotik.BGPSession
	for i := range sessions {
		if !belongsTo(sessions[i], conn) {
			continue
		}
		if mikrotik.ParseBool(sessions[i].Established) {
			return &sessions[i]
		}
		if found == nil {
			found = &sessions[i]
		}
	}
	return found
}

// belongsTo matches a session to its connection by name, RouterOS v7 naming a
// session "<connection>-<n>". The remote address is the fallback for a name in
// any other shape.
func belongsTo(session mikrotik.BGPSession, conn mikrotik.BGPConnection) bool {
	if base, ok := connectionName(session.Name); ok {
		return base == conn.Name
	}
	return hostAddress(session.RemoteAddress) == hostAddress(conn.RemoteAddress)
}

// connectionName strips the "-<n>" suffix, so "peer-rs1-NEW-1" gives
// "peer-rs1-NEW" and never matches "peer-rs1".
func connectionName(session string) (string, bool) {
	i := strings.LastIndex(session, "-")
	if i <= 0 || i == len(session)-1 {
		return "", false
	}
	if _, err := strconv.Atoi(session[i+1:]); err != nil {
		return "", false
	}
	return session[:i], true
}

func hostAddress(address string) string {
	host, _, _ := strings.Cut(address, "/")
	return host
}

func applySession(peer *BGPPeerState, session mikrotik.BGPSession, loc *time.Location) {
	peer.LocalAddress = hostAddress(session.LocalAddress)
	peer.LastStartedAt = routerTime(session.LastStarted, loc)
	peer.LastStoppedAt = routerTime(session.LastStopped, loc)
	if !mikrotik.ParseBool(session.Established) {
		return
	}
	peer.State = models.BGPStateEstablished
	if uptime, err := mikrotik.ParseDuration(session.Uptime); err == nil {
		peer.UptimeSeconds = int64(uptime.Seconds())
	}
	peer.PrefixCount, _ = mikrotik.ParseInt(session.PrefixCount)
}

func routerTime(s string, loc *time.Location) *time.Time {
	t, err := mikrotik.ParseTime(s, loc)
	if err != nil {
		return nil
	}
	return &t
}

// MatchInterface returns the interface whose subnet holds the peer's address,
// the most specific one when several do. Nil when none does: a multihop peer's
// interface is not guessed.
func MatchInterface(remote string, ifaces []models.RouterInterface) *uuid.UUID {
	addr, err := netip.ParseAddr(remote)
	if err != nil {
		return nil
	}
	var best *uuid.UUID
	bestBits := -1
	for i := range ifaces {
		bits := longestContaining(ifaces[i].Addresses, addr)
		if bits > bestBits {
			id := ifaces[i].ID
			best, bestBits = &id, bits
		}
	}
	return best
}

// longestContaining returns the prefix length of the most specific CIDR in a
// comma-separated list that holds addr, or -1 when none does.
func longestContaining(cidrs string, addr netip.Addr) int {
	best := -1
	for _, cidr := range strings.Split(cidrs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err == nil && prefix.Masked().Contains(addr) && prefix.Bits() > best {
			best = prefix.Bits()
		}
	}
	return best
}

// DiffBGP turns the difference between what was stored and what the router
// reports now into events. A session seen for the first time produces none:
// there is no earlier state for it to have changed from.
func DiffBGP(routerID uuid.UUID, stored []models.RouterBGPSession, current []BGPPeerState, now time.Time) []models.RouterBGPEvent {
	previous := make(map[string]models.RouterBGPSession, len(stored))
	for _, s := range stored {
		previous[s.Name] = s
	}

	var events []models.RouterBGPEvent
	for _, peer := range current {
		old, seen := previous[peer.Name]
		switch {
		case !seen:
		case old.State != peer.State:
			events = append(events, bgpEvent(routerID, peer.Name, old.State, peer.State, transitionTime(peer, now)))
		case peer.State == models.BGPStateEstablished && restarted(old, peer):
			events = append(events, restartEvents(routerID, peer)...)
		}
	}
	return events
}

func restarted(old models.RouterBGPSession, peer BGPPeerState) bool {
	return old.LastStartedAt != nil && peer.LastStartedAt != nil && !old.LastStartedAt.Equal(*peer.LastStartedAt)
}

// restartEvents records a session that went down and came back between two
// polls. Only last-started moving shows it, and an outage of seconds is still
// an outage.
func restartEvents(routerID uuid.UUID, peer BGPPeerState) []models.RouterBGPEvent {
	downAt := *peer.LastStartedAt
	if peer.LastStoppedAt != nil {
		downAt = *peer.LastStoppedAt
	}
	return []models.RouterBGPEvent{
		bgpEvent(routerID, peer.Name, models.BGPStateEstablished, models.BGPStateDown, downAt),
		bgpEvent(routerID, peer.Name, models.BGPStateDown, models.BGPStateEstablished, *peer.LastStartedAt),
	}
}

// transitionTime is when the router says the change happened, so an event
// carries the moment of the outage rather than the moment it was polled.
func transitionTime(peer BGPPeerState, now time.Time) time.Time {
	when := peer.LastStoppedAt
	if peer.State == models.BGPStateEstablished {
		when = peer.LastStartedAt
	}
	if when == nil {
		return now
	}
	return *when
}

func bgpEvent(routerID uuid.UUID, name string, from, to models.BGPState, at time.Time) models.RouterBGPEvent {
	return models.RouterBGPEvent{RouterID: routerID, SessionName: name, FromState: from, ToState: to, OccurredAt: at}
}
```

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestInterfaceRate|TestMergeBGP|TestMatchInterface|TestDiffBGP' -v`
Expected: PASS semua.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/router_rates.go backend/internal/services/router_rates_test.go \
  backend/internal/services/router_bgp.go backend/internal/services/router_bgp_test.go
git commit -m "feat(mikrotik): merge BGP sessions, match peers to interfaces, diff state"
```

---

### Task 6: RouterService — CRUD, rahasia, uji koneksi, terima sertifikat

**Files:**
- Create: `backend/internal/services/router_service.go`, `router_service_test.go`
- Create: `backend/internal/services/router_connection.go`, `router_connection_test.go`

**Interfaces:**
- Consumes: `utils.Encrypt/Decrypt`, `connectivity.SNMPTest`, `mikrotik.FetchFingerprint`, `mikrotik.NewClient`, `services.ErrValidation` (sudah ada di `wireguard_validate.go`), model Task 1.
- Produces:
  - `services.NewRouterService(db *gorm.DB, encryptionKey string) *RouterService`
  - `services.RouterInput{Name, Host string; SiteID *uuid.UUID; SNMPCommunity string; SNMPPort int; APIUsername, APIPassword string; APIPort int; TLSFingerprint string}`
  - `services.RouterUpdate{Name, Host, SNMPCommunity, APIUsername, APIPassword *string; SNMPPort, APIPort *int; SiteID *uuid.UUID; ClearSite bool; Enabled *bool}`
  - `(*RouterService) Create(RouterInput) (*models.MikrotikRouter, error)`, `Update(uuid.UUID, RouterUpdate) (*models.MikrotikRouter, error)`, `Delete(uuid.UUID) error`, `Get(uuid.UUID) (*models.MikrotikRouter, error)`, `List() ([]models.MikrotikRouter, error)`
  - `(*RouterService) Secrets(*models.MikrotikRouter) (community, password string, err error)`
  - `(*RouterService) TestConnection(ConnectionTestInput) ConnectionTestResult`
  - `(*RouterService) TrustCertificate(id uuid.UUID, fingerprint string) (*models.MikrotikRouter, error)`
  - `services.ConnectionTestInput{Host string; SNMPPort int; SNMPCommunity string; APIPort int; APIUsername, APIPassword string}`
  - `services.ConnectionTestResult{SNMPError, APIError, Fingerprint, BoardName, Version string}` (JSON tag snake_case, `omitempty`)
  - `services.ErrFingerprintMismatch`
  - Konstanta `services.DefaultRouterSNMPPort = 161`, `services.DefaultRouterAPIPort = 8443`

- [ ] **Step 1: Tulis test CRUD yang gagal**

`backend/internal/services/router_service_test.go`:

```go
package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

const routerTestKey = "12345678901234567890123456789012"

var testFingerprint = strings.Repeat("ab", 32)

func newRouterServiceForTest(t *testing.T) (*RouterService, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	return NewRouterService(db, routerTestKey), db
}

func validRouterInput() RouterInput {
	return RouterInput{
		Name: "Border APJII", Host: "192.0.2.1", SNMPCommunity: "community-secret",
		APIUsername: "tikman", APIPassword: "password-secret", TLSFingerprint: testFingerprint,
	}
}

func TestCreateRouterEncryptsSecretsAndSchedulesEveryTier(t *testing.T) {
	service, db := newRouterServiceForTest(t)

	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	var stored models.MikrotikRouter
	require.NoError(t, db.First(&stored, "id = ?", router.ID).Error)
	assert.NotEqual(t, "community-secret", stored.SNMPCommunity)
	assert.NotEqual(t, "password-secret", stored.APIPassword)
	assert.Equal(t, DefaultRouterSNMPPort, stored.SNMPPort)
	assert.Equal(t, DefaultRouterAPIPort, stored.APIPort)
	assert.True(t, stored.Enabled)
	assert.Equal(t, models.RouterStatusUnknown, stored.Status)

	community, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "community-secret", community)
	assert.Equal(t, "password-secret", password)

	var kinds []string
	require.NoError(t, db.Model(&models.RouterPollJob{}).Where("router_id = ?", router.ID).Pluck("kind", &kinds).Error)
	assert.ElementsMatch(t, []string{"interfaces", "bgp", "discovery"}, kinds)
}

func TestCreateRouterRejectsIncompleteInput(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	cases := map[string]func(*RouterInput){
		"no name":           func(in *RouterInput) { in.Name = "  " },
		"no host":           func(in *RouterInput) { in.Host = "" },
		"no community":      func(in *RouterInput) { in.SNMPCommunity = "" },
		"no password":       func(in *RouterInput) { in.APIPassword = "" },
		"short fingerprint": func(in *RouterInput) { in.TLSFingerprint = "abcd" },
		"not hex":           func(in *RouterInput) { in.TLSFingerprint = strings.Repeat("zz", 32) },
	}
	for name, mutate := range cases {
		in := validRouterInput()
		mutate(&in)
		_, err := service.Create(in)
		assert.True(t, errors.Is(err, ErrValidation), "%s: got %v", name, err)
	}
}

func TestUpdateRouterKeepsSecretsLeftBlank(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	name, blank, port := "Border baru", "", 18443
	updated, err := service.Update(router.ID, RouterUpdate{Name: &name, APIPassword: &blank, APIPort: &port})
	require.NoError(t, err)
	assert.Equal(t, "Border baru", updated.Name)
	assert.Equal(t, 18443, updated.APIPort)

	var stored models.MikrotikRouter
	require.NoError(t, db.First(&stored, "id = ?", router.ID).Error)
	_, password, err := service.Secrets(&stored)
	require.NoError(t, err)
	assert.Equal(t, "password-secret", password)
}

func TestUpdateRouterCanDisableIt(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	off := false
	updated, err := service.Update(router.ID, RouterUpdate{Enabled: &off})
	require.NoError(t, err)
	assert.False(t, updated.Enabled)
}

func TestUpdateRouterRejectsAnEmptyName(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	empty := " "
	_, err = service.Update(router.ID, RouterUpdate{Name: &empty})
	assert.True(t, errors.Is(err, ErrValidation))
}

func TestDeleteRouterReportsAnUnknownOne(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	err := service.Delete(uuid.New())
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestListRoutersIsOrderedByName(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	for _, name := range []string{"Zeta", "Alpha"} {
		in := validRouterInput()
		in.Name = name
		_, err := service.Create(in)
		require.NoError(t, err)
	}

	routers, err := service.List()
	require.NoError(t, err)
	require.Len(t, routers, 2)
	assert.Equal(t, "Alpha", routers[0].Name)
}
```

`setupTestDB(t)` adalah helper SQLite yang sudah ada di `user_service_test.go` (paket `services`, memakai `models.AutoMigrate`).

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'Router' -v`
Expected: FAIL, `undefined: NewRouterService`.

- [ ] **Step 3: Implementasi CRUD**

`backend/internal/services/router_service.go`:

```go
package services

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// Router ports used when none is given. REST sits on 8443 because RouterOS 7's
// reverse-proxy commonly holds 443.
const (
	DefaultRouterSNMPPort = 161
	DefaultRouterAPIPort  = 8443
)

// fingerprintLength is a hex SHA-256.
const fingerprintLength = 64

// RouterService stores MikroTik routers and their credentials.
type RouterService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewRouterService creates a router service.
func NewRouterService(db *gorm.DB, encryptionKey string) *RouterService {
	return &RouterService{db: db, encryptionKey: strings.TrimSpace(encryptionKey)}
}

// RouterInput is a new router. TLSFingerprint is the one the connection test
// returned and the operator confirmed.
type RouterInput struct {
	Name           string
	Host           string
	SiteID         *uuid.UUID
	SNMPCommunity  string
	SNMPPort       int
	APIUsername    string
	APIPassword    string
	APIPort        int
	TLSFingerprint string
}

// RouterUpdate changes some of a router's fields. A nil or empty secret keeps
// the stored one, since the form cannot show what it does not know.
type RouterUpdate struct {
	Name          *string
	Host          *string
	SNMPCommunity *string
	APIUsername   *string
	APIPassword   *string
	SNMPPort      *int
	APIPort       *int
	SiteID        *uuid.UUID
	ClearSite     bool
	Enabled       *bool
}

// Create stores a router and makes all its poll tiers due at once, so the
// first readings arrive within a minute rather than after the next sweep.
func (s *RouterService) Create(in RouterInput) (*models.MikrotikRouter, error) {
	if err := validateRouterInput(in); err != nil {
		return nil, err
	}
	community, err := utils.Encrypt(in.SNMPCommunity, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt community: %w", err)
	}
	password, err := utils.Encrypt(in.APIPassword, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	router := &models.MikrotikRouter{
		Name: strings.TrimSpace(in.Name), Host: strings.TrimSpace(in.Host), SiteID: in.SiteID,
		SNMPCommunity: community, SNMPPort: portOr(in.SNMPPort, DefaultRouterSNMPPort),
		APIUsername: in.APIUsername, APIPassword: password, APIPort: portOr(in.APIPort, DefaultRouterAPIPort),
		TLSFingerprint: strings.ToLower(in.TLSFingerprint), Enabled: true,
		Status: models.RouterStatusUnknown, APIStatus: models.RouterAPIStatusUnknown,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(router).Error; err != nil {
			return err
		}
		return createRouterJobs(tx, router.ID)
	})
	if err != nil {
		return nil, err
	}
	return router, nil
}

func createRouterJobs(tx *gorm.DB, routerID uuid.UUID) error {
	now := time.Now()
	for _, kind := range AllRouterPollKinds {
		if err := tx.Create(&models.RouterPollJob{RouterID: routerID, Kind: kind, DueAt: now}).Error; err != nil {
			return err
		}
	}
	return nil
}

func validateRouterInput(in RouterInput) error {
	switch {
	case strings.TrimSpace(in.Name) == "":
		return fmt.Errorf("%w: name is required", ErrValidation)
	case strings.TrimSpace(in.Host) == "":
		return fmt.Errorf("%w: host is required", ErrValidation)
	case in.SNMPCommunity == "":
		return fmt.Errorf("%w: SNMP community is required", ErrValidation)
	case in.APIUsername == "" || in.APIPassword == "":
		return fmt.Errorf("%w: API username and password are required", ErrValidation)
	case !validFingerprint(in.TLSFingerprint):
		return fmt.Errorf("%w: tls_fingerprint must be the SHA-256 the connection test returned", ErrValidation)
	}
	return nil
}

func validFingerprint(fingerprint string) bool {
	if len(fingerprint) != fingerprintLength {
		return false
	}
	_, err := hex.DecodeString(fingerprint)
	return err == nil
}

func portOr(port, fallback int) int {
	if port == 0 {
		return fallback
	}
	return port
}

// Update applies the fields that were given.
func (s *RouterService) Update(id uuid.UUID, upd RouterUpdate) (*models.MikrotikRouter, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	updates, err := s.updateColumns(upd)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.db.Model(&models.MikrotikRouter{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

func (s *RouterService) updateColumns(upd RouterUpdate) (map[string]interface{}, error) {
	updates := map[string]interface{}{}
	if upd.Name != nil {
		if strings.TrimSpace(*upd.Name) == "" {
			return nil, fmt.Errorf("%w: name is required", ErrValidation)
		}
		updates["name"] = strings.TrimSpace(*upd.Name)
	}
	if upd.Host != nil {
		if strings.TrimSpace(*upd.Host) == "" {
			return nil, fmt.Errorf("%w: host is required", ErrValidation)
		}
		updates["host"] = strings.TrimSpace(*upd.Host)
	}
	setIfGiven(updates, "api_username", upd.APIUsername)
	setIfGiven(updates, "snmp_port", upd.SNMPPort)
	setIfGiven(updates, "api_port", upd.APIPort)
	setIfGiven(updates, "enabled", upd.Enabled)
	setIfGiven(updates, "site_id", upd.SiteID)
	if upd.ClearSite {
		updates["site_id"] = nil
	}
	return updates, s.encryptGivenSecrets(updates, upd)
}

func setIfGiven[T any](updates map[string]interface{}, column string, value *T) {
	if value != nil {
		updates[column] = *value
	}
}

func (s *RouterService) encryptGivenSecrets(updates map[string]interface{}, upd RouterUpdate) error {
	for column, secret := range map[string]*string{"snmp_community": upd.SNMPCommunity, "api_password": upd.APIPassword} {
		if secret == nil || *secret == "" {
			continue
		}
		encrypted, err := utils.Encrypt(*secret, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", column, err)
		}
		updates[column] = encrypted
	}
	return nil
}

// Delete removes a router. Its interfaces, sessions, events and jobs go with
// it by foreign key; its metric samples age out by retention.
func (s *RouterService) Delete(id uuid.UUID) error {
	result := s.db.Delete(&models.MikrotikRouter{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Get returns one router.
func (s *RouterService) Get(id uuid.UUID) (*models.MikrotikRouter, error) {
	var router models.MikrotikRouter
	if err := s.db.First(&router, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &router, nil
}

// List returns every router by name.
func (s *RouterService) List() ([]models.MikrotikRouter, error) {
	var routers []models.MikrotikRouter
	return routers, s.db.Order("name").Find(&routers).Error
}
```

`AllRouterPollKinds` didefinisikan di Task 7. Supaya Task 6 bisa dikompilasi sendiri, tambahkan definisinya sekarang di `router_poll_jobs.go` (Task 7 akan melengkapi file itu):

```go
package services

import "github.com/tikman/olt-provisioning/internal/models"

// AllRouterPollKinds is every router tier, discovery first so a new router has
// interfaces before its first counter poll.
var AllRouterPollKinds = []models.RouterPollKind{
	models.RouterPollKindDiscovery,
	models.RouterPollKindInterfaces,
	models.RouterPollKindBGP,
}
```

- [ ] **Step 4: Jalankan test CRUD, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestCreateRouter|TestUpdateRouter|TestDeleteRouter|TestListRouters' -v`
Expected: PASS.

- [ ] **Step 5: Tulis test koneksi dan sertifikat yang gagal**

`backend/internal/services/router_connection_test.go`:

```go
package services

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestConnectionTestReturnsTheFingerprintAndBoard(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"board-name":"CCR2116-12G-4S+","version":"7.24.2 (stable)"}`))
	}))
	defer server.Close()
	host, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: host, SNMPPort: 1, SNMPCommunity: "x", APIPort: port, APIUsername: "u", APIPassword: "p",
	})

	assert.Empty(t, result.APIError)
	assert.Equal(t, mikrotik.Fingerprint(server.Certificate().Raw), result.Fingerprint)
	assert.Equal(t, "CCR2116-12G-4S+", result.BoardName)
	assert.Equal(t, "7.24.2 (stable)", result.Version)
	assert.NotEmpty(t, result.SNMPError, "nothing answers SNMP on port 1")
}

func TestConnectionTestReportsAnUnreachableAPI(t *testing.T) {
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{Host: "127.0.0.1", SNMPPort: 1, APIPort: 1})

	assert.NotEmpty(t, result.APIError)
	assert.Empty(t, result.Fingerprint)
}

func TestTrustCertificateAcceptsOnlyWhatTheRouterPresented(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	presented := "cd" + testFingerprint[2:]
	require.NoError(t, db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Updates(map[string]interface{}{
		"api_status": models.RouterAPIStatusCertificateChanged, "presented_fingerprint": presented,
	}).Error)

	_, err = service.TrustCertificate(router.ID, testFingerprint)
	assert.True(t, errors.Is(err, ErrFingerprintMismatch), "a fingerprint the router never showed is refused")

	trusted, err := service.TrustCertificate(router.ID, presented)
	require.NoError(t, err)
	assert.Equal(t, presented, trusted.TLSFingerprint)
	assert.Nil(t, trusted.PresentedFingerprint)
	assert.Equal(t, models.RouterAPIStatusUnknown, trusted.APIStatus)
}

func TestTrustCertificateRefusesWhenNothingChanged(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	_, err = service.TrustCertificate(router.ID, testFingerprint)
	assert.True(t, errors.Is(err, ErrFingerprintMismatch))
}
```

- [ ] **Step 6: Jalankan, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestConnectionTest|TestTrustCertificate' -v`
Expected: FAIL, `undefined: ConnectionTestInput`.

- [ ] **Step 7: Implementasi**

`backend/internal/services/router_connection.go`:

```go
package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

// routerSNMPTestTimeout matches the per-request timeout the poller uses, so a
// router that passes the test is not one the poller then times out on.
const routerSNMPTestTimeout = 2 * time.Second

// ErrFingerprintMismatch refuses a certificate the router was not seen
// presenting.
var ErrFingerprintMismatch = errors.New("fingerprint does not match the certificate the router presented")

// ConnectionTestInput is what the add-router form has before anything is
// stored.
type ConnectionTestInput struct {
	Host          string
	SNMPPort      int
	SNMPCommunity string
	APIPort       int
	APIUsername   string
	APIPassword   string
}

// ConnectionTestResult reports SNMP and REST separately: either can work while
// the other does not, and the operator fixes them in different places.
type ConnectionTestResult struct {
	SNMPError   string `json:"snmp_error,omitempty"`
	APIError    string `json:"api_error,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	BoardName   string `json:"board_name,omitempty"`
	Version     string `json:"version,omitempty"`
}

// TestConnection probes SNMP and REST, and returns the certificate fingerprint
// for the operator to confirm before it is pinned.
func (s *RouterService) TestConnection(in ConnectionTestInput) ConnectionTestResult {
	var result ConnectionTestResult
	snmpPort := portOr(in.SNMPPort, DefaultRouterSNMPPort)
	if err := connectivity.SNMPTest(in.Host, snmpPort, in.SNMPCommunity, routerSNMPTestTimeout); err != nil {
		result.SNMPError = err.Error()
	}

	apiPort := portOr(in.APIPort, DefaultRouterAPIPort)
	fingerprint, err := mikrotik.FetchFingerprint(in.Host, apiPort)
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	result.Fingerprint = fingerprint

	resource, err := mikrotik.NewClient(in.Host, apiPort, in.APIUsername, in.APIPassword, fingerprint).SystemResource()
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	result.BoardName, result.Version = resource.BoardName, resource.Version
	return result
}

// Secrets decrypts a router's SNMP community and API password.
func (s *RouterService) Secrets(router *models.MikrotikRouter) (string, string, error) {
	community, err := utils.Decrypt(router.SNMPCommunity, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt community: %w", err)
	}
	password, err := utils.Decrypt(router.APIPassword, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt password: %w", err)
	}
	return community, password, nil
}

// TrustCertificate pins the certificate the poller last saw the router
// presenting. The fingerprint must match it: an admin confirms what they were
// shown, not whatever the router happens to present by the time they click.
func (s *RouterService) TrustCertificate(id uuid.UUID, fingerprint string) (*models.MikrotikRouter, error) {
	router, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if router.PresentedFingerprint == nil || !strings.EqualFold(*router.PresentedFingerprint, fingerprint) {
		return nil, ErrFingerprintMismatch
	}
	err = s.db.Model(&models.MikrotikRouter{}).Where("id = ?", id).Updates(map[string]interface{}{
		"tls_fingerprint":       strings.ToLower(fingerprint),
		"presented_fingerprint": nil,
		"api_status":            models.RouterAPIStatusUnknown,
		"api_error":             nil,
	}).Error
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}
```

- [ ] **Step 8: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'Router|TestConnectionTest|TestTrustCertificate' -v -race`
Expected: PASS. `TestConnectionTestReturnsTheFingerprintAndBoard` menunggu SNMP ke port 1 paling lama 2 detik.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/services/router_service.go backend/internal/services/router_service_test.go \
  backend/internal/services/router_connection.go backend/internal/services/router_connection_test.go \
  backend/internal/services/router_poll_jobs.go
git commit -m "feat(mikrotik): store routers with encrypted credentials and a pinned certificate"
```

---

### Task 7: Antrian `router_poll_jobs`

**Files:**
- Modify: `backend/internal/services/router_poll_jobs.go` (dibuat sebagian di Task 6)
- Test: `backend/internal/services/router_poll_jobs_test.go`, `router_poll_jobs_postgres_test.go`

**Interfaces:**
- Consumes: `lockingSkipLocked()`, `maxBackoffShift` dari `poll_job_service.go` (paket yang sama).
- Produces: `services.NewRouterPollJobService(db) *RouterPollJobService`; `RouterIntervalFor(kind) time.Duration`; methods `Claim(workerID string) (*models.RouterPollJob, error)`, `Complete(job *models.RouterPollJob, took time.Duration) error`, `Fail(job *models.RouterPollJob, cause error) error`, `RunNow(routerID uuid.UUID, kinds ...models.RouterPollKind) error`.

Antrian ini meniru `PollJobService` karena tabel dan kuncinya berbeda (`router_id`, bukan `olt_id` dengan FK ke `olts`). Menggeneralisasi `PollJobService` berarti mengubah kode yang menjaga polling OLT produksi demi pemanggil kedua. Yang bisa dipakai ulang (`lockingSkipLocked`, `maxBackoffShift`) dipakai ulang.

- [ ] **Step 1: Tulis test yang gagal**

`backend/internal/services/router_poll_jobs_test.go`:

```go
package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func seedRouterJob(t *testing.T, service *RouterPollJobService, kind models.RouterPollKind, due time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, service.db.Create(&models.RouterPollJob{RouterID: id, Kind: kind, DueAt: due}).Error)
	return id
}

func TestRouterClaimTakesTheMostOverdueJob(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	oldest := seedRouterJob(t, service, models.RouterPollKindInterfaces, time.Now().Add(-time.Hour))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(time.Hour))

	job, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, oldest, job.RouterID)
	require.NotNil(t, job.LockedBy)
	assert.Equal(t, "worker-a", *job.LockedBy)
}

func TestRouterClaimSkipsAJobWhoseLeaseIsLive(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))

	first, err := service.Claim("worker-a")
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := service.Claim("worker-b")
	require.NoError(t, err)
	assert.Nil(t, second, "a held job is not handed out twice")
}

func TestRouterClaimRetakesAnAbandonedJob(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	id := seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Hour))
	stale := time.Now().Add(-routerPollLease - time.Minute)
	holder := "dead-worker"
	require.NoError(t, service.db.Model(&models.RouterPollJob{}).Where("router_id = ?", id).
		Updates(map[string]interface{}{"locked_by": holder, "locked_at": stale}).Error)

	job, err := service.Claim("worker-b")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, id, job.RouterID)
}

func TestRouterCompleteSchedulesTheNextRunAndReleases(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindDiscovery, time.Now().Add(-time.Minute))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)

	require.NoError(t, service.Complete(job, 300*time.Millisecond))

	var stored models.RouterPollJob
	require.NoError(t, service.db.First(&stored, "router_id = ?", job.RouterID).Error)
	assert.Nil(t, stored.LockedBy)
	assert.WithinDuration(t, time.Now().Add(RouterDiscoveryInterval), stored.DueAt, 5*time.Second)
	assert.Zero(t, stored.ConsecutiveFailures)
}

func TestRouterFailBacksOffAndCaps(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	job, err := service.Claim("worker-a")
	require.NoError(t, err)

	job.ConsecutiveFailures = 1
	require.NoError(t, service.Fail(job, errors.New("timeout")))

	var stored models.RouterPollJob
	require.NoError(t, service.db.First(&stored, "router_id = ?", job.RouterID).Error)
	assert.Equal(t, 2, stored.ConsecutiveFailures)
	assert.WithinDuration(t, time.Now().Add(2*RouterBGPInterval), stored.DueAt, 5*time.Second)
	require.NotNil(t, stored.LastError)
	assert.Equal(t, "timeout", *stored.LastError)

	assert.Equal(t, RouterBGPInterval*32, routerBackoff(models.RouterPollKindBGP, 40))
}

func TestRouterRunNowMakesTheGivenTiersDue(t *testing.T) {
	service := NewRouterPollJobService(setupTestDB(t))
	later := time.Now().Add(time.Hour)
	id := uuid.New()
	for _, kind := range AllRouterPollKinds {
		require.NoError(t, service.db.Create(&models.RouterPollJob{RouterID: id, Kind: kind, DueAt: later}).Error)
	}

	require.NoError(t, service.RunNow(id, models.RouterPollKindDiscovery))

	var discovery, bgp models.RouterPollJob
	require.NoError(t, service.db.First(&discovery, "router_id = ? AND kind = ?", id, models.RouterPollKindDiscovery).Error)
	require.NoError(t, service.db.First(&bgp, "router_id = ? AND kind = ?", id, models.RouterPollKindBGP).Error)
	assert.True(t, discovery.DueAt.Before(time.Now().Add(time.Second)))
	assert.True(t, bgp.DueAt.After(time.Now().Add(30*time.Minute)), "other tiers keep their schedule")
}
```

`backend/internal/services/router_poll_jobs_postgres_test.go`:

```go
package services

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Same reason as the OLT queue's Postgres test: only a real Postgres has FOR
// UPDATE SKIP LOCKED and concurrent writers.
func TestConcurrentRouterClaimsNeverHandOutTheSameJob(t *testing.T) {
	db := setupPollJobPostgres(t)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS router_poll_jobs").Error)
	require.NoError(t, db.AutoMigrate(&models.RouterPollJob{}))
	service := NewRouterPollJobService(db)

	const jobs = 8
	for i := 0; i < jobs; i++ {
		seedRouterJob(t, service, models.RouterPollKindBGP, time.Now().Add(-time.Minute))
	}

	var mu sync.Mutex
	seen := map[string]bool{}
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			job, err := service.Claim("worker")
			require.NoError(t, err)
			require.NotNil(t, job)
			mu.Lock()
			defer mu.Unlock()
			require.False(t, seen[job.RouterID.String()], "job handed out twice")
			seen[job.RouterID.String()] = true
		}(w)
	}
	wg.Wait()
	require.Len(t, seen, jobs)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestRouterClaim|TestRouterComplete|TestRouterFail|TestRouterRunNow' -v`
Expected: FAIL, `undefined: NewRouterPollJobService`.

- [ ] **Step 3: Implementasi**

Ganti isi `backend/internal/services/router_poll_jobs.go` dengan:

```go
package services

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// Router tier schedules. Unlike an OLT chassis, a router answers a full IF-MIB
// read and a BGP listing in well under a second, so these are fixed rather than
// stretched to the device's capacity.
const (
	RouterInterfacesInterval = time.Minute
	RouterBGPInterval        = time.Minute
	RouterDiscoveryInterval  = 6 * time.Hour
)

// Leases: an order of magnitude over what each tier needs, short enough that a
// killed worker's router is picked up again within minutes.
const (
	routerPollLease      = 2 * time.Minute
	routerDiscoveryLease = 5 * time.Minute
)

// AllRouterPollKinds is every router tier, discovery first so a new router has
// interfaces before its first counter poll.
var AllRouterPollKinds = []models.RouterPollKind{
	models.RouterPollKindDiscovery,
	models.RouterPollKindInterfaces,
	models.RouterPollKindBGP,
}

// RouterPollJobService hands out one router's work to one worker at a time.
type RouterPollJobService struct {
	db *gorm.DB
}

// NewRouterPollJobService creates a router poll job service.
func NewRouterPollJobService(db *gorm.DB) *RouterPollJobService {
	return &RouterPollJobService{db: db}
}

// RouterIntervalFor returns how long after a successful run the tier is due
// again.
func RouterIntervalFor(kind models.RouterPollKind) time.Duration {
	switch kind {
	case models.RouterPollKindInterfaces:
		return RouterInterfacesInterval
	case models.RouterPollKindBGP:
		return RouterBGPInterval
	default:
		return RouterDiscoveryInterval
	}
}

// Claim takes the most overdue job no other worker holds, or nil when nothing
// is due.
func (s *RouterPollJobService) Claim(workerID string) (*models.RouterPollJob, error) {
	var claimed models.RouterPollJob
	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		query := tx.Where("due_at <= ?", now).
			Where("locked_at IS NULL OR (kind = ? AND locked_at < ?) OR (kind <> ? AND locked_at < ?)",
				models.RouterPollKindDiscovery, now.Add(-routerDiscoveryLease),
				models.RouterPollKindDiscovery, now.Add(-routerPollLease)).
			Order("due_at")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(lockingSkipLocked())
		}

		var candidate models.RouterPollJob
		if err := query.First(&candidate).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.RouterPollJob{}).
			Where("router_id = ? AND kind = ?", candidate.RouterID, candidate.Kind).
			Updates(map[string]interface{}{"locked_by": workerID, "locked_at": now}).Error; err != nil {
			return err
		}
		candidate.LockedBy, candidate.LockedAt = &workerID, &now
		claimed = candidate
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &claimed, nil
}

// Complete releases the job and schedules its next run.
func (s *RouterPollJobService) Complete(job *models.RouterPollJob, took time.Duration) error {
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(RouterIntervalFor(job.Kind)),
		"last_run_at":          now,
		"last_duration_ms":     took.Milliseconds(),
		"last_error":           nil,
		"consecutive_failures": 0,
	})
}

// Fail releases the job and backs its next run off, so a router that is down
// is not retried every minute for as long as it stays down.
func (s *RouterPollJobService) Fail(job *models.RouterPollJob, cause error) error {
	failures := job.ConsecutiveFailures + 1
	now := time.Now()
	return s.release(job, map[string]interface{}{
		"due_at":               now.Add(routerBackoff(job.Kind, failures)),
		"last_run_at":          now,
		"last_error":           cause.Error(),
		"consecutive_failures": failures,
	})
}

func (s *RouterPollJobService) release(job *models.RouterPollJob, updates map[string]interface{}) error {
	updates["locked_by"] = nil
	updates["locked_at"] = nil
	return s.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ?", job.RouterID, job.Kind).
		Updates(updates).Error
}

func routerBackoff(kind models.RouterPollKind, failures int) time.Duration {
	shift := failures - 1
	if shift > maxBackoffShift {
		shift = maxBackoffShift
	}
	return RouterIntervalFor(kind) * time.Duration(1<<shift)
}

// RunNow makes the given tiers due immediately. It schedules rather than runs,
// so an HTTP request never waits on a router.
func (s *RouterPollJobService) RunNow(routerID uuid.UUID, kinds ...models.RouterPollKind) error {
	return s.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind IN ?", routerID, kinds).
		Update("due_at", time.Now()).Error
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestRouterClaim|TestRouterComplete|TestRouterFail|TestRouterRunNow|TestConcurrentRouterClaims' -v -race`
Expected: PASS. Test Postgres lulus kalau `TEST_POSTGRES_DSN` diisi, dan di-skip secara lokal kalau tidak (di CI wajib jalan).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/router_poll_jobs.go backend/internal/services/router_poll_jobs_test.go \
  backend/internal/services/router_poll_jobs_postgres_test.go
git commit -m "feat(mikrotik): claimable poll queue for routers"
```

---

### Task 8: Polling — counter interface, BGP, discovery

**Files:**
- Create: `backend/internal/services/router_poll.go`, `router_poll_test.go`
- Create: `backend/internal/services/router_discovery.go`, `router_discovery_test.go`

**Interfaces:**
- Consumes: `RouterService.Secrets`, `InterfaceRate`, `MergeBGP`, `MatchInterface`, `DiffBGP`, `connectivity.WalkIfCounters`, `connectivity.IfCounters`, `mikrotik.*`.
- Produces:
  - `services.RouterAPI` interface (`SystemResource`, `Clock`, `Interfaces`, `VLANs`, `Addresses`, `BGPConnections`, `BGPSessions`, signature sama dengan `*mikrotik.Client`)
  - `services.IfCounterReader func(ctx context.Context, host string, port int, community string) ([]connectivity.IfCounters, error)`
  - `services.NewRouterPollService(db *gorm.DB, routers *RouterService) *RouterPollService`
  - Methods: `PollInterfaces(router *models.MikrotikRouter, now time.Time) error`, `PollBGP(router *models.MikrotikRouter, now time.Time) error`, `Discover(router *models.MikrotikRouter) error`

- [ ] **Step 1: Tulis test polling yang gagal**

`backend/internal/services/router_poll_test.go`:

```go
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// fakeRouterAPI answers like a router; err, when set, is returned by every call.
type fakeRouterAPI struct {
	resource    mikrotik.SystemResource
	clock       mikrotik.Clock
	interfaces  []mikrotik.Interface
	vlans       []mikrotik.VLAN
	addresses   []mikrotik.Address
	connections []mikrotik.BGPConnection
	sessions    []mikrotik.BGPSession
	err         error
}

func (f *fakeRouterAPI) SystemResource() (mikrotik.SystemResource, error) { return f.resource, f.err }
func (f *fakeRouterAPI) Clock() (mikrotik.Clock, error)                   { return f.clock, f.err }
func (f *fakeRouterAPI) Interfaces() ([]mikrotik.Interface, error)        { return f.interfaces, f.err }
func (f *fakeRouterAPI) VLANs() ([]mikrotik.VLAN, error)                  { return f.vlans, f.err }
func (f *fakeRouterAPI) Addresses() ([]mikrotik.Address, error)           { return f.addresses, f.err }
func (f *fakeRouterAPI) BGPConnections() ([]mikrotik.BGPConnection, error) {
	return f.connections, f.err
}
func (f *fakeRouterAPI) BGPSessions() ([]mikrotik.BGPSession, error) { return f.sessions, f.err }

type pollFixture struct {
	service  *RouterPollService
	db       *gorm.DB
	router   *models.MikrotikRouter
	api      *fakeRouterAPI
	counters []connectivity.IfCounters
	snmpErr  error
}

func newPollFixture(t *testing.T) *pollFixture {
	t.Helper()
	routers, db := newRouterServiceForTest(t)
	router, err := routers.Create(validRouterInput())
	require.NoError(t, err)

	f := &pollFixture{db: db, router: router, api: &fakeRouterAPI{clock: mikrotik.Clock{GMTOffset: "+07:00"}}}
	f.service = &RouterPollService{
		db:      db,
		routers: routers,
		newAPI:  func(*models.MikrotikRouter, string) RouterAPI { return f.api },
		readCounters: func(context.Context, string, int, string) ([]connectivity.IfCounters, error) {
			return f.counters, f.snmpErr
		},
		fetchFingerprint: func(string, int) (string, error) { return "ef" + testFingerprint[2:], nil },
	}
	return f
}

func (f *pollFixture) addInterface(t *testing.T, name, addresses string, disabled bool) models.RouterInterface {
	t.Helper()
	iface := models.RouterInterface{RouterID: f.router.ID, Name: name, Type: "vlan", Addresses: addresses,
		Disabled: disabled, OperStatus: "unknown"}
	require.NoError(t, f.db.Create(&iface).Error)
	return iface
}

func TestPollInterfacesRecordsARateFromTheSecondSample(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{IfIndex: 19, Name: "vlan-transit", InOctets: 1_000, OutOctets: 2_000, SpeedMbps: 10000, OperUp: true}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	var samples int64
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.Zero(t, samples, "one counter reading is not a rate")

	f.counters[0].InOctets += 750_000_000
	f.counters[0].OutOctets += 75_000_000
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	var metric models.RouterInterfaceMetric
	require.NoError(t, f.db.First(&metric, "interface_id = ?", iface.ID).Error)
	assert.InDelta(t, 100_000_000, metric.InBps, 0.01)
	assert.InDelta(t, 10_000_000, metric.OutBps, 0.01)

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "id = ?", iface.ID).Error)
	require.NotNil(t, stored.IfIndex)
	assert.Equal(t, 19, *stored.IfIndex)
	assert.Equal(t, "up", stored.OperStatus)
	require.NotNil(t, stored.LastInBps)
	assert.InDelta(t, 100_000_000, *stored.LastInBps, 0.01)

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterStatusUp, router.Status)
}

func TestPollInterfacesDropsTheSampleAfterACounterReset(t *testing.T) {
	f := newPollFixture(t)
	f.addInterface(t, "vlan-transit", "", false)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{Name: "vlan-transit", InOctets: 9_000_000, OutOctets: 9_000_000}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))
	f.counters[0].InOctets, f.counters[0].OutOctets = 500, 500
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	var samples int64
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.Zero(t, samples)

	// The reset reading becomes the new baseline, so the next minute has a rate.
	f.counters[0].InOctets, f.counters[0].OutOctets = 500+7_500, 500+7_500
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(2*time.Minute)))
	f.db.Model(&models.RouterInterfaceMetric{}).Count(&samples)
	assert.EqualValues(t, 1, samples)
}

func TestPollInterfacesSkipsDisabledAndUnknownInterfaces(t *testing.T) {
	f := newPollFixture(t)
	off := f.addInterface(t, "vlan-old", "", true)
	start := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)

	f.counters = []connectivity.IfCounters{{Name: "vlan-old", InOctets: 1}, {Name: "<pppoe-x>", InOctets: 1}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "id = ?", off.ID).Error)
	assert.Nil(t, stored.PrevSampleAt)
}

func TestPollInterfacesMarksTheRouterDownWhenSNMPFails(t *testing.T) {
	f := newPollFixture(t)
	f.snmpErr = errors.New("request timeout")

	err := f.service.PollInterfaces(f.router, time.Now())

	assert.Error(t, err)
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterStatusDown, router.Status)
}

func TestPollBGPStoresSessionsMatchedToTheirVLANAndTheRoutersResources(t *testing.T) {
	f := newPollFixture(t)
	rs := f.addInterface(t, "vlan-rs", "198.18.0.140/23", false)
	f.addInterface(t, "vlan-transit", "198.51.100.1/31", false)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	f.api.resource = mikrotik.SystemResource{Uptime: "4w6d1h44m21s", Version: "7.24.2 (stable)", CPULoad: "21",
		FreeMemory: "12669456384", TotalMemory: "17179869184", BoardName: "CCR2116-12G-4S+"}

	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var sessions []models.RouterBGPSession
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Order("name").Find(&sessions).Error)
	require.Len(t, sessions, 5)
	for _, s := range sessions {
		if s.Name == "peer-rs2" {
			require.NotNil(t, s.InterfaceID)
			assert.Equal(t, rs.ID, *s.InterfaceID)
		}
	}

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusOK, router.APIStatus)
	require.NotNil(t, router.CPULoad)
	assert.EqualValues(t, 21, *router.CPULoad)
	require.NotNil(t, router.UptimeSeconds)
	assert.EqualValues(t, 34*86400+3600+44*60+21, *router.UptimeSeconds)
}

func TestPollBGPWritesEventsAtTheRoutersLocalTime(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	f.api.sessions[0].Established, f.api.sessions[0].LastStopped = "", "2026-10-12 08:00:05"
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var events []models.RouterBGPEvent
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "peer-transit", events[0].SessionName)
	assert.Equal(t, time.Date(2026, 10, 12, 1, 0, 5, 0, time.UTC), events[0].OccurredAt.UTC())
}

func TestPollBGPDropsSessionsNoLongerConfigured(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	f.api.connections = f.api.connections[:1]
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	var count int64
	f.db.Model(&models.RouterBGPSession{}).Where("router_id = ?", f.router.ID).Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestPollBGPRecordsAChangedCertificateWithWhatTheRouterNowPresents(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = mikrotik.ErrCertificateChanged

	err := f.service.PollBGP(f.router, time.Now())

	assert.True(t, errors.Is(err, mikrotik.ErrCertificateChanged))
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusCertificateChanged, router.APIStatus)
	require.NotNil(t, router.PresentedFingerprint)
	assert.Equal(t, "ef"+testFingerprint[2:], *router.PresentedFingerprint)
}

func TestPollBGPRecordsOtherAPIErrors(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = mikrotik.ErrUnauthorized

	assert.Error(t, f.service.PollBGP(f.router, time.Now()))

	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusError, router.APIStatus)
	require.NotNil(t, router.APIError)
	assert.Contains(t, *router.APIError, "rejected")
}

func TestPollBGPRejectsAnUnreadableClockOffset(t *testing.T) {
	f := newPollFixture(t)
	f.api.clock.GMTOffset = "WIB"
	f.api.connections = borderConnections()

	assert.Error(t, f.service.PollBGP(f.router, time.Now()), "guessing UTC would shift every event by hours")
}
```

- [ ] **Step 2: Tulis test discovery yang gagal**

`backend/internal/services/router_discovery_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

func borderInventory(api *fakeRouterAPI) {
	api.interfaces = []mikrotik.Interface{
		{Name: "sfp-sfpplus1", Type: "ether", Comment: "Uplink transit", Running: "true"},
		{Name: "vlan-transit", Type: "vlan", Running: "true"},
		{Name: "vlan-pop-a", Type: "vlan", Comment: "POP A via carrier", Running: "true"},
		{Name: "vlan-retired", Type: "vlan", Disabled: "true"},
		{Name: "<pppoe-12345>", Type: "pppoe-in", Dynamic: "true", Running: "true"},
	}
	api.vlans = []mikrotik.VLAN{
		{Name: "vlan-transit", VLANID: "527", Interface: "sfp-sfpplus1"},
		{Name: "vlan-pop-a", VLANID: "1113", Interface: "sfp-sfpplus1"},
	}
	api.addresses = []mikrotik.Address{
		{Address: "198.51.100.1/31", Interface: "vlan-transit"},
		{Address: "192.0.2.33/29", Interface: "vlan-pop-a"},
		{Address: "192.0.2.57/29", Interface: "vlan-pop-a"},
		{Address: "192.0.2.121/29", Interface: "vlan-retired", Invalid: "true"},
	}
}

func TestDiscoverStoresInterfacesWithVLANsAndAddresses(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)

	require.NoError(t, f.service.Discover(f.router))

	var ifaces []models.RouterInterface
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Order("name").Find(&ifaces).Error)
	names := make([]string, len(ifaces))
	for i, iface := range ifaces {
		names[i] = iface.Name
	}
	assert.ElementsMatch(t, []string{"sfp-sfpplus1", "vlan-transit", "vlan-pop-a", "vlan-retired"}, names,
		"a dynamic interface is renamed every time it comes up and is not kept")

	for _, iface := range ifaces {
		switch iface.Name {
		case "vlan-pop-a":
			require.NotNil(t, iface.VLANID)
			assert.Equal(t, 1113, *iface.VLANID)
			require.NotNil(t, iface.ParentName)
			assert.Equal(t, "sfp-sfpplus1", *iface.ParentName)
			assert.Equal(t, "POP A via carrier", iface.Comment)
			assert.Equal(t, "192.0.2.33/29,192.0.2.57/29", iface.Addresses)
		case "vlan-retired":
			assert.True(t, iface.Disabled)
			assert.Empty(t, iface.Addresses, "an invalid address is not a subnet a peer lives in")
		}
	}
}

func TestDiscoverKeepsAnInterfacesIdentityAndRemovesWhatIsGone(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	require.NoError(t, f.service.Discover(f.router))
	var before models.RouterInterface
	require.NoError(t, f.db.First(&before, "router_id = ? AND name = ?", f.router.ID, "vlan-pop-a").Error)

	f.api.interfaces = f.api.interfaces[:3]
	f.api.interfaces[2].Comment = "POP A, new carrier"
	require.NoError(t, f.service.Discover(f.router))

	var after models.RouterInterface
	require.NoError(t, f.db.First(&after, "router_id = ? AND name = ?", f.router.ID, "vlan-pop-a").Error)
	assert.Equal(t, before.ID, after.ID, "the graph history hangs off this ID")
	assert.Equal(t, "POP A, new carrier", after.Comment)

	var retired int64
	f.db.Model(&models.RouterInterface{}).Where("name = ?", "vlan-retired").Count(&retired)
	assert.Zero(t, retired)
}

// A router that answers with no interfaces is a fault (a user without read
// rights, a half-failed response), not a router with nothing on it. Accepting
// it would delete every interface and orphan all their graphs.
func TestDiscoverRefusesAnEmptyInterfaceList(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	require.NoError(t, f.service.Discover(f.router))

	f.api.interfaces = nil
	assert.Error(t, f.service.Discover(f.router))

	var count int64
	f.db.Model(&models.RouterInterface{}).Where("router_id = ?", f.router.ID).Count(&count)
	assert.EqualValues(t, 4, count)
}

func TestDiscoverRematchesBGPSessionsToInterfaces(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections = []mikrotik.BGPConnection{{Name: "peer-transit", RemoteAddress: "198.51.100.0/32", RemoteAS: "64500"}}
	require.NoError(t, f.service.PollBGP(f.router, nowForTest()))

	borderInventory(f.api)
	require.NoError(t, f.service.Discover(f.router))

	var session models.RouterBGPSession
	require.NoError(t, f.db.First(&session, "router_id = ? AND name = ?", f.router.ID, "peer-transit").Error)
	var transit models.RouterInterface
	require.NoError(t, f.db.First(&transit, "router_id = ? AND name = ?", f.router.ID, "vlan-transit").Error)
	require.NotNil(t, session.InterfaceID)
	assert.Equal(t, transit.ID, *session.InterfaceID)
}
```

Tambahkan helper kecil ini di akhir `router_discovery_test.go`:

```go
func nowForTest() time.Time { return time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC) }
```

dan tambahkan `"time"` ke import file itu.

- [ ] **Step 3: Jalankan, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestPoll|TestDiscover' -v`
Expected: FAIL, `undefined: RouterPollService`.

- [ ] **Step 4: Implementasi polling**

`backend/internal/services/router_poll.go`:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// snmpWalkBudget bounds one IF-MIB read as a whole. Five columns of about fifty
// rows each take a few GETBULK round trips; a router that needs longer is not
// answering.
const snmpWalkBudget = 10 * time.Second

// RouterAPI is what polling reads from a router's REST API. *mikrotik.Client
// satisfies it; tests pass a fake.
type RouterAPI interface {
	SystemResource() (mikrotik.SystemResource, error)
	Clock() (mikrotik.Clock, error)
	Interfaces() ([]mikrotik.Interface, error)
	VLANs() ([]mikrotik.VLAN, error)
	Addresses() ([]mikrotik.Address, error)
	BGPConnections() ([]mikrotik.BGPConnection, error)
	BGPSessions() ([]mikrotik.BGPSession, error)
}

// IfCounterReader reads IF-MIB counters; connectivity.WalkIfCounters in
// production.
type IfCounterReader func(ctx context.Context, host string, port int, community string) ([]connectivity.IfCounters, error)

// RouterPollService runs the work of each router poll tier.
type RouterPollService struct {
	db               *gorm.DB
	routers          *RouterService
	newAPI           func(router *models.MikrotikRouter, password string) RouterAPI
	readCounters     IfCounterReader
	fetchFingerprint func(host string, port int) (string, error)
}

// NewRouterPollService creates a poll service that talks to real routers.
func NewRouterPollService(db *gorm.DB, routers *RouterService) *RouterPollService {
	return &RouterPollService{
		db:      db,
		routers: routers,
		newAPI: func(router *models.MikrotikRouter, password string) RouterAPI {
			return mikrotik.NewClient(router.Host, router.APIPort, router.APIUsername, password, router.TLSFingerprint)
		},
		readCounters:     connectivity.WalkIfCounters,
		fetchFingerprint: mikrotik.FetchFingerprint,
	}
}

// PollInterfaces reads every interface's counters and records a rate for each
// enabled interface that has an earlier sample to compare against.
func (s *RouterPollService) PollInterfaces(router *models.MikrotikRouter, now time.Time) error {
	community, _, err := s.routers.Secrets(router)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), snmpWalkBudget)
	defer cancel()
	counters, err := s.readCounters(ctx, router.Host, router.SNMPPort, community)
	if err != nil {
		s.db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Update("status", models.RouterStatusDown)
		return fmt.Errorf("read interface counters: %w", err)
	}

	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ? AND disabled = ?", router.ID, false).Find(&ifaces).Error; err != nil {
		return err
	}
	byName := make(map[string]models.RouterInterface, len(ifaces))
	for _, iface := range ifaces {
		byName[iface.Name] = iface
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, c := range counters {
			iface, ok := byName[c.Name]
			if !ok {
				continue
			}
			if err := recordCounters(tx, iface, c, now); err != nil {
				return err
			}
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).
			Updates(map[string]interface{}{"status": models.RouterStatusUp, "last_seen_at": now}).Error
	})
}

func recordCounters(tx *gorm.DB, iface models.RouterInterface, c connectivity.IfCounters, now time.Time) error {
	updates := map[string]interface{}{
		"if_index": c.IfIndex, "speed_mbps": int64(c.SpeedMbps), "oper_status": operStatus(c.OperUp),
		"prev_in_octets": int64(c.InOctets), "prev_out_octets": int64(c.OutOctets),
		"prev_sample_at": now, "last_polled_at": now,
	}
	if iface.PrevSampleAt != nil && iface.PrevInOctets != nil && iface.PrevOutOctets != nil {
		inBps, inOK := InterfaceRate(uint64(*iface.PrevInOctets), c.InOctets, *iface.PrevSampleAt, now)
		outBps, outOK := InterfaceRate(uint64(*iface.PrevOutOctets), c.OutOctets, *iface.PrevSampleAt, now)
		if inOK && outOK {
			sample := models.RouterInterfaceMetric{Time: now, InterfaceID: iface.ID, InBps: inBps, OutBps: outBps}
			if err := tx.Create(&sample).Error; err != nil {
				return err
			}
			updates["last_in_bps"], updates["last_out_bps"] = inBps, outBps
		}
	}
	return tx.Model(&models.RouterInterface{}).Where("id = ?", iface.ID).Updates(updates).Error
}

func operStatus(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

// bgpSnapshot is everything one BGP poll reads from the router.
type bgpSnapshot struct {
	resource    mikrotik.SystemResource
	location    *time.Location
	connections []mikrotik.BGPConnection
	sessions    []mikrotik.BGPSession
}

// PollBGP stores each configured peer's state, an event for each change, and
// the router's resource summary.
func (s *RouterPollService) PollBGP(router *models.MikrotikRouter, now time.Time) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	snapshot, err := readBGPSnapshot(api)
	if err != nil {
		return s.recordAPIError(router, err)
	}

	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ?", router.ID).Find(&ifaces).Error; err != nil {
		return err
	}
	var stored []models.RouterBGPSession
	if err := s.db.Where("router_id = ?", router.ID).Find(&stored).Error; err != nil {
		return err
	}
	peers := MergeBGP(snapshot.connections, snapshot.sessions, snapshot.location)
	events := DiffBGP(router.ID, stored, peers, now)

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := replaceSessions(tx, router.ID, peers, ifaces); err != nil {
			return err
		}
		if len(events) > 0 {
			if err := tx.Create(&events).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).
			Updates(resourceUpdates(snapshot.resource)).Error
	})
}

func readBGPSnapshot(api RouterAPI) (bgpSnapshot, error) {
	var snapshot bgpSnapshot
	var err error
	if snapshot.resource, err = api.SystemResource(); err != nil {
		return snapshot, err
	}
	clock, err := api.Clock()
	if err != nil {
		return snapshot, err
	}
	if snapshot.location, err = mikrotik.ParseOffset(clock.GMTOffset); err != nil {
		return snapshot, err
	}
	if snapshot.connections, err = api.BGPConnections(); err != nil {
		return snapshot, err
	}
	snapshot.sessions, err = api.BGPSessions()
	return snapshot, err
}

func replaceSessions(tx *gorm.DB, routerID uuid.UUID, peers []BGPPeerState, ifaces []models.RouterInterface) error {
	names := make([]string, 0, len(peers))
	for _, p := range peers {
		names = append(names, p.Name)
		row := models.RouterBGPSession{
			RouterID: routerID, Name: p.Name, RemoteAddress: p.RemoteAddress, RemoteAS: p.RemoteAS,
			LocalAddress: p.LocalAddress, State: p.State, UptimeSeconds: p.UptimeSeconds,
			PrefixCount: p.PrefixCount, LastStartedAt: p.LastStartedAt, LastStoppedAt: p.LastStoppedAt,
			InterfaceID: MatchInterface(p.RemoteAddress, ifaces),
		}
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "router_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"remote_address", "remote_as", "local_address", "state",
				"uptime_seconds", "prefix_count", "last_started_at", "last_stopped_at", "interface_id", "updated_at"}),
		}).Create(&row).Error
		if err != nil {
			return err
		}
	}
	gone := tx.Where("router_id = ?", routerID)
	if len(names) > 0 {
		gone = gone.Where("name NOT IN ?", names)
	}
	return gone.Delete(&models.RouterBGPSession{}).Error
}

func resourceUpdates(r mikrotik.SystemResource) map[string]interface{} {
	updates := map[string]interface{}{
		"api_status": models.RouterAPIStatusOK, "api_error": nil, "presented_fingerprint": nil,
		"ros_version": r.Version, "board_name": r.BoardName,
	}
	for column, value := range map[string]string{"cpu_load": r.CPULoad, "memory_total": r.TotalMemory, "memory_free": r.FreeMemory} {
		if n, err := mikrotik.ParseInt(value); err == nil {
			updates[column] = n
		}
	}
	if uptime, err := mikrotik.ParseDuration(r.Uptime); err == nil {
		updates["uptime_seconds"] = int64(uptime.Seconds())
	}
	return updates
}

func (s *RouterPollService) apiFor(router *models.MikrotikRouter) (RouterAPI, error) {
	_, password, err := s.routers.Secrets(router)
	if err != nil {
		return nil, err
	}
	return s.newAPI(router, password), nil
}

// recordAPIError stores why REST failed and returns the cause for the job to
// fail on. A changed certificate also stores what the router presents now, so
// an admin can see and accept it.
func (s *RouterPollService) recordAPIError(router *models.MikrotikRouter, cause error) error {
	updates := map[string]interface{}{"api_status": models.RouterAPIStatusError, "api_error": cause.Error()}
	if errors.Is(cause, mikrotik.ErrCertificateChanged) {
		updates["api_status"] = models.RouterAPIStatusCertificateChanged
		if fingerprint, err := s.fetchFingerprint(router.Host, router.APIPort); err == nil {
			updates["presented_fingerprint"] = fingerprint
		}
	}
	if err := s.db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("%w (recording it also failed: %v)", cause, err)
	}
	return cause
}
```

- [ ] **Step 5: Implementasi discovery**

`backend/internal/services/router_discovery.go`:

```go
package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errNoInterfaces refuses a discovery that would empty the router: every
// interface and the history hanging off it would go with that one reply.
var errNoInterfaces = errors.New("router reported no interfaces; keeping the ones already known")

type routerInventory struct {
	interfaces []mikrotik.Interface
	vlans      []mikrotik.VLAN
	addresses  []mikrotik.Address
}

// Discover brings the stored interfaces in line with the router's, then
// re-matches BGP sessions to them.
func (s *RouterPollService) Discover(router *models.MikrotikRouter) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	inventory, err := readInventory(api)
	if err != nil {
		return s.recordAPIError(router, err)
	}
	rows := buildInterfaceRows(router.ID, inventory)
	if len(rows) == 0 {
		return errNoInterfaces
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertInterfaces(tx, router.ID, rows); err != nil {
			return err
		}
		return rematchSessions(tx, router.ID)
	})
}

func readInventory(api RouterAPI) (routerInventory, error) {
	var inventory routerInventory
	var err error
	if inventory.interfaces, err = api.Interfaces(); err != nil {
		return inventory, err
	}
	if inventory.vlans, err = api.VLANs(); err != nil {
		return inventory, err
	}
	inventory.addresses, err = api.Addresses()
	return inventory, err
}

func buildInterfaceRows(routerID uuid.UUID, inventory routerInventory) []models.RouterInterface {
	vlans := make(map[string]mikrotik.VLAN, len(inventory.vlans))
	for _, v := range inventory.vlans {
		vlans[v.Name] = v
	}
	addresses := map[string][]string{}
	for _, a := range inventory.addresses {
		if !mikrotik.ParseBool(a.Disabled) && !mikrotik.ParseBool(a.Invalid) {
			addresses[a.Interface] = append(addresses[a.Interface], a.Address)
		}
	}

	rows := make([]models.RouterInterface, 0, len(inventory.interfaces))
	for _, i := range inventory.interfaces {
		// A dynamic interface (one pppoe-in per connection) gets a new name
		// each time it comes up, so keyed on its name it would be a new
		// interface every time.
		if mikrotik.ParseBool(i.Dynamic) {
			continue
		}
		row := models.RouterInterface{
			RouterID: routerID, Name: i.Name, Comment: i.Comment, Type: i.Type, OperStatus: "unknown",
			Disabled: mikrotik.ParseBool(i.Disabled), Addresses: strings.Join(addresses[i.Name], ","),
		}
		if vlan, ok := vlans[i.Name]; ok {
			applyVLAN(&row, vlan)
		}
		rows = append(rows, row)
	}
	return rows
}

func applyVLAN(row *models.RouterInterface, vlan mikrotik.VLAN) {
	parent := vlan.Interface
	row.ParentName = &parent
	if id, err := mikrotik.ParseInt(vlan.VLANID); err == nil {
		vlanID := int(id)
		row.VLANID = &vlanID
	}
}

func upsertInterfaces(tx *gorm.DB, routerID uuid.UUID, rows []models.RouterInterface) error {
	names := make([]string, 0, len(rows))
	for i := range rows {
		names = append(names, rows[i].Name)
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "router_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"comment", "type", "vlan_id", "parent_name",
				"disabled", "addresses", "updated_at"}),
		}).Create(&rows[i]).Error
		if err != nil {
			return err
		}
	}
	return tx.Where("router_id = ? AND name NOT IN ?", routerID, names).Delete(&models.RouterInterface{}).Error
}

func rematchSessions(tx *gorm.DB, routerID uuid.UUID) error {
	var ifaces []models.RouterInterface
	if err := tx.Where("router_id = ?", routerID).Find(&ifaces).Error; err != nil {
		return err
	}
	var sessions []models.RouterBGPSession
	if err := tx.Where("router_id = ?", routerID).Find(&sessions).Error; err != nil {
		return err
	}
	for _, session := range sessions {
		err := tx.Model(&models.RouterBGPSession{}).Where("id = ?", session.ID).
			Update("interface_id", MatchInterface(session.RemoteAddress, ifaces)).Error
		if err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestPoll|TestDiscover' -v -race`
Expected: PASS.

Kalau `TestDiscoverKeepsAnInterfacesIdentity` gagal karena ID berubah, artinya `OnConflict` di SQLite memperbarui kolom `id`. Pastikan `id` tidak ada di `AssignmentColumns`, lalu cek bahwa `BeforeCreate` tidak dipanggil ulang untuk baris yang konflik.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/router_poll.go backend/internal/services/router_poll_test.go \
  backend/internal/services/router_discovery.go backend/internal/services/router_discovery_test.go
git commit -m "feat(mikrotik): poll interface counters, BGP state and inventory"
```

---

### Task 9: Worker menjalankan job router

**Files:**
- Create: `backend/cmd/worker/router_jobs.go`
- Modify: `backend/cmd/worker/jobs.go` (struct `workerRuntime`)
- Modify: `backend/cmd/worker/main.go` (konstruksi dan loop)

**Interfaces:**
- Consumes: `RouterPollJobService`, `RouterService`, `RouterPollService` (Task 6–8).
- Produces: `runNextRouterJob(rt *workerRuntime) bool`.

`cmd/*` dikecualikan dari baseline coverage dan dari batas 50 baris untuk `main()`. Logika yang dijalankan di sini sudah diuji di Task 7–8; task ini hanya menyambungkan.

- [ ] **Step 1: Tambah field runtime**

Di `backend/cmd/worker/jobs.go`, sisipkan tiga field ini di struct `workerRuntime`, di antara field `events` dan `logger` yang sudah ada:

```go
	routers    *services.RouterService
	routerJobs *services.RouterPollJobService
	routerPoll *services.RouterPollService
```

Lalu jalankan `gofmt -w cmd/worker/jobs.go` untuk merapikan perataan.

- [ ] **Step 2: Tulis runner**

`backend/cmd/worker/router_jobs.go`:

```go
package main

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
)

// runNextRouterJob claims one router job and runs it, reporting whether there
// was work.
func runNextRouterJob(rt *workerRuntime) bool {
	job, err := rt.routerJobs.Claim(rt.id)
	if err != nil {
		rt.logger.Error("Failed to claim a router poll job", zap.Error(err))
		time.Sleep(errorWait)
		return true
	}
	if job == nil {
		return false
	}

	router, err := rt.routers.Get(job.RouterID)
	if err != nil || !router.Enabled {
		// Gone (its jobs follow by foreign key) or switched off: either way
		// there is nothing to read, and that is not a failure to back off over.
		if completeErr := rt.routerJobs.Complete(job, 0); completeErr != nil {
			rt.logger.Error("Failed to release a router job", zap.Error(completeErr))
		}
		return true
	}

	started := time.Now()
	if err := runRouterJob(rt, job, router); err != nil {
		rt.logger.Warn("Router poll job failed",
			zap.String("router", router.Name), zap.String("kind", string(job.Kind)), zap.Error(err))
		if failErr := rt.routerJobs.Fail(job, err); failErr != nil {
			rt.logger.Error("Failed to record a router job failure", zap.Error(failErr))
		}
		return true
	}
	if err := rt.routerJobs.Complete(job, time.Since(started)); err != nil {
		rt.logger.Error("Failed to record a router job completion", zap.Error(err))
	}
	return true
}

func runRouterJob(rt *workerRuntime, job *models.RouterPollJob, router *models.MikrotikRouter) error {
	switch job.Kind {
	case models.RouterPollKindInterfaces:
		return rt.routerPoll.PollInterfaces(router, time.Now())
	case models.RouterPollKindBGP:
		return rt.routerPoll.PollBGP(router, time.Now())
	case models.RouterPollKindDiscovery:
		return rt.routerPoll.Discover(router)
	default:
		return fmt.Errorf("unknown router poll kind %q", job.Kind)
	}
}
```

- [ ] **Step 3: Sambungkan di main**

Di `backend/cmd/worker/main.go`, setelah `eventService := services.NewEventService(db)`:

```go
	routerService := services.NewRouterService(db, cfg.EncryptionKey)
```

Di literal `rt := &workerRuntime{`, tambahkan:

```go
		routers:    routerService,
		routerJobs: services.NewRouterPollJobService(db),
		routerPoll: services.NewRouterPollService(db, routerService),
```

Ganti blok di dalam loop:

```go
		if !runNextJob(rt) {
			// Nothing is due. Sleeping here rather than spinning is what keeps
			// an idle worker off the database.
			time.Sleep(idleWait)
		}
```

dengan:

```go
		// Both queues every pass, so a busy OLT queue cannot starve the
		// routers' minute-level polls, nor the reverse.
		ranOLTJob := runNextJob(rt)
		ranRouterJob := runNextRouterJob(rt)
		if !ranOLTJob && !ranRouterJob {
			// Nothing is due. Sleeping here rather than spinning is what keeps
			// an idle worker off the database.
			time.Sleep(idleWait)
		}
```

- [ ] **Step 4: Build, vet, test**

Run: `cd backend && go build ./cmd/worker/ && go vet ./cmd/worker/ && go test ./cmd/worker/ -count=1`
Expected: build dan vet bersih, test worker yang sudah ada tetap PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/worker/
git commit -m "feat(worker): run router poll jobs alongside the OLT queue"
```

---

### Task 10: Query untuk halaman

**Files:**
- Create: `backend/internal/services/router_query.go`, `router_query_test.go`

**Interfaces:**
- Consumes: `rawRetention`, `fiveMinRetention` dari `metrics_source.go` (nilai yang sama dengan retensi migrasi 56).
- Produces:
  - `services.NewRouterQueryService(db) *RouterQueryService`
  - `services.TrafficPoint{Time time.Time; InBps, OutBps float64}` (JSON `time`, `in_bps`, `out_bps`)
  - `services.BGPCount{RouterID uuid.UUID; Established, Total int}`
  - Methods: `ListInterfaces(routerID) ([]models.RouterInterface, error)`, `ListSessions(routerID) ([]models.RouterBGPSession, error)`, `ListEvents(routerID, from, to time.Time) ([]models.RouterBGPEvent, error)`, `InterfaceTraffic(routerID, interfaceID uuid.UUID, from, to time.Time) ([]TrafficPoint, error)`, `BGPSummary() (map[uuid.UUID]BGPCount, error)`

- [ ] **Step 1: Tulis test yang gagal**

`backend/internal/services/router_query_test.go`:

```go
package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func TestRouterTrafficSourceFollowsSpanAndRetention(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		from, to time.Time
		want     routerTrafficSource
	}{
		{"last six hours", now.Add(-6 * time.Hour), now, routerTrafficRaw},
		{"last day", now.Add(-24 * time.Hour), now, routerTraffic5Min},
		{"last week", now.Add(-7 * 24 * time.Hour), now, routerTraffic5Min},
		{"last month", now.Add(-30 * 24 * time.Hour), now, routerTrafficHourly},
		// One hour, but ten days back: raw is kept seven days, so the short
		// span alone would read rows retention has already dropped.
		{"one hour ten days ago", now.Add(-10 * 24 * time.Hour), now.Add(-10*24*time.Hour + time.Hour), routerTraffic5Min},
		{"one hour forty days ago", now.Add(-40 * 24 * time.Hour), now.Add(-40*24*time.Hour + time.Hour), routerTrafficHourly},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, routerTrafficSourceFor(c.from, c.to, now), c.name)
	}
}

func TestInterfaceTrafficReadsRawSamplesInOrder(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)
	now := time.Now().UTC().Truncate(time.Minute)
	for i, bps := range []float64{3e8, 1e8, 2e8} {
		require.NoError(t, f.db.Create(&models.RouterInterfaceMetric{
			Time: now.Add(-time.Duration(3-i) * time.Minute), InterfaceID: iface.ID, InBps: bps, OutBps: bps / 10,
		}).Error)
	}
	queries := NewRouterQueryService(f.db)

	points, err := queries.InterfaceTraffic(f.router.ID, iface.ID, now.Add(-time.Hour), now)

	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.InDelta(t, 3e8, points[0].InBps, 1)
	assert.InDelta(t, 2e7, points[2].OutBps, 1)
	assert.True(t, points[0].Time.Before(points[1].Time))
}

func TestInterfaceTrafficRefusesAnInterfaceOfAnotherRouter(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)

	_, err := NewRouterQueryService(f.db).InterfaceTraffic(uuid.New(), iface.ID, time.Now().Add(-time.Hour), time.Now())

	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestBGPSummaryCountsEstablishedSessionsPerRouter(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	summary, err := NewRouterQueryService(f.db).BGPSummary()

	require.NoError(t, err)
	assert.Equal(t, 4, summary[f.router.ID].Established)
	assert.Equal(t, 5, summary[f.router.ID].Total)
}

func TestListEventsIsNewestFirstWithinTheRange(t *testing.T) {
	f := newPollFixture(t)
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, f.db.Create(&models.RouterBGPEvent{RouterID: f.router.ID, SessionName: "p",
			FromState: models.BGPStateEstablished, ToState: models.BGPStateDown,
			OccurredAt: base.Add(time.Duration(i) * time.Hour)}).Error)
	}

	events, err := NewRouterQueryService(f.db).ListEvents(f.router.ID, base.Add(30*time.Minute), base.Add(3*time.Hour))

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.True(t, events[0].OccurredAt.After(events[1].OccurredAt))
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run 'TestRouterTrafficSource|TestInterfaceTraffic|TestBGPSummary|TestListEvents' -v`
Expected: FAIL, `undefined: routerTrafficSourceFor`.

- [ ] **Step 3: Implementasi**

`backend/internal/services/router_query.go`:

```go
package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// routerTrafficSource is the relation a traffic graph reads from.
type routerTrafficSource string

const (
	routerTrafficRaw    routerTrafficSource = "router_interface_metrics"
	routerTraffic5Min   routerTrafficSource = "router_interface_metrics_5min"
	routerTrafficHourly routerTrafficSource = "router_interface_metrics_hourly"
)

// Longest span each resolution is drawn at: past these, a graph has more
// points than pixels.
const (
	routerRawSpan  = 6 * time.Hour
	router5MinSpan = 7 * 24 * time.Hour
)

// maxEventRows bounds the event list; a flapping session over a long range
// would otherwise return thousands of rows to a page that shows a few dozen.
const maxEventRows = 500

// routerTrafficSourceFor picks the finest source fit for the span that still
// holds the range's start. Span alone is not enough: an hour ten days back is
// short, but raw rows that old are already gone.
func routerTrafficSourceFor(from, to, now time.Time) routerTrafficSource {
	span, age := to.Sub(from), now.Sub(from)
	switch {
	case span <= routerRawSpan && age < rawRetention:
		return routerTrafficRaw
	case span <= router5MinSpan && age < fiveMinRetention:
		return routerTraffic5Min
	default:
		return routerTrafficHourly
	}
}

// TrafficPoint is one point on an interface's traffic graph.
type TrafficPoint struct {
	Time   time.Time `json:"time"`
	InBps  float64   `json:"in_bps"`
	OutBps float64   `json:"out_bps"`
}

// BGPCount is a router's session tally for the router list.
type BGPCount struct {
	RouterID    uuid.UUID
	Established int
	Total       int
}

// RouterQueryService reads what the router pages show.
type RouterQueryService struct {
	db *gorm.DB
}

// NewRouterQueryService creates a router query service.
func NewRouterQueryService(db *gorm.DB) *RouterQueryService {
	return &RouterQueryService{db: db}
}

// ListInterfaces returns a router's interfaces grouped by parent port.
func (s *RouterQueryService) ListInterfaces(routerID uuid.UUID) ([]models.RouterInterface, error) {
	var ifaces []models.RouterInterface
	return ifaces, s.db.Where("router_id = ?", routerID).Order("parent_name, name").Find(&ifaces).Error
}

// ListSessions returns a router's BGP sessions by name.
func (s *RouterQueryService) ListSessions(routerID uuid.UUID) ([]models.RouterBGPSession, error) {
	var sessions []models.RouterBGPSession
	return sessions, s.db.Where("router_id = ?", routerID).Order("name").Find(&sessions).Error
}

// ListEvents returns a router's BGP events in a range, newest first.
func (s *RouterQueryService) ListEvents(routerID uuid.UUID, from, to time.Time) ([]models.RouterBGPEvent, error) {
	var events []models.RouterBGPEvent
	return events, s.db.Where("router_id = ? AND occurred_at >= ? AND occurred_at <= ?", routerID, from, to).
		Order("occurred_at DESC").Limit(maxEventRows).Find(&events).Error
}

// InterfaceTraffic returns an interface's traffic over a range. The interface
// must belong to the router, so one router's page cannot read another's.
func (s *RouterQueryService) InterfaceTraffic(routerID, interfaceID uuid.UUID, from, to time.Time) ([]TrafficPoint, error) {
	var iface models.RouterInterface
	if err := s.db.Where("id = ? AND router_id = ?", interfaceID, routerID).First(&iface).Error; err != nil {
		return nil, err
	}

	source := routerTrafficSourceFor(from, to, time.Now())
	query := `SELECT time, in_bps, out_bps FROM router_interface_metrics
		WHERE interface_id = ? AND time >= ? AND time <= ? ORDER BY time`
	if source != routerTrafficRaw {
		// source is one of the constants above, never request input.
		query = fmt.Sprintf(`SELECT bucket AS time, avg_in_bps AS in_bps, avg_out_bps AS out_bps FROM %s
			WHERE interface_id = ? AND bucket >= ? AND bucket <= ? ORDER BY bucket`, source)
	}

	var points []TrafficPoint
	return points, s.db.Raw(query, interfaceID, from, to).Scan(&points).Error
}

// BGPSummary tallies sessions per router for the router list.
func (s *RouterQueryService) BGPSummary() (map[uuid.UUID]BGPCount, error) {
	var rows []BGPCount
	err := s.db.Model(&models.RouterBGPSession{}).
		Select("router_id, SUM(CASE WHEN state = ? THEN 1 ELSE 0 END) AS established, COUNT(*) AS total",
			models.BGPStateEstablished).
		Group("router_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	summary := make(map[uuid.UUID]BGPCount, len(rows))
	for _, row := range rows {
		summary[row.RouterID] = row
	}
	return summary, nil
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run 'TestRouterTrafficSource|TestInterfaceTraffic|TestBGPSummary|TestListEvents' -v`
Expected: PASS.

Pembacaan dari dua view rollup tidak bisa diuji di SQLite. Itu dibuktikan di Task 14, langkah verifikasi produksi (grafik 7 hari dan 30 hari terisi).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/router_query.go backend/internal/services/router_query_test.go
git commit -m "feat(mikrotik): read interfaces, sessions, events and traffic for the pages"
```

---

### Task 11: API `/api/v1/routers`

**Files:**
- Create: `backend/internal/api/router_dto.go`, `router_handler.go`, `router_handler_reads.go`, `router_routes_mikrotik.go`, `router_handler_test.go`
- Modify: `backend/internal/api/router.go` (satu panggilan), `backend/internal/api/router_handlers.go` (field + konstruksi), `backend/internal/api/router_rbac_test.go`

**Interfaces:**
- Consumes: service dari Task 6, 7, 10.
- Produces: route di spec bagian 6. Response router: objek `MikrotikRouter` (tanpa rahasia) ditambah `bgp_established` dan `bgp_total`.

- [ ] **Step 1: Tulis test handler yang gagal**

`backend/internal/api/router_handler_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

var routerTestFingerprint = strings.Repeat("ab", 32)

func setupRouterHandlerTest(t *testing.T) (*RouterHandler, *gorm.DB) {
	t.Helper()
	db := TestDB(t)
	handler := NewRouterHandler(services.NewRouterService(db, testEncryptionKey),
		services.NewRouterPollJobService(db), services.NewRouterQueryService(db), nil)
	return handler, db
}

func createRouterRequest() CreateRouterRequest {
	return CreateRouterRequest{Name: "Border", Host: "192.0.2.1", SNMPCommunity: "c-secret",
		APIUsername: "tikman", APIPassword: "p-secret", TLSFingerprint: routerTestFingerprint}
}

func createRouterVia(t *testing.T, handler *RouterHandler) RouterResponse {
	t.Helper()
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", createRouterRequest())
	handler.Create(c)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var out RouterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestCreateRouterNeverEchoesItsSecrets(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", createRouterRequest())

	handler.Create(c)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.NotContains(t, w.Body.String(), "c-secret")
	assert.NotContains(t, w.Body.String(), "p-secret")
	assert.NotContains(t, w.Body.String(), "snmp_community")
	assert.Contains(t, w.Body.String(), `"api_port":8443`)
}

func TestCreateRouterRejectsAFingerprintThatIsNotOne(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	req := createRouterRequest()
	req.TLSFingerprint = "not-a-fingerprint"
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListRoutersCarriesTheBGPTally(t *testing.T) {
	handler, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	for _, s := range []models.RouterBGPSession{
		{RouterID: created.ID, Name: "a", State: models.BGPStateEstablished},
		{RouterID: created.ID, Name: "b", State: models.BGPStateDown},
	} {
		require.NoError(t, db.Create(&s).Error)
	}
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers", nil)

	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	var out []RouterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
	assert.Equal(t, 1, out[0].BGPEstablished)
	assert.Equal(t, 2, out[0].BGPTotal)
}

func TestDeleteUnknownRouterIs404(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	id := uuid.New().String()
	w, c := SetupTestContext(http.MethodDelete, "/api/v1/routers/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	handler.Delete(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTrustCertificateRefusesAFingerprintTheRouterNeverPresented(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers/"+created.ID.String()+"/trust-certificate",
		TrustCertificateRequest{Fingerprint: strings.Repeat("cd", 32)})
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

	handler.TrustCertificate(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTrafficRejectsABackwardsRange(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	path := "/api/v1/routers/" + created.ID.String() + "/interfaces/" + uuid.NewString() +
		"/traffic?from=2026-10-12T10:00:00Z&to=2026-10-12T09:00:00Z"
	w, c := SetupTestContext(http.MethodGet, path, nil)
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}, {Key: "ifId", Value: uuid.NewString()}}

	handler.Traffic(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInterfacesOfUnknownRouterIs404(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	id := uuid.NewString()
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+id+"/interfaces", nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	handler.Interfaces(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRefreshMakesDiscoveryDue(t *testing.T) {
	handler, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	require.NoError(t, db.Model(&models.RouterPollJob{}).Where("router_id = ?", created.ID).
		Update("due_at", "2099-01-01").Error)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers/"+created.ID.String()+"/refresh", nil)
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

	handler.Refresh(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	var job models.RouterPollJob
	require.NoError(t, db.First(&job, "router_id = ? AND kind = ?", created.ID, models.RouterPollKindDiscovery).Error)
	assert.Less(t, job.DueAt.Year(), 2099)
}
```

Tambahkan ke `backend/internal/api/router_rbac_test.go`:

```go
func TestRouterWritesFollowTheirRoles(t *testing.T) {
	router, store := rbacTestRouter(t)
	id := uuid.NewString()

	assert.Equal(t, http.StatusOK, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, "/api/v1/routers"))
	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleViewer, http.MethodPost, "/api/v1/routers"))
	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodPost, "/api/v1/routers"))
	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodDelete, "/api/v1/routers/"+id))
	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodPost, "/api/v1/routers/"+id+"/trust-certificate"))
	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleViewer, http.MethodPost, "/api/v1/routers/"+id+"/refresh"))
	assert.NotEqual(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodPost, "/api/v1/routers/"+id+"/refresh"))
	assert.NotEqual(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodPut, "/api/v1/routers/"+id))
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run 'Router' -v`
Expected: FAIL, `undefined: NewRouterHandler`.

- [ ] **Step 3: DTO**

`backend/internal/api/router_dto.go`:

```go
package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// CreateRouterRequest adds a router. TLSFingerprint is the one the connection
// test returned.
type CreateRouterRequest struct {
	Name           string     `json:"name" binding:"required,min=2,max=255"`
	Host           string     `json:"host" binding:"required,max=255"`
	SiteID         *uuid.UUID `json:"site_id"`
	SNMPCommunity  string     `json:"snmp_community" binding:"required"`
	SNMPPort       int        `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername    string     `json:"api_username" binding:"required,max=64"`
	APIPassword    string     `json:"api_password" binding:"required"`
	APIPort        int        `json:"api_port" binding:"omitempty,min=1,max=65535"`
	TLSFingerprint string     `json:"tls_fingerprint" binding:"required,len=64,hexadecimal"`
}

func (r CreateRouterRequest) toInput() services.RouterInput {
	return services.RouterInput{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, SNMPCommunity: r.SNMPCommunity, SNMPPort: r.SNMPPort,
		APIUsername: r.APIUsername, APIPassword: r.APIPassword, APIPort: r.APIPort, TLSFingerprint: r.TLSFingerprint,
	}
}

// UpdateRouterRequest changes a router. An omitted or empty secret keeps the
// stored one.
type UpdateRouterRequest struct {
	Name          *string    `json:"name" binding:"omitempty,min=2,max=255"`
	Host          *string    `json:"host" binding:"omitempty,max=255"`
	SiteID        *uuid.UUID `json:"site_id"`
	ClearSite     bool       `json:"clear_site"`
	SNMPCommunity *string    `json:"snmp_community"`
	SNMPPort      *int       `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername   *string    `json:"api_username" binding:"omitempty,max=64"`
	APIPassword   *string    `json:"api_password"`
	APIPort       *int       `json:"api_port" binding:"omitempty,min=1,max=65535"`
	Enabled       *bool      `json:"enabled"`
}

func (r UpdateRouterRequest) toUpdate() services.RouterUpdate {
	return services.RouterUpdate{
		Name: r.Name, Host: r.Host, SiteID: r.SiteID, ClearSite: r.ClearSite, SNMPCommunity: r.SNMPCommunity,
		SNMPPort: r.SNMPPort, APIUsername: r.APIUsername, APIPassword: r.APIPassword, APIPort: r.APIPort,
		Enabled: r.Enabled,
	}
}

// TestRouterConnectionRequest is the add-router form before anything is
// stored.
type TestRouterConnectionRequest struct {
	Host          string `json:"host" binding:"required,max=255"`
	SNMPCommunity string `json:"snmp_community" binding:"required"`
	SNMPPort      int    `json:"snmp_port" binding:"omitempty,min=1,max=65535"`
	APIUsername   string `json:"api_username" binding:"required"`
	APIPassword   string `json:"api_password" binding:"required"`
	APIPort       int    `json:"api_port" binding:"omitempty,min=1,max=65535"`
}

func (r TestRouterConnectionRequest) toInput() services.ConnectionTestInput {
	return services.ConnectionTestInput{
		Host: r.Host, SNMPPort: r.SNMPPort, SNMPCommunity: r.SNMPCommunity,
		APIPort: r.APIPort, APIUsername: r.APIUsername, APIPassword: r.APIPassword,
	}
}

// TrustCertificateRequest accepts the certificate the router now presents.
type TrustCertificateRequest struct {
	Fingerprint string `json:"fingerprint" binding:"required,len=64,hexadecimal"`
}

// RouterResponse is a router without its secrets, with its BGP tally.
type RouterResponse struct {
	models.MikrotikRouter
	BGPEstablished int `json:"bgp_established"`
	BGPTotal       int `json:"bgp_total"`
}
```

- [ ] **Step 4: Handler tulis**

`backend/internal/api/router_handler.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

const routerAuditResource = "mikrotik_router"

// RouterHandler serves the MikroTik router pages.
type RouterHandler struct {
	routers *services.RouterService
	jobs    *services.RouterPollJobService
	queries *services.RouterQueryService
	audit   *services.AuditService
}

// NewRouterHandler creates a router handler.
func NewRouterHandler(routers *services.RouterService, jobs *services.RouterPollJobService,
	queries *services.RouterQueryService, audit *services.AuditService) *RouterHandler {
	return &RouterHandler{routers: routers, jobs: jobs, queries: queries, audit: audit}
}

// List returns every router with its BGP tally.
func (h *RouterHandler) List(c *gin.Context) {
	routers, err := h.routers.List()
	if err != nil {
		respondRouterError(c, err)
		return
	}
	summary, err := h.queries.BGPSummary()
	if err != nil {
		respondRouterError(c, err)
		return
	}
	out := make([]RouterResponse, len(routers))
	for i := range routers {
		tally := summary[routers[i].ID]
		out[i] = RouterResponse{MikrotikRouter: routers[i], BGPEstablished: tally.Established, BGPTotal: tally.Total}
	}
	c.JSON(http.StatusOK, out)
}

// Create adds a router.
func (h *RouterHandler) Create(c *gin.Context) {
	var req CreateRouterRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.Create(req.toInput())
	if err != nil {
		respondRouterError(c, err)
		return
	}
	h.record(c, "create", router.ID, map[string]interface{}{"name": router.Name, "host": router.Host})
	c.JSON(http.StatusCreated, RouterResponse{MikrotikRouter: *router})
}

// Update changes a router.
func (h *RouterHandler) Update(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req UpdateRouterRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.Update(id, req.toUpdate())
	if err != nil {
		respondRouterError(c, err)
		return
	}
	// Which fields changed, never the secret values themselves.
	h.record(c, "update", id, map[string]interface{}{"name": router.Name, "host": router.Host,
		"community_changed": req.SNMPCommunity != nil && *req.SNMPCommunity != "",
		"password_changed":  req.APIPassword != nil && *req.APIPassword != ""})
	c.JSON(http.StatusOK, RouterResponse{MikrotikRouter: *router})
}

// Delete removes a router.
func (h *RouterHandler) Delete(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	if err := h.routers.Delete(id); err != nil {
		respondRouterError(c, err)
		return
	}
	h.record(c, "delete", id, nil)
	c.Status(http.StatusNoContent)
}

// TestConnection probes SNMP and REST and returns the certificate fingerprint.
func (h *RouterHandler) TestConnection(c *gin.Context) {
	var req TestRouterConnectionRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	c.JSON(http.StatusOK, h.routers.TestConnection(req.toInput()))
}

// Refresh makes discovery due now. It answers before discovery runs.
func (h *RouterHandler) Refresh(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	if _, err := h.routers.Get(id); err != nil {
		respondRouterError(c, err)
		return
	}
	if err := h.jobs.RunNow(id, models.RouterPollKindDiscovery); err != nil {
		respondRouterError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}

// TrustCertificate pins the certificate the router now presents and resumes
// REST polling.
func (h *RouterHandler) TrustCertificate(c *gin.Context) {
	id, ok := routerParam(c, "id")
	if !ok {
		return
	}
	var req TrustCertificateRequest
	if !bindRouterJSON(c, &req) {
		return
	}
	router, err := h.routers.TrustCertificate(id, req.Fingerprint)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	if err := h.jobs.RunNow(id, models.RouterPollKindBGP, models.RouterPollKindDiscovery); err != nil {
		respondRouterError(c, err)
		return
	}
	h.record(c, "trust_certificate", id, map[string]interface{}{"fingerprint": router.TLSFingerprint})
	c.JSON(http.StatusOK, RouterResponse{MikrotikRouter: *router})
}

func (h *RouterHandler) record(c *gin.Context, action string, id uuid.UUID, values map[string]interface{}) {
	if h.audit == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.audit.Log(actorID, action, routerAuditResource, id, nil, values, c.ClientIP(), c.Request.UserAgent())
}

func routerParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid " + name, Code: "INVALID_ID"})
		return uuid.Nil, false
	}
	return id, true
}

func bindRouterJSON(c *gin.Context, out interface{}) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return false
	}
	return true
}

func respondRouterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_ROUTER"})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Router not found", Code: "NOT_FOUND"})
	case errors.Is(err, services.ErrFingerprintMismatch):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "FINGERPRINT_MISMATCH"})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Router request failed", Code: "ROUTER_FAILED"})
	}
}
```

- [ ] **Step 5: Handler baca**

`backend/internal/api/router_handler_reads.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxRouterRange is the longest range a graph or event list may ask for: the
// hourly rollup's year of retention, with room for the month it started in.
const maxRouterRange = 400 * 24 * time.Hour

// Interfaces lists a router's interfaces.
func (h *RouterHandler) Interfaces(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	ifaces, err := h.queries.ListInterfaces(id)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, ifaces)
}

// Sessions lists a router's BGP sessions.
func (h *RouterHandler) Sessions(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	sessions, err := h.queries.ListSessions(id)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessions)
}

// Events lists a router's BGP events in a range.
func (h *RouterHandler) Events(c *gin.Context) {
	id, ok := h.existingRouter(c)
	if !ok {
		return
	}
	from, to, ok := routerTimeRange(c)
	if !ok {
		return
	}
	events, err := h.queries.ListEvents(id, from, to)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, events)
}

// Traffic returns one interface's traffic in a range.
func (h *RouterHandler) Traffic(c *gin.Context) {
	routerID, ok := routerParam(c, "id")
	if !ok {
		return
	}
	interfaceID, ok := routerParam(c, "ifId")
	if !ok {
		return
	}
	from, to, ok := routerTimeRange(c)
	if !ok {
		return
	}
	points, err := h.queries.InterfaceTraffic(routerID, interfaceID, from, to)
	if err != nil {
		respondRouterError(c, err)
		return
	}
	c.JSON(http.StatusOK, points)
}

func (h *RouterHandler) existingRouter(c *gin.Context) (uuid.UUID, bool) {
	id, ok := routerParam(c, "id")
	if !ok {
		return uuid.Nil, false
	}
	if _, err := h.routers.Get(id); err != nil {
		respondRouterError(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func routerTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
	from, errFrom := time.Parse(time.RFC3339, c.Query("from"))
	to, errTo := time.Parse(time.RFC3339, c.Query("to"))
	if errFrom != nil || errTo != nil || !to.After(from) || to.Sub(from) > maxRouterRange {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "from and to must be RFC3339, with to after from and at most 400 days apart",
			Code:  "INVALID_RANGE",
		})
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}
```

- [ ] **Step 6: Route dan wiring**

`backend/internal/api/router_routes_mikrotik.go`:

```go
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
)

// registerMikrotikRoutes covers the MikroTik border routers. Reads are open to
// every role; adding, deleting and trusting a certificate are admin decisions.
func (h *handlers) registerMikrotikRoutes(api *gin.RouterGroup, authenticated gin.HandlerFunc) {
	admin := middleware.RequireRole(models.UserRoleAdmin)
	operator := middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician)

	routers := api.Group("/routers")
	routers.Use(authenticated)
	{
		routers.GET("", h.routerHandler.List)
		routers.POST("", admin, h.routerHandler.Create)
		routers.POST("/test-connection", admin, h.routerHandler.TestConnection)
		routers.PUT("/:id", operator, h.routerHandler.Update)
		routers.DELETE("/:id", admin, h.routerHandler.Delete)
		routers.POST("/:id/refresh", operator, h.routerHandler.Refresh)
		routers.POST("/:id/trust-certificate", admin, h.routerHandler.TrustCertificate)
		routers.GET("/:id/interfaces", h.routerHandler.Interfaces)
		routers.GET("/:id/interfaces/:ifId/traffic", h.routerHandler.Traffic)
		routers.GET("/:id/bgp/sessions", h.routerHandler.Sessions)
		routers.GET("/:id/bgp/events", h.routerHandler.Events)
	}
}
```

Di `backend/internal/api/router.go`, setelah `h.registerVPNRoutes(api, authenticated)`:

```go
	h.registerMikrotikRoutes(api, authenticated)
```

Di `backend/internal/api/router_handlers.go`, tambahkan field ke struct `handlers`:

```go
	mappingHandler         *MappingHandler
	routerHandler          *RouterHandler
```

dan di literal `&handlers{`:

```go
		mappingHandler:         NewMappingHandler(services.NewMappingService(db)),
		routerHandler: NewRouterHandler(services.NewRouterService(db, cfg.EncryptionKey),
			services.NewRouterPollJobService(db), services.NewRouterQueryService(db), auditService),
```

- [ ] **Step 7: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -run 'Router' -v -race && go test ./internal/api/ -count=1`
Expected: PASS semua, termasuk test API yang sudah ada.

Kalau `TestRefreshMakesDiscoveryDue` gagal karena SQLite menyimpan `"2099-01-01"` sebagai teks yang tidak terbaca balik sebagai waktu, ganti `Update("due_at", "2099-01-01")` dengan `Update("due_at", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))` dan tambahkan import `time`.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/api/router_dto.go backend/internal/api/router_handler.go \
  backend/internal/api/router_handler_reads.go backend/internal/api/router_routes_mikrotik.go \
  backend/internal/api/router_handler_test.go backend/internal/api/router.go \
  backend/internal/api/router_handlers.go backend/internal/api/router_rbac_test.go
git commit -m "feat(api): /api/v1/routers for MikroTik monitoring"
```

---

### Task 12: Frontend — entity, repository, hooks, helper

**Files:**
- Create: `frontend/src/domain/entities/MikrotikRouter.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/infrastructure/repositories/MikrotikRouterRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Create: `frontend/src/application/hooks/useMikrotikRouters.ts`
- Create: `frontend/src/presentation/components/routers/routerTraffic.ts`, `routerTraffic.test.ts`

**Interfaces:**
- Consumes: endpoint Task 11 (respons di-camelCase oleh `apiClient`).
- Produces: tipe `MikrotikRouter`, `RouterInterface`, `RouterBgpSession`, `RouterBgpEvent`, `RouterTrafficPoint`, `CreateRouterDto`, `UpdateRouterDto`, `TestRouterConnectionDto`, `RouterConnectionTestResult`, `TimeRange`; hooks `useMikrotikRouters`, `useRouterInterfaces`, `useRouterBgpSessions`, `useRouterBgpEvents`, `useInterfaceTraffic`, `useInterfacesTraffic`, `useCreateMikrotikRouter`, `useUpdateMikrotikRouter`, `useDeleteMikrotikRouter`, `useTestRouterConnection`, `useRefreshRouter`, `useTrustRouterCertificate`; helper `interfaceLabel`, `isUpstream`, `directionLabels`, `groupSessionsByInterface`, `groupInterfacesByParent`, `filterInterfaces`, `stackSeries`, `interfaceSeries`, `formatBps`, `formatUptime`, `memoryUsedPercent`.

- [ ] **Step 1: Tulis test helper yang gagal**

`frontend/src/presentation/components/routers/routerTraffic.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type {
  RouterBgpSession,
  RouterInterface,
  RouterTrafficPoint,
} from "@/domain/entities";
import {
  directionLabels,
  filterInterfaces,
  formatBps,
  formatUptime,
  groupInterfacesByParent,
  groupSessionsByInterface,
  interfaceLabel,
  interfaceSeries,
  isUpstream,
  memoryUsedPercent,
  stackSeries,
} from "./routerTraffic";

const iface = (over: Partial<RouterInterface>): RouterInterface => ({
  id: "if",
  routerId: "r",
  name: "ether1",
  comment: "",
  type: "ether",
  vlanId: null,
  parentName: null,
  ifIndex: null,
  speedMbps: null,
  disabled: false,
  operStatus: "up",
  addresses: "",
  lastInBps: null,
  lastOutBps: null,
  lastPolledAt: null,
  ...over,
});

const session = (over: Partial<RouterBgpSession>): RouterBgpSession => ({
  id: "s",
  routerId: "r",
  name: "peer",
  remoteAddress: "192.0.2.1",
  remoteAs: 64500,
  localAddress: "192.0.2.0",
  state: "established",
  uptimeSeconds: 0,
  prefixCount: 0,
  lastStartedAt: null,
  lastStoppedAt: null,
  interfaceId: null,
  ...over,
});

describe("interfaceLabel", () => {
  it("prefers the comment, which is what operators named the link", () => {
    expect(
      interfaceLabel(iface({ name: "vlan-ix", comment: "CDN IX" })),
    ).toBe("CDN IX");
    expect(interfaceLabel(iface({ name: "vlan-ix" }))).toBe("vlan-ix");
  });
});

describe("isUpstream and directionLabels", () => {
  it("calls an interface carrying a BGP session an upstream", () => {
    const ix = iface({ id: "ix" });
    const pop = iface({ id: "pop" });
    const sessions = [session({ interfaceId: "ix" })];
    expect(isUpstream(ix, sessions)).toBe(true);
    expect(isUpstream(pop, sessions)).toBe(false);
  });

  it("reads an upstream's rx as download and a POP's tx as towards subscribers", () => {
    expect(directionLabels(true)).toEqual({ in: "Download", out: "Upload" });
    expect(directionLabels(false)).toEqual({
      in: "Dari pelanggan",
      out: "Ke pelanggan",
    });
  });
});

describe("groupSessionsByInterface", () => {
  it("puts every route-server session under its shared VLAN and unmatched ones last", () => {
    const vlan = iface({ id: "rs", comment: "IX route servers" });
    const groups = groupSessionsByInterface(
      [
        session({ id: "1", interfaceId: "rs" }),
        session({ id: "2", interfaceId: null }),
        session({ id: "3", interfaceId: "rs" }),
      ],
      [vlan],
    );
    expect(groups).toHaveLength(2);
    expect(groups[0].iface?.id).toBe("rs");
    expect(groups[0].sessions.map((s) => s.id)).toEqual(["1", "3"]);
    expect(groups[1].iface).toBeNull();
  });
});

describe("groupInterfacesByParent", () => {
  it("lists VLANs under their physical port, and the port itself with them", () => {
    const groups = groupInterfacesByParent([
      iface({ id: "a", name: "vlan-b", parentName: "sfp1" }),
      iface({ id: "b", name: "sfp1" }),
      iface({ id: "c", name: "vlan-a", parentName: "sfp1" }),
      iface({ id: "d", name: "ether2" }),
    ]);
    expect(groups.map((g) => g.parent)).toEqual(["ether2", "sfp1"]);
    expect(groups[1].interfaces.map((i) => i.name)).toEqual([
      "sfp1",
      "vlan-a",
      "vlan-b",
    ]);
  });
});

describe("filterInterfaces", () => {
  it("matches name or comment, ignoring case", () => {
    const list = [
      iface({ id: "1", name: "vlan-1113", comment: "Cariu Via Fibernet" }),
      iface({ id: "2", name: "vlan-1119", comment: "Depok" }),
    ];
    expect(filterInterfaces(list, "cariu").map((i) => i.id)).toEqual(["1"]);
    expect(filterInterfaces(list, "1119").map((i) => i.id)).toEqual(["2"]);
    expect(filterInterfaces(list, "  ")).toHaveLength(2);
  });
});

describe("stackSeries", () => {
  it("lines each upstream up by timestamp for a stacked chart", () => {
    const a: RouterTrafficPoint[] = [
      { time: "2026-10-12T01:00:00Z", inBps: 10, outBps: 1 },
      { time: "2026-10-12T01:01:00Z", inBps: 20, outBps: 2 },
    ];
    const b: RouterTrafficPoint[] = [
      { time: "2026-10-12T01:01:00Z", inBps: 5, outBps: 0 },
    ];
    expect(stackSeries({ Transit: a, IX: b }, "inBps")).toEqual([
      { time: "2026-10-12T01:00:00Z", Transit: 10 },
      { time: "2026-10-12T01:01:00Z", Transit: 20, IX: 5 },
    ]);
  });
});

describe("interfaceSeries", () => {
  it("names the two directions with the labels it is given", () => {
    expect(
      interfaceSeries([{ time: "t", inBps: 3, outBps: 4 }], {
        in: "Download",
        out: "Upload",
      }),
    ).toEqual([{ time: "t", Download: 3, Upload: 4 }]);
  });
});

describe("formatting", () => {
  it("formats bits per second in the unit that keeps them readable", () => {
    expect(formatBps(642_100_000)).toBe("642.10 Mbps");
    expect(formatBps(1_500_000_000)).toBe("1.50 Gbps");
    expect(formatBps(null)).toBe("—");
  });

  it("formats an uptime in days, hours and minutes", () => {
    expect(formatUptime(38 * 3600 + 52 * 60 + 22)).toBe("1h 14j 52m");
    expect(formatUptime(15 * 3600 + 53 * 60)).toBe("15j 53m");
    expect(formatUptime(0)).toBe("—");
  });

  it("computes memory used from total and free", () => {
    expect(memoryUsedPercent(16, 12)).toBe(25);
    expect(memoryUsedPercent(null, 12)).toBeNull();
    expect(memoryUsedPercent(0, 0)).toBeNull();
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/routerTraffic.test.ts`
Expected: FAIL, modul `./routerTraffic` tidak ditemukan.

- [ ] **Step 3: Entity**

`frontend/src/domain/entities/MikrotikRouter.ts`:

```ts
export type RouterStatus = "unknown" | "up" | "down";
export type RouterApiStatus =
  | "unknown"
  | "ok"
  | "error"
  | "certificate_changed";
export type BgpState = "established" | "down";

export interface MikrotikRouter {
  id: string;
  name: string;
  host: string;
  siteId: string | null;
  snmpPort: number;
  apiUsername: string;
  apiPort: number;
  enabled: boolean;
  tlsFingerprint: string;
  presentedFingerprint: string | null;
  status: RouterStatus;
  apiStatus: RouterApiStatus;
  apiError: string | null;
  cpuLoad: number | null;
  memoryTotal: number | null;
  memoryFree: number | null;
  uptimeSeconds: number | null;
  rosVersion: string | null;
  boardName: string | null;
  lastSeenAt: string | null;
  bgpEstablished: number;
  bgpTotal: number;
}

export interface RouterInterface {
  id: string;
  routerId: string;
  name: string;
  comment: string;
  type: string;
  vlanId: number | null;
  parentName: string | null;
  ifIndex: number | null;
  speedMbps: number | null;
  disabled: boolean;
  operStatus: string;
  addresses: string;
  lastInBps: number | null;
  lastOutBps: number | null;
  lastPolledAt: string | null;
}

export interface RouterBgpSession {
  id: string;
  routerId: string;
  name: string;
  remoteAddress: string;
  remoteAs: number;
  localAddress: string;
  state: BgpState;
  uptimeSeconds: number;
  prefixCount: number;
  lastStartedAt: string | null;
  lastStoppedAt: string | null;
  interfaceId: string | null;
}

export interface RouterBgpEvent {
  id: string;
  routerId: string;
  sessionName: string;
  fromState: BgpState;
  toState: BgpState;
  occurredAt: string;
}

export interface RouterTrafficPoint {
  time: string;
  inBps: number;
  outBps: number;
}

export interface TimeRange {
  from: string;
  to: string;
}

export interface TestRouterConnectionDto {
  host: string;
  snmpCommunity: string;
  snmpPort?: number;
  apiUsername: string;
  apiPassword: string;
  apiPort?: number;
}

export interface CreateRouterDto extends TestRouterConnectionDto {
  name: string;
  tlsFingerprint: string;
}

export interface UpdateRouterDto {
  name?: string;
  host?: string;
  snmpCommunity?: string;
  snmpPort?: number;
  apiUsername?: string;
  apiPassword?: string;
  apiPort?: number;
  enabled?: boolean;
}

export interface RouterConnectionTestResult {
  snmpError?: string;
  apiError?: string;
  fingerprint?: string;
  boardName?: string;
  version?: string;
}
```

Di `frontend/src/domain/entities/index.ts`, tambahkan baris terakhir:

```ts
export * from "./MikrotikRouter";
```

- [ ] **Step 4: Endpoint, repository, hooks**

Di `frontend/src/infrastructure/http/endpoints.ts`, setelah blok `// WireGuard VPN`:

```ts
  // MikroTik border routers
  MIKROTIK_ROUTERS: "/api/v1/routers",
  MIKROTIK_ROUTER_BY_ID: (id: string) => `/api/v1/routers/${id}`,
  MIKROTIK_ROUTER_TEST: "/api/v1/routers/test-connection",
  MIKROTIK_ROUTER_REFRESH: (id: string) => `/api/v1/routers/${id}/refresh`,
  MIKROTIK_ROUTER_TRUST: (id: string) =>
    `/api/v1/routers/${id}/trust-certificate`,
  MIKROTIK_ROUTER_INTERFACES: (id: string) =>
    `/api/v1/routers/${id}/interfaces`,
  MIKROTIK_ROUTER_TRAFFIC: (id: string, interfaceId: string) =>
    `/api/v1/routers/${id}/interfaces/${interfaceId}/traffic`,
  MIKROTIK_ROUTER_SESSIONS: (id: string) => `/api/v1/routers/${id}/bgp/sessions`,
  MIKROTIK_ROUTER_EVENTS: (id: string) => `/api/v1/routers/${id}/bgp/events`,
```

`frontend/src/infrastructure/repositories/MikrotikRouterRepository.ts`:

```ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  CreateRouterDto,
  MikrotikRouter,
  RouterBgpEvent,
  RouterBgpSession,
  RouterConnectionTestResult,
  RouterInterface,
  RouterTrafficPoint,
  TestRouterConnectionDto,
  TimeRange,
  UpdateRouterDto,
} from "@/domain/entities";

export class MikrotikRouterRepository {
  async list(): Promise<MikrotikRouter[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTERS)).data;
  }

  async create(data: CreateRouterDto): Promise<MikrotikRouter> {
    return (await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTERS, data)).data;
  }

  async update(id: string, data: UpdateRouterDto): Promise<MikrotikRouter> {
    return (await apiClient.put(API_ENDPOINTS.MIKROTIK_ROUTER_BY_ID(id), data))
      .data;
  }

  async remove(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MIKROTIK_ROUTER_BY_ID(id));
  }

  async testConnection(
    data: TestRouterConnectionDto,
  ): Promise<RouterConnectionTestResult> {
    return (await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_TEST, data))
      .data;
  }

  async refresh(id: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_REFRESH(id));
  }

  async trustCertificate(
    id: string,
    fingerprint: string,
  ): Promise<MikrotikRouter> {
    return (
      await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_TRUST(id), {
        fingerprint,
      })
    ).data;
  }

  async interfaces(id: string): Promise<RouterInterface[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_INTERFACES(id)))
      .data;
  }

  async sessions(id: string): Promise<RouterBgpSession[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_SESSIONS(id)))
      .data;
  }

  async events(id: string, range: TimeRange): Promise<RouterBgpEvent[]> {
    return (
      await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_EVENTS(id), {
        params: range,
      })
    ).data;
  }

  async traffic(
    id: string,
    interfaceId: string,
    range: TimeRange,
  ): Promise<RouterTrafficPoint[]> {
    return (
      await apiClient.get(
        API_ENDPOINTS.MIKROTIK_ROUTER_TRAFFIC(id, interfaceId),
        {
          params: range,
        },
      )
    ).data;
  }
}
```

Di `frontend/src/infrastructure/repositories/index.ts`, tambahkan:

```ts
export * from "./MikrotikRouterRepository";
```

`frontend/src/application/hooks/useMikrotikRouters.ts`:

```ts
import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { MikrotikRouterRepository } from "@/infrastructure/repositories";
import type {
  CreateRouterDto,
  RouterInterface,
  RouterTrafficPoint,
  TestRouterConnectionDto,
  TimeRange,
  UpdateRouterDto,
} from "@/domain/entities";

const repository = new MikrotikRouterRepository();

// Matches the worker's one-minute interface and BGP tiers: polling faster only
// re-reads the same row.
const POLL_INTERVAL_MS = 60_000;

const routersKey = ["mikrotik", "routers"] as const;

export function useMikrotikRouters() {
  return useQuery({
    queryKey: routersKey,
    queryFn: () => repository.list(),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterInterfaces(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "interfaces"],
    queryFn: () => repository.interfaces(routerId),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterBgpSessions(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "sessions"],
    queryFn: () => repository.sessions(routerId),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterBgpEvents(routerId: string, range: TimeRange) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "events", range.from, range.to],
    queryFn: () => repository.events(routerId, range),
  });
}

export function useInterfaceTraffic(
  routerId: string,
  interfaceId: string,
  range: TimeRange,
) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "traffic", interfaceId, range.from, range.to],
    queryFn: () => repository.traffic(routerId, interfaceId, range),
  });
}

/** Traffic of several interfaces at once, keyed by the label each is drawn under. */
export function useInterfacesTraffic(
  routerId: string,
  interfaces: { iface: RouterInterface; label: string }[],
  range: TimeRange,
): Record<string, RouterTrafficPoint[]> {
  const results = useQueries({
    queries: interfaces.map(({ iface }) => ({
      queryKey: ["mikrotik", routerId, "traffic", iface.id, range.from, range.to],
      queryFn: () => repository.traffic(routerId, iface.id, range),
    })),
  });
  const series: Record<string, RouterTrafficPoint[]> = {};
  interfaces.forEach(({ label }, i) => {
    series[label] = results[i]?.data ?? [];
  });
  return series;
}

function useRouterMutation<TArgs, TResult>(
  mutationFn: (args: TArgs) => Promise<TResult>,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mikrotik"] });
    },
  });
}

export function useCreateMikrotikRouter() {
  return useRouterMutation((data: CreateRouterDto) => repository.create(data));
}

export function useUpdateMikrotikRouter() {
  return useRouterMutation(
    ({ id, data }: { id: string; data: UpdateRouterDto }) =>
      repository.update(id, data),
  );
}

export function useDeleteMikrotikRouter() {
  return useRouterMutation((id: string) => repository.remove(id));
}

export function useRefreshRouter() {
  return useRouterMutation((id: string) => repository.refresh(id));
}

export function useTrustRouterCertificate() {
  return useRouterMutation(
    ({ id, fingerprint }: { id: string; fingerprint: string }) =>
      repository.trustCertificate(id, fingerprint),
  );
}

export function useTestRouterConnection() {
  return useMutation({
    mutationFn: (data: TestRouterConnectionDto) =>
      repository.testConnection(data),
  });
}
```

- [ ] **Step 5: Helper**

`frontend/src/presentation/components/routers/routerTraffic.ts`:

```ts
import type {
  RouterBgpSession,
  RouterInterface,
  RouterTrafficPoint,
} from "@/domain/entities";
import { formatSpeed } from "../trafficFormat";

const BITS_PER_MEGABIT = 1_000_000;
const SECONDS_PER_DAY = 86_400;
const SECONDS_PER_HOUR = 3_600;
const SECONDS_PER_MINUTE = 60;

export interface DirectionLabels {
  in: string;
  out: string;
}

export interface SessionGroup {
  iface: RouterInterface | null;
  sessions: RouterBgpSession[];
}

export interface InterfaceGroup {
  parent: string;
  interfaces: RouterInterface[];
}

export type ChartRow = Record<string, string | number>;

/** The comment is what operators named the link; the name is the fallback. */
export function interfaceLabel(iface: RouterInterface): string {
  return iface.comment.trim() || iface.name;
}

/** An interface carrying a BGP session is where traffic enters and leaves. */
export function isUpstream(
  iface: RouterInterface,
  sessions: RouterBgpSession[],
): boolean {
  return sessions.some((s) => s.interfaceId === iface.id);
}

/**
 * Names rx and tx from the side that matters. On an upstream the router
 * receives what subscribers download; on a POP VLAN the router transmits it,
 * so the same rx/tx would read backwards there.
 */
export function directionLabels(upstream: boolean): DirectionLabels {
  return upstream
    ? { in: "Download", out: "Upload" }
    : { in: "Dari pelanggan", out: "Ke pelanggan" };
}

/** Sessions under the interface they run over; unmatched ones in a last group. */
export function groupSessionsByInterface(
  sessions: RouterBgpSession[],
  interfaces: RouterInterface[],
): SessionGroup[] {
  const byId = new Map(interfaces.map((i) => [i.id, i]));
  const groups = new Map<string, SessionGroup>();
  const unmatched: RouterBgpSession[] = [];
  for (const s of sessions) {
    const iface = s.interfaceId ? byId.get(s.interfaceId) : undefined;
    if (!iface) {
      unmatched.push(s);
      continue;
    }
    const group = groups.get(iface.id) ?? { iface, sessions: [] };
    group.sessions.push(s);
    groups.set(iface.id, group);
  }
  const result = [...groups.values()];
  if (unmatched.length > 0) result.push({ iface: null, sessions: unmatched });
  return result;
}

/** VLANs under their physical port, the port listed first in its own group. */
export function groupInterfacesByParent(
  interfaces: RouterInterface[],
): InterfaceGroup[] {
  const groups = new Map<string, RouterInterface[]>();
  for (const iface of interfaces) {
    const parent = iface.parentName ?? iface.name;
    groups.set(parent, [...(groups.get(parent) ?? []), iface]);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([parent, list]) => ({
      parent,
      interfaces: [...list].sort((a, b) => {
        if (a.name === parent) return -1;
        if (b.name === parent) return 1;
        return a.name.localeCompare(b.name);
      }),
    }));
}

export function filterInterfaces(
  interfaces: RouterInterface[],
  query: string,
): RouterInterface[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return interfaces;
  return interfaces.filter(
    (i) =>
      i.name.toLowerCase().includes(needle) ||
      i.comment.toLowerCase().includes(needle),
  );
}

/** One row per timestamp, one column per series, for a stacked area chart. */
export function stackSeries(
  series: Record<string, RouterTrafficPoint[]>,
  field: "inBps" | "outBps",
): ChartRow[] {
  const rows = new Map<string, ChartRow>();
  for (const [label, points] of Object.entries(series)) {
    for (const point of points) {
      const row = rows.get(point.time) ?? { time: point.time };
      row[label] = point[field];
      rows.set(point.time, row);
    }
  }
  return [...rows.values()].sort((a, b) =>
    String(a.time).localeCompare(String(b.time)),
  );
}

export function interfaceSeries(
  points: RouterTrafficPoint[],
  labels: DirectionLabels,
): ChartRow[] {
  return points.map((p) => ({
    time: p.time,
    [labels.in]: p.inBps,
    [labels.out]: p.outBps,
  }));
}

export function formatBps(bps: number | null | undefined): string {
  if (bps === null || bps === undefined || !Number.isFinite(bps)) return "—";
  return formatSpeed(bps / BITS_PER_MEGABIT);
}

/** Days, hours and minutes in Indonesian short form: "1h 14j 52m". */
export function formatUptime(seconds: number | null | undefined): string {
  if (!seconds) return "—";
  const days = Math.floor(seconds / SECONDS_PER_DAY);
  const hours = Math.floor((seconds % SECONDS_PER_DAY) / SECONDS_PER_HOUR);
  const minutes = Math.floor((seconds % SECONDS_PER_HOUR) / SECONDS_PER_MINUTE);
  const parts = [
    days ? `${days}h` : "",
    hours ? `${hours}j` : "",
    `${minutes}m`,
  ];
  return parts.filter(Boolean).join(" ");
}

export function memoryUsedPercent(
  total: number | null,
  free: number | null,
): number | null {
  if (!total || free === null) return null;
  return Math.round(((total - free) / total) * 100);
}
```

Periksa bahwa `formatSpeed(642.1)` menghasilkan `"642.10 Mbps"` dan `formatSpeed(1500)` menghasilkan `"1.50 Gbps"` (lihat `trafficFormat.ts`). Kalau format keluarannya berbeda, sesuaikan nilai yang diharapkan di test dengan keluaran `formatSpeed`, jangan buat formatter kedua.

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd frontend && npm test -- --run src/presentation/components/routers/routerTraffic.test.ts`
Expected: PASS.

Run: `cd frontend && npx tsc --noEmit -p .`
Expected: tanpa error.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/domain/entities/MikrotikRouter.ts frontend/src/domain/entities/index.ts \
  frontend/src/infrastructure/http/endpoints.ts \
  frontend/src/infrastructure/repositories/MikrotikRouterRepository.ts \
  frontend/src/infrastructure/repositories/index.ts \
  frontend/src/application/hooks/useMikrotikRouters.ts \
  frontend/src/presentation/components/routers/routerTraffic.ts \
  frontend/src/presentation/components/routers/routerTraffic.test.ts
git commit -m "feat(frontend): MikroTik router data layer and traffic helpers"
```

---

### Task 13: Frontend — halaman dan komponen

**Files:**
- Create di `frontend/src/presentation/components/routers/`: `RouterStatusTag.tsx`, `TrafficRangePicker.tsx`, `TrafficChart.tsx`, `BgpSessionTable.tsx`, `BgpEventList.tsx`, `UpstreamTab.tsx`, `InterfacesTab.tsx`, `InterfaceTrafficModal.tsx`, `RouterInfoTab.tsx`, `RouterFormModal.tsx`
- Create: `frontend/src/presentation/pages/RoutersPage.tsx`, `RouterDetailPage.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx` + `navigationRoutes.test.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`

**Interfaces:**
- Consumes: hooks dan helper Task 12, `useAuthStore` dari `@/application/stores`, `UserRole` dari `@/domain/entities`.

- [ ] **Step 1: Tulis test navigasi yang gagal**

Tambahkan ke `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`, di dalam `describe("buildNavigationRoutes", ...)`:

```ts
  it("lists MikroTik for every role, since reading routers is open to all", () => {
    expect(paths(UserRole.ADMIN)).toContain("/routers");
    expect(paths(UserRole.TECHNICIAN)).toContain("/routers");
    expect(paths(UserRole.VIEWER)).toContain("/routers");
  });
```

Run: `cd frontend && npm test -- --run src/presentation/components/layout/navigationRoutes.test.tsx`
Expected: FAIL.

- [ ] **Step 2: Menu dan route**

Di `navigationRoutes.tsx`, tambahkan `ClusterOutlined` ke import `@ant-design/icons`, lalu sisipkan setelah entri VPN:

```tsx
    { path: "/routers", name: "MikroTik", icon: <ClusterOutlined /> },
```

Di `frontend/src/presentation/routes/index.tsx`, tambahkan import di bawah `import VpnPage from "../pages/VpnPage";`:

```tsx
import RoutersPage from "../pages/RoutersPage";
```

dan lazy import di bawah `NetworkMapPage` (halaman detail memuat recharts, jadi ikut dimuat saat dibutuhkan seperti Graphs):

```tsx
const RouterDetailPage = lazy(() =>
  import("../pages/RouterDetailPage").then((m) => ({
    default: m.RouterDetailPage,
  })),
);
```

Di daftar children, setelah route `vpn`:

```tsx
          {
            path: "routers",
            element: <RoutersPage />,
          },
          {
            path: "routers/:id",
            element: (
              <Suspense
                fallback={
                  <div style={{ padding: 24, textAlign: "center" }}>
                    <Spin />
                  </div>
                }
              >
                <RouterDetailPage />
              </Suspense>
            ),
          },
```

Run: `cd frontend && npm test -- --run src/presentation/components/layout/navigationRoutes.test.tsx`
Expected: PASS (setelah halaman di langkah berikut ada, build juga lulus).

- [ ] **Step 3: Komponen kecil**

`RouterStatusTag.tsx`:

```tsx
import { Tag } from "antd";
import type { MikrotikRouter } from "@/domain/entities";

export function RouterStatusTag({ router }: { router: MikrotikRouter }) {
  if (!router.enabled) return <Tag>Nonaktif</Tag>;
  if (router.status === "down") return <Tag color="red">SNMP tidak menjawab</Tag>;
  if (router.apiStatus === "certificate_changed")
    return <Tag color="orange">Sertifikat berubah</Tag>;
  if (router.apiStatus === "error") return <Tag color="orange">REST error</Tag>;
  if (router.status === "up") return <Tag color="green">Up</Tag>;
  return <Tag>Belum terbaca</Tag>;
}
```

`TrafficRangePicker.tsx`:

```tsx
import { DatePicker } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import type { TimeRange } from "@/domain/entities";

const { RangePicker } = DatePicker;

export const DEFAULT_RANGE_HOURS = 6;

export function lastHours(hours: number): TimeRange {
  const to = dayjs();
  return {
    from: to.subtract(hours, "hour").toISOString(),
    to: to.toISOString(),
  };
}

const presets: { label: string; value: () => [Dayjs, Dayjs] }[] = [
  { label: "6 jam", value: () => [dayjs().subtract(6, "hour"), dayjs()] },
  { label: "24 jam", value: () => [dayjs().subtract(1, "day"), dayjs()] },
  { label: "7 hari", value: () => [dayjs().subtract(7, "day"), dayjs()] },
  { label: "30 hari", value: () => [dayjs().subtract(30, "day"), dayjs()] },
];

interface Props {
  value: TimeRange;
  onChange: (range: TimeRange) => void;
}

export function TrafficRangePicker({ value, onChange }: Props) {
  return (
    <RangePicker
      showTime
      presets={presets}
      value={[dayjs(value.from), dayjs(value.to)]}
      onChange={(dates) => {
        if (dates?.[0] && dates[1]) {
          onChange({
            from: dates[0].toISOString(),
            to: dates[1].toISOString(),
          });
        }
      }}
    />
  );
}
```

`TrafficChart.tsx`:

```tsx
import { Card, Empty } from "antd";
import dayjs from "dayjs";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatBps, type ChartRow } from "./routerTraffic";

const COLORS = ["#1677ff", "#52c41a", "#fa8c16", "#eb2f96", "#722ed1", "#13c2c2"];
const CHART_HEIGHT = 280;

interface Props {
  title: string;
  data: ChartRow[];
  series: string[];
  stacked?: boolean;
}

export function TrafficChart({ title, data, series, stacked = false }: Props) {
  return (
    <Card size="small" title={title}>
      {data.length === 0 ? (
        <Empty description="Belum ada data di rentang ini" />
      ) : (
        <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
          <AreaChart data={data}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis
              dataKey="time"
              minTickGap={40}
              tickFormatter={(t) => dayjs(t).format("DD/MM HH:mm")}
            />
            <YAxis width={90} tickFormatter={(v) => formatBps(Number(v))} />
            <Tooltip
              labelFormatter={(t) => dayjs(t).format("DD MMM YYYY HH:mm")}
              formatter={(v) => formatBps(Number(v))}
            />
            <Legend />
            {series.map((name, i) => (
              <Area
                key={name}
                type="monotone"
                dataKey={name}
                stackId={stacked ? "stack" : undefined}
                stroke={COLORS[i % COLORS.length]}
                fill={COLORS[i % COLORS.length]}
                fillOpacity={stacked ? 0.6 : 0.2}
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      )}
    </Card>
  );
}
```

`BgpSessionTable.tsx`:

```tsx
import { Card, Space, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { RouterBgpSession } from "@/domain/entities";
import { formatUptime, interfaceLabel, type SessionGroup } from "./routerTraffic";

const columns: ColumnsType<RouterBgpSession> = [
  { title: "Sesi", dataIndex: "name" },
  {
    title: "Peer",
    render: (_, s) => `${s.remoteAddress} (AS${s.remoteAs})`,
  },
  {
    title: "State",
    render: (_, s) =>
      s.state === "established" ? (
        <Tag color="green">Established</Tag>
      ) : (
        <Tag color="red">Down</Tag>
      ),
  },
  { title: "Uptime", render: (_, s) => formatUptime(s.uptimeSeconds) },
  { title: "Prefix", render: (_, s) => s.prefixCount.toLocaleString("id-ID") },
  {
    title: "Terakhir putus",
    render: (_, s) =>
      s.lastStoppedAt ? dayjs(s.lastStoppedAt).format("DD MMM HH:mm:ss") : "—",
  },
];

export function BgpSessionTable({ groups }: { groups: SessionGroup[] }) {
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {groups.map((group) => (
        <Card
          key={group.iface?.id ?? "unmatched"}
          size="small"
          title={
            group.iface ? interfaceLabel(group.iface) : "Interface tidak diketahui"
          }
        >
          <Table
            size="small"
            rowKey="id"
            pagination={false}
            columns={columns}
            dataSource={group.sessions}
          />
        </Card>
      ))}
    </Space>
  );
}
```

`BgpEventList.tsx`:

```tsx
import { Card, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { BgpState, RouterBgpEvent } from "@/domain/entities";

const stateTag = (state: BgpState) =>
  state === "established" ? (
    <Tag color="green">Established</Tag>
  ) : (
    <Tag color="red">Down</Tag>
  );

const columns: ColumnsType<RouterBgpEvent> = [
  {
    title: "Waktu",
    render: (_, e) => dayjs(e.occurredAt).format("DD MMM YYYY HH:mm:ss"),
  },
  { title: "Sesi", dataIndex: "sessionName" },
  {
    title: "Perubahan",
    render: (_, e) => (
      <>
        {stateTag(e.fromState)} → {stateTag(e.toState)}
      </>
    ),
  },
];

export function BgpEventList({ events }: { events: RouterBgpEvent[] }) {
  return (
    <Card size="small" title="Riwayat sesi BGP">
      <Table
        size="small"
        rowKey="id"
        columns={columns}
        dataSource={events}
        pagination={{ pageSize: 20 }}
      />
    </Card>
  );
}
```

- [ ] **Step 4: Tab Upstream**

`UpstreamTab.tsx`:

```tsx
import { useMemo, useState } from "react";
import { Space } from "antd";
import {
  useInterfacesTraffic,
  useRouterBgpEvents,
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { BgpEventList } from "./BgpEventList";
import { BgpSessionTable } from "./BgpSessionTable";
import { TrafficChart } from "./TrafficChart";
import {
  DEFAULT_RANGE_HOURS,
  TrafficRangePicker,
  lastHours,
} from "./TrafficRangePicker";
import {
  groupSessionsByInterface,
  interfaceLabel,
  isUpstream,
  stackSeries,
} from "./routerTraffic";

export function UpstreamTab({ routerId }: { routerId: string }) {
  const [range, setRange] = useState(() => lastHours(DEFAULT_RANGE_HOURS));
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const { data: events = [] } = useRouterBgpEvents(routerId, range);

  const upstreams = useMemo(
    () =>
      interfaces
        .filter((i) => isUpstream(i, sessions))
        .map((iface) => ({ iface, label: interfaceLabel(iface) })),
    [interfaces, sessions],
  );
  const traffic = useInterfacesTraffic(routerId, upstreams, range);
  const labels = upstreams.map((u) => u.label);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <TrafficRangePicker value={range} onChange={setRange} />
      <TrafficChart
        stacked
        title="Download per upstream"
        series={labels}
        data={stackSeries(traffic, "inBps")}
      />
      <TrafficChart
        stacked
        title="Upload per upstream"
        series={labels}
        data={stackSeries(traffic, "outBps")}
      />
      <BgpSessionTable groups={groupSessionsByInterface(sessions, interfaces)} />
      <BgpEventList events={events} />
    </Space>
  );
}
```

- [ ] **Step 5: Tab Interface & VLAN**

`InterfaceTrafficModal.tsx`:

```tsx
import { useState } from "react";
import { Modal, Space } from "antd";
import type { RouterInterface } from "@/domain/entities";
import { useInterfaceTraffic } from "@/application/hooks/useMikrotikRouters";
import { TrafficChart } from "./TrafficChart";
import {
  DEFAULT_RANGE_HOURS,
  TrafficRangePicker,
  lastHours,
} from "./TrafficRangePicker";
import {
  directionLabels,
  interfaceLabel,
  interfaceSeries,
} from "./routerTraffic";

interface Props {
  routerId: string;
  iface: RouterInterface;
  upstream: boolean;
  onClose: () => void;
}

export function InterfaceTrafficModal({ routerId, iface, upstream, onClose }: Props) {
  const [range, setRange] = useState(() => lastHours(DEFAULT_RANGE_HOURS));
  const { data: points = [] } = useInterfaceTraffic(routerId, iface.id, range);
  const labels = directionLabels(upstream);

  return (
    <Modal open width={960} footer={null} title={interfaceLabel(iface)} onCancel={onClose}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <TrafficRangePicker value={range} onChange={setRange} />
        <TrafficChart
          title={`${iface.name}${iface.vlanId ? ` · VLAN ${iface.vlanId}` : ""}`}
          series={[labels.in, labels.out]}
          data={interfaceSeries(points, labels)}
        />
      </Space>
    </Modal>
  );
}
```

`InterfacesTab.tsx`:

```tsx
import { useState } from "react";
import { Card, Input, Space, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { RouterInterface } from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { formatSpeed } from "../trafficFormat";
import { InterfaceTrafficModal } from "./InterfaceTrafficModal";
import {
  filterInterfaces,
  formatBps,
  groupInterfacesByParent,
  interfaceLabel,
  isUpstream,
} from "./routerTraffic";

const statusTag = (iface: RouterInterface) => {
  if (iface.disabled) return <Tag>Disabled</Tag>;
  if (iface.operStatus === "up") return <Tag color="green">Up</Tag>;
  if (iface.operStatus === "down") return <Tag color="red">Down</Tag>;
  return <Tag>—</Tag>;
};

const columns: ColumnsType<RouterInterface> = [
  { title: "Label", render: (_, i) => interfaceLabel(i) },
  { title: "Nama", dataIndex: "name" },
  { title: "VLAN", render: (_, i) => i.vlanId ?? "—" },
  { title: "Status", render: (_, i) => statusTag(i) },
  {
    title: "Kecepatan",
    render: (_, i) => (i.speedMbps ? formatSpeed(i.speedMbps) : "—"),
  },
  { title: "Masuk (rx)", render: (_, i) => formatBps(i.lastInBps) },
  { title: "Keluar (tx)", render: (_, i) => formatBps(i.lastOutBps) },
];

export function InterfacesTab({ routerId }: { routerId: string }) {
  const { data: interfaces = [], isLoading } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<RouterInterface | null>(null);
  const groups = groupInterfacesByParent(filterInterfaces(interfaces, search));

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Input.Search
        allowClear
        placeholder="Cari nama atau comment (mis. Cariu)"
        onChange={(e) => setSearch(e.target.value)}
        style={{ maxWidth: 360 }}
      />
      {groups.map((group) => (
        <Card key={group.parent} size="small" title={group.parent}>
          <Table
            size="small"
            rowKey="id"
            loading={isLoading}
            pagination={false}
            columns={columns}
            dataSource={group.interfaces}
            onRow={(iface) => ({
              onClick: () => setSelected(iface),
              style: { cursor: "pointer" },
            })}
          />
        </Card>
      ))}
      {selected && (
        <InterfaceTrafficModal
          routerId={routerId}
          iface={selected}
          upstream={isUpstream(selected, sessions)}
          onClose={() => setSelected(null)}
        />
      )}
    </Space>
  );
}
```

- [ ] **Step 6: Form tambah/ubah router**

`RouterFormModal.tsx`:

```tsx
import { useState } from "react";
import { Alert, Button, Form, Input, InputNumber, Modal, Space, message } from "antd";
import type { AxiosError } from "axios";
import type {
  MikrotikRouter,
  RouterConnectionTestResult,
} from "@/domain/entities";
import {
  useCreateMikrotikRouter,
  useTestRouterConnection,
  useUpdateMikrotikRouter,
} from "@/application/hooks/useMikrotikRouters";

const DEFAULT_SNMP_PORT = 161;
const DEFAULT_API_PORT = 8443;

interface FormValues {
  name: string;
  host: string;
  snmpCommunity: string;
  snmpPort: number;
  apiUsername: string;
  apiPassword: string;
  apiPort: number;
}

interface Props {
  router?: MikrotikRouter;
  onClose: () => void;
}

function errorText(error: unknown): string {
  return (
    (error as AxiosError<{ error?: string }>).response?.data?.error ??
    "Permintaan gagal"
  );
}

function TestResult({ result }: { result: RouterConnectionTestResult }) {
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Alert
        type={result.snmpError ? "error" : "success"}
        message={result.snmpError ? `SNMP: ${result.snmpError}` : "SNMP menjawab"}
      />
      <Alert
        type={result.apiError ? "error" : "success"}
        message={
          result.apiError
            ? `REST: ${result.apiError}`
            : `REST: ${result.boardName} · RouterOS ${result.version}`
        }
        description={
          result.fingerprint &&
          `Fingerprint sertifikat (cocokkan dengan router sebelum menyimpan): ${result.fingerprint}`
        }
      />
    </Space>
  );
}

export function RouterFormModal({ router, onClose }: Props) {
  const [form] = Form.useForm<FormValues>();
  const create = useCreateMikrotikRouter();
  const update = useUpdateMikrotikRouter();
  const test = useTestRouterConnection();
  const [tested, setTested] = useState<RouterConnectionTestResult | null>(null);
  const editing = router !== undefined;
  const testPassed = Boolean(
    tested?.fingerprint && !tested.apiError && !tested.snmpError,
  );

  const runTest = async () => {
    try {
      setTested(await test.mutateAsync(await form.validateFields()));
    } catch (error) {
      message.error(errorText(error));
    }
  };

  const save = async () => {
    try {
      const values = await form.validateFields();
      if (editing) {
        await update.mutateAsync({ id: router.id, data: values });
      } else if (tested?.fingerprint) {
        await create.mutateAsync({ ...values, tlsFingerprint: tested.fingerprint });
      }
      onClose();
    } catch (error) {
      message.error(errorText(error));
    }
  };

  const secretRule = editing ? [] : [{ required: true }];
  return (
    <Modal
      open
      title={editing ? "Ubah router" : "Tambah router MikroTik"}
      onCancel={onClose}
      footer={[
        !editing && (
          <Button key="test" onClick={runTest} loading={test.isPending}>
            Uji koneksi
          </Button>
        ),
        <Button
          key="save"
          type="primary"
          disabled={!editing && !testPassed}
          loading={create.isPending || update.isPending}
          onClick={save}
        >
          Simpan
        </Button>,
      ]}
    >
      <Form
        form={form}
        layout="vertical"
        onValuesChange={() => !editing && setTested(null)}
        initialValues={{
          name: router?.name,
          host: router?.host,
          snmpPort: router?.snmpPort ?? DEFAULT_SNMP_PORT,
          apiUsername: router?.apiUsername,
          apiPort: router?.apiPort ?? DEFAULT_API_PORT,
        }}
      >
        <Form.Item name="name" label="Nama" rules={[{ required: true, min: 2 }]}>
          <Input />
        </Form.Item>
        <Form.Item name="host" label="IP atau hostname" rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item
          name="snmpCommunity"
          label="SNMP community"
          rules={secretRule}
          extra={editing ? "Kosongkan untuk tetap memakai yang tersimpan" : undefined}
        >
          <Input.Password autoComplete="off" />
        </Form.Item>
        <Form.Item name="snmpPort" label="Port SNMP">
          <InputNumber min={1} max={65535} />
        </Form.Item>
        <Form.Item name="apiUsername" label="User REST (read-only)" rules={[{ required: true }]}>
          <Input autoComplete="off" />
        </Form.Item>
        <Form.Item
          name="apiPassword"
          label="Password REST"
          rules={secretRule}
          extra={editing ? "Kosongkan untuk tetap memakai yang tersimpan" : undefined}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="apiPort" label="Port REST (www-ssl)">
          <InputNumber min={1} max={65535} />
        </Form.Item>
      </Form>
      {tested && <TestResult result={tested} />}
    </Modal>
  );
}
```

- [ ] **Step 7: Tab Router**

`RouterInfoTab.tsx`:

```tsx
import { useState } from "react";
import { Alert, Button, Descriptions, Popconfirm, Space, message } from "antd";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import { UserRole, type MikrotikRouter } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import {
  useDeleteMikrotikRouter,
  useRefreshRouter,
  useTrustRouterCertificate,
} from "@/application/hooks/useMikrotikRouters";
import { RouterFormModal } from "./RouterFormModal";
import { RouterStatusTag } from "./RouterStatusTag";
import { formatUptime, memoryUsedPercent } from "./routerTraffic";

export function RouterInfoTab({ router }: { router: MikrotikRouter }) {
  const role = useAuthStore((state) => state.user?.role);
  const isAdmin = role === UserRole.ADMIN;
  const canOperate = isAdmin || role === UserRole.TECHNICIAN;
  const [editing, setEditing] = useState(false);
  const refresh = useRefreshRouter();
  const trust = useTrustRouterCertificate();
  const remove = useDeleteMikrotikRouter();
  const navigate = useNavigate();
  const memory = memoryUsedPercent(router.memoryTotal, router.memoryFree);
  const presented =
    router.apiStatus === "certificate_changed" ? router.presentedFingerprint : null;

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {presented && (
        <Alert
          type="warning"
          message="Sertifikat REST router berubah. Polling BGP berhenti sampai sertifikat baru diterima."
          description={`Fingerprint baru: ${presented}. Cocokkan dengan /certificate print di router sebelum menerima.`}
          action={
            isAdmin && (
              <Popconfirm
                title="Terima sertifikat baru?"
                onConfirm={() => trust.mutate({ id: router.id, fingerprint: presented })}
              >
                <Button size="small">Terima</Button>
              </Popconfirm>
            )
          }
        />
      )}
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label="Status"><RouterStatusTag router={router} /></Descriptions.Item>
        <Descriptions.Item label="Host">{router.host}</Descriptions.Item>
        <Descriptions.Item label="Board">{router.boardName ?? "—"}</Descriptions.Item>
        <Descriptions.Item label="RouterOS">{router.rosVersion ?? "—"}</Descriptions.Item>
        <Descriptions.Item label="CPU">{router.cpuLoad === null ? "—" : `${router.cpuLoad}%`}</Descriptions.Item>
        <Descriptions.Item label="RAM terpakai">{memory === null ? "—" : `${memory}%`}</Descriptions.Item>
        <Descriptions.Item label="Uptime">{formatUptime(router.uptimeSeconds)}</Descriptions.Item>
        <Descriptions.Item label="SNMP terakhir terbaca">
          {router.lastSeenAt ? dayjs(router.lastSeenAt).format("DD MMM YYYY HH:mm:ss") : "—"}
        </Descriptions.Item>
        <Descriptions.Item label="Error REST terakhir">{router.apiError ?? "—"}</Descriptions.Item>
        <Descriptions.Item label="Fingerprint sertifikat">{router.tlsFingerprint}</Descriptions.Item>
      </Descriptions>
      {canOperate && (
        <Space>
          <Button
            loading={refresh.isPending}
            onClick={() =>
              refresh.mutate(router.id, {
                onSuccess: () => message.success("Discovery dijadwalkan, hasilnya muncul dalam semenit"),
              })
            }
          >
            Refresh discovery
          </Button>
          <Button onClick={() => setEditing(true)}>Ubah</Button>
          {isAdmin && (
            <Popconfirm
              title="Hapus router ini beserta interface dan riwayat BGP-nya?"
              onConfirm={() => remove.mutate(router.id, { onSuccess: () => navigate("/routers") })}
            >
              <Button danger>Hapus</Button>
            </Popconfirm>
          )}
        </Space>
      )}
      {editing && <RouterFormModal router={router} onClose={() => setEditing(false)} />}
    </Space>
  );
}
```

Cek dulu bahwa `@/application/stores` mengekspor `useAuthStore` dengan `state.user?.role` bertipe `UserRole` (lihat `Dashboard.tsx:38`). Kalau bentuknya berbeda, ikuti bentuk yang dipakai `Dashboard.tsx`.

- [ ] **Step 8: Halaman**

`frontend/src/presentation/pages/RoutersPage.tsx`:

```tsx
import { useState } from "react";
import { Button, Card, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Link } from "react-router-dom";
import { UserRole, type MikrotikRouter } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { RouterFormModal } from "@/presentation/components/routers/RouterFormModal";
import { RouterStatusTag } from "@/presentation/components/routers/RouterStatusTag";

const columns: ColumnsType<MikrotikRouter> = [
  {
    title: "Nama",
    render: (_, r) => <Link to={`/routers/${r.id}`}>{r.name}</Link>,
  },
  { title: "Host", dataIndex: "host" },
  { title: "Status", render: (_, r) => <RouterStatusTag router={r} /> },
  {
    title: "BGP",
    render: (_, r) => `${r.bgpEstablished}/${r.bgpTotal} Established`,
  },
  { title: "CPU", render: (_, r) => (r.cpuLoad === null ? "—" : `${r.cpuLoad}%`) },
  { title: "RouterOS", render: (_, r) => r.rosVersion ?? "—" },
];

export default function RoutersPage() {
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const isAdmin = useAuthStore((state) => state.user?.role) === UserRole.ADMIN;
  const [adding, setAdding] = useState(false);

  return (
    <Card
      title="MikroTik"
      extra={
        isAdmin && (
          <Button type="primary" onClick={() => setAdding(true)}>
            Tambah router
          </Button>
        )
      }
    >
      <Table
        rowKey="id"
        loading={isLoading}
        columns={columns}
        dataSource={routers}
        pagination={false}
      />
      {adding && <RouterFormModal onClose={() => setAdding(false)} />}
    </Card>
  );
}
```

`frontend/src/presentation/pages/RouterDetailPage.tsx`:

```tsx
import { Card, Result, Spin, Tabs } from "antd";
import { useParams } from "react-router-dom";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { InterfacesTab } from "@/presentation/components/routers/InterfacesTab";
import { RouterInfoTab } from "@/presentation/components/routers/RouterInfoTab";
import { RouterStatusTag } from "@/presentation/components/routers/RouterStatusTag";
import { UpstreamTab } from "@/presentation/components/routers/UpstreamTab";

export function RouterDetailPage() {
  const { id = "" } = useParams();
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const router = routers.find((r) => r.id === id);

  if (isLoading) return <Spin />;
  if (!router) return <Result status="404" title="Router tidak ditemukan" />;

  return (
    <Card title={router.name} extra={<RouterStatusTag router={router} />}>
      <Tabs
        items={[
          {
            key: "upstream",
            label: "Upstream & BGP",
            children: <UpstreamTab routerId={router.id} />,
          },
          {
            key: "interfaces",
            label: "Interface & VLAN",
            children: <InterfacesTab routerId={router.id} />,
          },
          {
            key: "router",
            label: "Router",
            children: <RouterInfoTab router={router} />,
          },
        ]}
      />
    </Card>
  );
}
```

- [ ] **Step 9: Format, lint, test, build**

Run: `cd frontend && npm run format && npm run lint && npm test -- --run && npm run build`
Expected: lint tanpa error, semua test PASS, build sukses. Perbaiki error TypeScript yang muncul (misalnya tipe `formatter` recharts) di komponennya sendiri, tanpa `any` dan tanpa `// @ts-ignore`.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/presentation/components/routers/ frontend/src/presentation/pages/RoutersPage.tsx \
  frontend/src/presentation/pages/RouterDetailPage.tsx \
  frontend/src/presentation/components/layout/navigationRoutes.tsx \
  frontend/src/presentation/components/layout/navigationRoutes.test.tsx \
  frontend/src/presentation/routes/index.tsx
git commit -m "feat(frontend): MikroTik pages for upstream traffic, VLANs and BGP"
```

---

### Task 14: Dokumentasi, verifikasi penuh, dan verifikasi ke router

**Files:**
- Modify: `docs/operator_guide.md` (bagian baru)
- Modify: `CLAUDE.md` (deskripsi worker)

- [ ] **Step 1: Panduan operator**

Tambahkan bagian di akhir `docs/operator_guide.md`:

````markdown
## Memantau router MikroTik border

TikMan membaca router lewat SNMP (trafik interface dan VLAN) dan REST API
RouterOS v7 (status BGP, daftar interface, CPU/RAM). Aksesnya hanya baca.
Jalankan perintah berikut di router, ganti `IP_VPS`, `IP_ROUTER`, `PASSWORD`,
dan `COMMUNITY`:

```
/user group add name=tikman-ro policy=read,api,rest-api
/user add name=tikman group=tikman-ro password="PASSWORD" address=IP_VPS/32
/certificate add name=tikman-rest common-name=IP_ROUTER subject-alt-name=IP:IP_ROUTER days-valid=3650 key-usage=digital-signature,key-encipherment,tls-server
/certificate sign tikman-rest
/ip service set www-ssl certificate=tikman-rest port=8443 address=IP_VPS/32 disabled=no
/snmp community add name="COMMUNITY" addresses=IP_VPS/32 read-access=yes write-access=no
```

REST memakai port 8443 karena 443 biasanya dipakai `reverse-proxy` RouterOS 7.
Bila chain input punya rule drop, izinkan tcp/8443 dan udp/161 dari `IP_VPS`
di atasnya. Batasi juga `addresses` setiap community SNMP lain ke pemakainya:
community yang terbuka ke `::/0` bisa dibaca siapa saja dan dipakai untuk
serangan amplifikasi.

Di TikMan, buka **MikroTik → Tambah router**, isi data, lalu klik **Uji
koneksi**. Cocokkan fingerprint yang tampil dengan
`/certificate print detail where name=tikman-rest` di router sebelum menyimpan.
Kalau sertifikat router diganti, TikMan berhenti membaca BGP dan menampilkan
fingerprint baru di tab Router sampai admin menerimanya.
````

- [ ] **Step 2: CLAUDE.md**

Di `CLAUDE.md`, pada butir `cmd/worker/main.go`, setelah kalimat tentang `discovery` job OLT, tambahkan:

```markdown
  The same loop also claims `router_poll_jobs` for MikroTik border routers:
  `interfaces` (1 min, SNMP IF-MIB), `bgp` (1 min, RouterOS REST) and
  `discovery` (6 hours, REST inventory). Both queues are tried every pass.
```

- [ ] **Step 3: Verifikasi penuh**

```bash
cd backend && gofmt -s -l . && go vet ./... && go mod verify && go test ./... -race -count=1
cd backend && go build -o /dev/null ./cmd/api && go build -o /dev/null ./cmd/worker
cd frontend && npm run format:check && npm run lint && npm test -- --run && npm run build
```

Expected: `gofmt` tanpa output, semua lulus. Kalau test WireGuard di `internal/api` gagal 400 di Mac saat `ppp0` aktif, itu lingkungan lokal (memory *Mac ppp0 test failures*), bukan regresi. Buktikan dengan menjalankan ulang setelah `ppp0` turun, atau lihat hasil CI.

Cek ukuran file baru:

```bash
wc -l backend/internal/services/router_*.go backend/internal/api/router_*.go \
  backend/internal/mikrotik/*.go frontend/src/presentation/components/routers/*.tsx \
  frontend/src/presentation/components/routers/*.ts | sort -n | tail
```

Expected: tidak ada file non-test di atas 350 baris (kecuali `snmp_ifxtable.go` yang dikecualikan).

- [ ] **Step 4: Update graph**

Run: `graphify update .`

- [ ] **Step 5: Commit**

```bash
git add docs/operator_guide.md CLAUDE.md graphify-out
git commit -m "docs: MikroTik router setup and worker router queue"
```

- [ ] **Step 6: Verifikasi langsung ke router (setelah deploy, dengan operator)**

Langkah ini dijalankan bersama operator setelah deploy manual sesuai CLAUDE.md, dan setelah setup router (Step 1) selesai di router.

1. Route ada: `docker exec tikman-api wget -qO- http://localhost:8080/api/v1/routers` menjawab **401**, bukan 404.
2. Bentuk JSON REST cocok dengan `internal/mikrotik/types.go`. Dari VPS:
   `curl -sk -u tikman:PASSWORD https://IP_ROUTER:8443/rest/routing/bgp/session` dan `/rest/system/clock`.
   Pastikan field `established`, `last-started`, `last-stopped`, `prefix-count`, dan `gmt-offset` ada dengan nama itu. Kalau berbeda, perbaiki tag JSON dan tambahkan test dari output asli yang sudah disamarkan.
3. Tambah router lewat UI, lalu tunggu 2 menit. Tab Router menunjukkan CPU/RAM/uptime, tab Upstream menampilkan 5 Established dan 2 Down, dan RS1-NEW/RS2-NEW tidak menempel ke RS1/RS2.
4. Grafik upstream cocok dengan `/interface monitor-traffic VLAN-...` di router, dengan selisih wajar karena rata-rata per menit.
5. Setelah 2 hari, grafik 7 hari dan 30 hari terisi. Ini membuktikan pembacaan dari kedua rollup, yang tidak bisa diuji di SQLite.
````
