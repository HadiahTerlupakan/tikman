# Pemetaan Jaringan — Rencana Implementasi

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mengganti model plant TikMan (`ODC`/`ODCFeed`/`ODP`) dengan `mapping_nodes` + `mapping_edges`, lalu memberi peta alur kerja cepat: klik tombol jenis node, klik di peta, isi seperlunya, dan hubungkan dengan menarik kabel.

**Architecture:** Dua tabel menggantikan tiga. Node punya empat jenis (`server`, `odc`, `odp`, `ont`) dan hanya mewajibkan nama serta koordinat. Hubungan dinyatakan lewat edge dengan tujuh jenis kabel, termasuk kaskade ODP→ODP dan ODC→ODC yang model lama tidak bisa. Kapasitas slot divalidasi saat kabel dibuat. `ONT.ODPID` dan registrasi GPON ZTE dialihkan menunjuk `mapping_nodes`, bukan dihapus.

**Tech Stack:** Go 1.25 (gin, GORM, Postgres/TimescaleDB), React 18 + TypeScript (Ant Design 5, TanStack Query 5, Leaflet, Vitest).

**Spec:** `docs/superpowers/specs/2026-09-16-network-mapping-design.md`

## Global Constraints

- Empat jenis node: `server`, `odc`, `odp`, `ont`. Tujuh jenis kabel: `feeder`, `distribution`, `drop`, `odp_to_odp`, `odp_to_odp_ratio`, `odc_to_odc`, `odc_to_odc_ratio`.
- Node wajib punya `node_id`, `type`, `name`, `latitude`, `longitude`. Sisanya opsional — itu inti desain ini.
- `node_id` dan `edge_id` unik. Menyimpan yang sudah ada dijawab 409.
- Kapasitas dihitung **per jenis tujuan**, bukan total. `capacity` 0 berarti tanpa batas. Penolakan menyebut angkanya: `ODC "X" slots are full (8/8)`.
- Panjang kabel dihitung dari jalur yang digambar, bukan garis lurus.
- Tiap node dan kabel disimpan sendiri-sendiri. **Tidak ada operasi "sync" yang menghapus seluruh peta lalu menulis ulang** — panel aslinya begitu, dan itu membuat dua penyunting saling menimpa.
- `ONT.ODPID` menunjuk baris `mapping_nodes` bertipe `odp`; indeks unik `uq_onts_odp_port` tetap.
- Migrasi SQL bernomor `53`. AutoMigrate membuat tabel dari model; berkas SQL menambahkan yang tag model tidak bisa nyatakan.
- Berkas maksimal 350 baris, fungsi maksimal 50 baris, nesting maksimal 3. Komentar menjelaskan **kenapa**, bukan apa.
- Kode dan nama tes dalam bahasa Inggris; teks antarmuka dalam bahasa Indonesia.
- `gofmt -s -l .` kosong, `go vet ./...` bersih, `npm run lint`, `npm run format:check`, `npm run build` bersih.
- Tes lama tetap lulus kecuali ~30 tes WireGuard yang memang gagal di Mac ini karena `ppp0` memegang 10.0.0.0/8.
- Tes `OnPostgres` dan `internal/database` butuh `TEST_POSTGRES_DSN`. Di Mac ini: `docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test -p 5439:5432 timescale/timescaledb:latest-pg15`, lalu `TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable"`, lalu `docker stop tikman-test-pg`.
- Repo ini **publik**: tidak ada nama staf, nomor pelanggan, IP, atau kredensial asli di berkas mana pun, termasuk fixture.
- Kerja di branch `feat/network-mapping`. Merge, push, dan deploy hanya dengan izin user.
- Tiap commit diakhiri baris `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

## Peta berkas

**Backend**

| Berkas | Tanggung jawab |
|---|---|
| `backend/internal/models/mapping.go` | `MappingNode`, `MappingEdge`, konstanta jenis |
| `backend/migrations/53_network_mapping.sql` | CHECK, indeks unik, dan pemindahan ODC/ODP lama |
| `backend/internal/services/mapping_nodes.go` | CRUD node |
| `backend/internal/services/mapping_edges.go` | CRUD edge + aturan kapasitas |
| `backend/internal/api/mapping_dto.go` | bentuk permintaan dan jawaban |
| `backend/internal/api/mapping_handler.go` | handler node, edge, settings |

**Frontend**

| Berkas | Tanggung jawab |
|---|---|
| `frontend/src/domain/entities/Mapping.ts` | tipe node, edge, settings |
| `frontend/src/infrastructure/repositories/MappingRepository.ts` | pemanggilan API |
| `frontend/src/application/hooks/useMapping.ts` | hook TanStack Query |
| `frontend/src/presentation/pages/NetworkMapPage.tsx` | halaman peta |
| `frontend/src/presentation/components/netmap/MapToolbar.tsx` | tombol jenis node dan tarik kabel |
| `frontend/src/presentation/components/netmap/NodeFormModal.tsx` | formulir node |
| `frontend/src/presentation/components/netmap/CableDrawing.tsx` | menggambar kabel |
| `frontend/src/presentation/components/netmap/CountCards.tsx` | kartu jumlah |
| `frontend/src/presentation/components/netmap/NodeList.tsx` | tampilan daftar |

**Dihapus:** `backend/internal/models/distribution.go`, `services/distribution_*.go`, `services/route_store.go`, `api/distribution_*.go`, dan seluruh `frontend/src/presentation/components/map/`.

---

### Task 1: Model dan migrasi

**Files:**
- Create: `backend/internal/models/mapping.go`
- Create: `backend/migrations/53_network_mapping.sql`
- Modify: `backend/internal/models/models.go` (daftar AutoMigrate)
- Test: `backend/internal/database/migrations_mapping_postgres_test.go`

**Interfaces:**
- Produces: `models.MappingNode`, `models.MappingEdge`; `models.NodeType` dengan `NodeServer`/`NodeODC`/`NodeODP`/`NodeONT`; `models.FiberType` dengan `FiberFeeder`/`FiberDistribution`/`FiberDrop`/`FiberODPToODP`/`FiberODPToODPRatio`/`FiberODCToODC`/`FiberODCToODCRatio`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/database/migrations_mapping_postgres_test.go`:

```go
package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// A node id names a box in the field. Two boxes with one name would make every
// cable that points at it ambiguous.
func TestDatabaseRefusesTwoNodesWithOneNodeID(t *testing.T) {
	db := freshPostgres(t)
	insert := func() error {
		return db.Exec(`INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude)
			VALUES (?, 'ODP-01', 'odp', 'ODP Satu', -6.2, 106.8)`, uuid.New()).Error
	}
	require.NoError(t, insert())
	require.Error(t, insert(), "a second node with the same node_id must be refused")
}

func TestDatabaseRefusesAnUnknownNodeType(t *testing.T) {
	db := freshPostgres(t)
	err := db.Exec(`INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude)
		VALUES (?, 'X-01', 'splitter', 'Bukan Jenis', -6.2, 106.8)`, uuid.New()).Error
	require.Error(t, err)
}

func TestDatabaseRefusesAnUnknownFiberType(t *testing.T) {
	db := freshPostgres(t)
	err := db.Exec(`INSERT INTO mapping_edges (id, edge_id, source, target, fiber_type)
		VALUES (?, 'E-01', 'A', 'B', 'kabel-listrik')`, uuid.New()).Error
	require.Error(t, err)
}

func TestDatabaseAcceptsEveryFiberTypeTheFieldUses(t *testing.T) {
	db := freshPostgres(t)
	for i, ft := range []string{
		"feeder", "distribution", "drop",
		"odp_to_odp", "odp_to_odp_ratio", "odc_to_odc", "odc_to_odc_ratio",
	} {
		err := db.Exec(`INSERT INTO mapping_edges (id, edge_id, source, target, fiber_type)
			VALUES (?, ?, 'A', 'B', ?)`, uuid.New(), "E-"+ft, ft).Error
		require.NoErrorf(t, err, "fiber type %d (%s) must be accepted", i, ft)
	}
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" \
  go test ./internal/database/ -run TestDatabaseRefusesTwoNodesWithOneNodeID -v
```
Expected: FAIL — tabel `mapping_nodes` belum ada.

- [ ] **Step 3: Tulis model**

Buat `backend/internal/models/mapping.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NodeType is what a point on the map is.
type NodeType string

const (
	NodeServer NodeType = "server"
	NodeODC    NodeType = "odc"
	NodeODP    NodeType = "odp"
	NodeONT    NodeType = "ont"
)

// FiberType is what a cable between two nodes carries. The cascade kinds exist
// because a distribution box is often fed from another one rather than from the
// cabinet, which the plant model before this could not express at all.
type FiberType string

const (
	FiberFeeder         FiberType = "feeder"
	FiberDistribution   FiberType = "distribution"
	FiberDrop           FiberType = "drop"
	FiberODPToODP       FiberType = "odp_to_odp"
	FiberODPToODPRatio  FiberType = "odp_to_odp_ratio"
	FiberODCToODC       FiberType = "odc_to_odc"
	FiberODCToODCRatio  FiberType = "odc_to_odc_ratio"
)

// MappingNode is one box, cabinet or customer unit on the map. Only a name and
// a position are required: a technician standing at a pole should be able to
// record it in two taps and fill the rest in later.
type MappingNode struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	NodeID       string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"node_id"`
	Type         NodeType  `gorm:"type:varchar(16);not null" json:"type"`
	Name         string    `gorm:"type:varchar(120);not null" json:"name"`
	Latitude     float64   `gorm:"not null" json:"latitude"`
	Longitude    float64   `gorm:"not null" json:"longitude"`
	Capacity     int       `json:"capacity"`
	Splitter     string    `gorm:"type:varchar(16)" json:"splitter"`
	PPPoE        string    `gorm:"type:varchar(64)" json:"pppoe"`
	SerialNumber string    `gorm:"type:varchar(64)" json:"serial_number"`
	Notes        string    `gorm:"type:text" json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (n *MappingNode) BeforeCreate(*gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (MappingNode) TableName() string { return "mapping_nodes" }

// MappingEdge is one cable. Waypoints hold the path it actually takes, so the
// length shown is the cable that was pulled rather than the straight line.
type MappingEdge struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	EdgeID    string         `gorm:"type:varchar(64);not null;uniqueIndex" json:"edge_id"`
	Source    string         `gorm:"type:varchar(64);not null" json:"source"`
	Target    string         `gorm:"type:varchar(64);not null" json:"target"`
	FiberType FiberType      `gorm:"type:varchar(24)" json:"fiber_type"`
	Distance  float64        `json:"distance"`
	Waypoints datatypes.JSON `gorm:"type:jsonb" json:"waypoints"`
	Notes     string         `gorm:"type:text" json:"notes"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func (e *MappingEdge) BeforeCreate(*gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (MappingEdge) TableName() string { return "mapping_edges" }
```

- [ ] **Step 4: Daftarkan ke AutoMigrate**

Di `backend/internal/models/models.go`, tepat setelah baris `&ODP{},`, tambahkan:

```go
		&MappingNode{},
		&MappingEdge{},
```

(`&ODC{}`, `&ODCFeed{}`, dan `&ODP{}` dihapus nanti di Task 7, bukan sekarang — Task 6 masih membutuhkannya.)

- [ ] **Step 5: Tulis migrasi**

Buat `backend/migrations/53_network_mapping.sql`:

```sql
-- mapping_nodes dan mapping_edges: peta jaringan yang bisa diisi dari lapangan.
--
-- AutoMigrate membuat tabelnya dari model; berkas ini menambahkan yang tag model
-- tidak bisa nyatakan, lalu memindahkan isi model plant lama.
--
-- Tidak ada foreign key dari edge ke node. Sebuah kabel digambar lebih dulu dan
-- ujungnya diberi nama menyusul, dan menghapus satu node tidak boleh membuat
-- seluruh jalur yang sudah ditarik ikut hilang tanpa jejak.

ALTER TABLE mapping_nodes DROP CONSTRAINT IF EXISTS mapping_nodes_type_valid;
ALTER TABLE mapping_nodes ADD CONSTRAINT mapping_nodes_type_valid
    CHECK (type IN ('server', 'odc', 'odp', 'ont'));

ALTER TABLE mapping_edges DROP CONSTRAINT IF EXISTS mapping_edges_fiber_type_valid;
ALTER TABLE mapping_edges ADD CONSTRAINT mapping_edges_fiber_type_valid
    CHECK (fiber_type IS NULL OR fiber_type IN (
        'feeder', 'distribution', 'drop',
        'odp_to_odp', 'odp_to_odp_ratio', 'odc_to_odc', 'odc_to_odc_ratio'));

-- Dibaca saat menggambar kabel dan saat menghitung slot terpakai.
CREATE INDEX IF NOT EXISTS idx_mapping_edges_source ON mapping_edges (source);
CREATE INDEX IF NOT EXISTS idx_mapping_edges_target ON mapping_edges (target);
CREATE INDEX IF NOT EXISTS idx_mapping_nodes_type ON mapping_nodes (type);

-- Pindahkan ODC dan ODP lama. Keduanya hanya berisi satu baris di produksi,
-- tapi memindahkannya berarti tidak ada yang hilang di instalasi mana pun.
INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT gen_random_uuid(), 'ODC-' || o.code, 'odc', o.code,
       COALESCE(o.latitude, 0), COALESCE(o.longitude, 0), 0,
       COALESCE(o.notes, ''), o.created_at, o.updated_at
FROM odcs o
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = 'ODC-' || o.code);

INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT gen_random_uuid(), 'ODP-' || p.code, 'odp', p.code,
       COALESCE(p.latitude, 0), COALESCE(p.longitude, 0), COALESCE(p.port_count, 0),
       COALESCE(p.notes, ''), p.created_at, p.updated_at
FROM odps p
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = 'ODP-' || p.code);
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

```bash
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" \
  go test ./internal/database/ -run TestDatabase -v
```
Expected: keempat tes baru PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/models/mapping.go backend/migrations/53_network_mapping.sql \
        backend/internal/models/models.go backend/internal/database/migrations_mapping_postgres_test.go
git commit -m "feat(map): add the tables a field-filled map needs

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Layanan node

**Files:**
- Create: `backend/internal/services/mapping_nodes.go`
- Test: `backend/internal/services/mapping_nodes_test.go`

**Interfaces:**
- Consumes: `models.MappingNode`, `models.NodeType` (Task 1).
- Produces: `func NewMappingService(db *gorm.DB) *MappingService`; metode `CreateNode(models.MappingNode) (*models.MappingNode, error)`, `ListNodes() ([]models.MappingNode, error)`, `GetNode(nodeID string) (*models.MappingNode, error)`, `UpdateNode(nodeID string, in models.MappingNode) (*models.MappingNode, error)`, `DeleteNode(nodeID string) error`. Error sentinel `ErrNodeExists`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/mapping_nodes_test.go`:

```go
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func mappingSetup(t *testing.T) *MappingService {
	t.Helper()
	return NewMappingService(setupTestDB(t))
}

func odpNode(id, name string) models.MappingNode {
	return models.MappingNode{
		NodeID: id, Type: models.NodeODP, Name: name,
		Latitude: -6.2, Longitude: 106.8, Capacity: 8,
	}
}

// Two taps in the field: a name and a position. Everything else can wait, and
// demanding more is what left the plant map empty for a year.
func TestANodeNeedsOnlyANameAndAPosition(t *testing.T) {
	s := mappingSetup(t)

	saved, err := s.CreateNode(models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Depan Masjid",
		Latitude: -6.21, Longitude: 106.81,
	})

	require.NoError(t, err)
	assert.Equal(t, "ODP-01", saved.NodeID)
	assert.Equal(t, 0, saved.Capacity, "capacity may be left unset")
	assert.Empty(t, saved.Splitter)
}

func TestASecondNodeWithTheSameIDIsRefused(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Satu"))
	require.NoError(t, err)

	_, err = s.CreateNode(odpNode("ODP-01", "Dua"))

	require.ErrorIs(t, err, ErrNodeExists)
}

func TestUpdatingANodeKeepsItsIdentity(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Nama Lama"))
	require.NoError(t, err)

	updated, err := s.UpdateNode("ODP-01", models.MappingNode{
		Name: "Nama Baru", Type: models.NodeODP,
		Latitude: -6.3, Longitude: 106.9, Capacity: 16,
	})

	require.NoError(t, err)
	assert.Equal(t, "ODP-01", updated.NodeID)
	assert.Equal(t, "Nama Baru", updated.Name)
	assert.Equal(t, 16, updated.Capacity)
}

func TestListingReturnsEveryNodePlaced(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Satu"))
	require.NoError(t, err)
	_, err = s.CreateNode(odpNode("ODP-02", "Dua"))
	require.NoError(t, err)

	nodes, err := s.ListNodes()

	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestDeletingANodeThatIsNotThereSaysSo(t *testing.T) {
	s := mappingSetup(t)

	err := s.DeleteNode("ODP-TIDAK-ADA")

	require.Error(t, err)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd backend && go test ./internal/services/ -run TestANodeNeedsOnlyANameAndAPosition
```
Expected: FAIL — `undefined: NewMappingService`.

- [ ] **Step 3: Tulis layanan**

Buat `backend/internal/services/mapping_nodes.go`:

```go
package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ErrNodeExists marks a node id that is already on the map.
var ErrNodeExists = errors.New("node id already exists")

// MappingService holds the network map: the boxes and the cables between them.
type MappingService struct {
	db *gorm.DB
}

func NewMappingService(db *gorm.DB) *MappingService {
	return &MappingService{db: db}
}

// CreateNode places one box on the map. Only a name and a position are
// required; the rest is filled in later, from a desk rather than a pole.
func (s *MappingService) CreateNode(in models.MappingNode) (*models.MappingNode, error) {
	if err := s.db.Create(&in).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("%w: %s", ErrNodeExists, in.NodeID)
		}
		return nil, fmt.Errorf("create node: %w", err)
	}
	return &in, nil
}

func (s *MappingService) ListNodes() ([]models.MappingNode, error) {
	var nodes []models.MappingNode
	if err := s.db.Order("type, name").Find(&nodes).Error; err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	return nodes, nil
}

func (s *MappingService) GetNode(nodeID string) (*models.MappingNode, error) {
	var node models.MappingNode
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return nil, fmt.Errorf("get node %s: %w", nodeID, err)
	}
	return &node, nil
}

// UpdateNode changes what a box says about itself. Its node_id is its identity
// and the cables point at it, so it is never part of an update.
func (s *MappingService) UpdateNode(nodeID string, in models.MappingNode) (*models.MappingNode, error) {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{
		"type": in.Type, "name": in.Name,
		"latitude": in.Latitude, "longitude": in.Longitude,
		"capacity": in.Capacity, "splitter": in.Splitter,
		"pppoe": in.PPPoE, "serial_number": in.SerialNumber, "notes": in.Notes,
	}
	if err := s.db.Model(node).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("update node %s: %w", nodeID, err)
	}
	return s.GetNode(nodeID)
}

func (s *MappingService) DeleteNode(nodeID string) error {
	res := s.db.Where("node_id = ?", nodeID).Delete(&models.MappingNode{})
	if res.Error != nil {
		return fmt.Errorf("delete node %s: %w", nodeID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("delete node %s: %w", nodeID, gorm.ErrRecordNotFound)
	}
	return nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

```bash
cd backend && go test ./internal/services/ -run 'TestANode|TestASecondNode|TestUpdatingANode|TestListingReturns|TestDeletingANode' -v
```
Expected: kelima tes PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/mapping_nodes.go backend/internal/services/mapping_nodes_test.go
git commit -m "feat(map): place a box with a name and a position

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Layanan kabel dan aturan kapasitas

**Files:**
- Create: `backend/internal/services/mapping_edges.go`
- Test: `backend/internal/services/mapping_edges_test.go`

**Interfaces:**
- Consumes: `MappingService`, `ErrNodeExists` (Task 2); `models.MappingEdge`, `models.FiberType` (Task 1).
- Produces: metode `CreateEdge(models.MappingEdge) (*models.MappingEdge, error)`, `ListEdges() ([]models.MappingEdge, error)`, `UpdateEdge(edgeID string, in models.MappingEdge) (*models.MappingEdge, error)`, `DeleteEdge(edgeID string) error`. Error sentinel `ErrEdgeExists`, `ErrSlotsFull`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/mapping_edges_test.go`:

```go
package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func plantNode(t *testing.T, s *MappingService, id string, kind models.NodeType, capacity int) {
	t.Helper()
	_, err := s.CreateNode(models.MappingNode{
		NodeID: id, Type: kind, Name: id,
		Latitude: -6.2, Longitude: 106.8, Capacity: capacity,
	})
	require.NoError(t, err)
}

func drawCable(s *MappingService, id, from, to string, kind models.FiberType) error {
	_, err := s.CreateEdge(models.MappingEdge{
		EdgeID: id, Source: from, Target: to, FiberType: kind,
	})
	return err
}

// A cabinet with eight slots holds eight boxes. The ninth is a mistake worth
// catching at the moment it is drawn, not at the moment someone visits the site.
func TestACabinetRefusesOneBoxPastItsSlots(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 8)
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 8)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}
	plantNode(t, s, "ODP-09", models.NodeODP, 8)

	err := drawCable(s, "E-ODP-09", "ODC-01", "ODP-09", models.FiberDistribution)

	require.ErrorIs(t, err, ErrSlotsFull)
	assert.Contains(t, err.Error(), "8/8", "the refusal has to say how full it is")
}

// Cascaded boxes are counted against their own kind, so feeding one cabinet
// from another does not eat the slots meant for distribution boxes.
func TestACascadeIsCountedApartFromOrdinaryDrops(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODP-01", models.NodeODP, 1)
	plantNode(t, s, "ONT-01", models.NodeONT, 0)
	plantNode(t, s, "ODP-02", models.NodeODP, 1)

	require.NoError(t, drawCable(s, "E-1", "ODP-01", "ONT-01", models.FiberDrop))

	err := drawCable(s, "E-2", "ODP-01", "ODP-02", models.FiberODPToODP)

	require.NoError(t, err, "a cascade does not consume the drop slot")
}

// Nobody counts the ports on every box the day they place it.
func TestACapacityOfZeroNeverRefuses(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 0)
	for i := 1; i <= 20; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 0)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}
}

func TestASecondCableWithTheSameIDIsRefused(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 0)
	plantNode(t, s, "ODP-01", models.NodeODP, 0)
	require.NoError(t, drawCable(s, "E-1", "ODC-01", "ODP-01", models.FiberDistribution))

	err := drawCable(s, "E-1", "ODC-01", "ODP-01", models.FiberDistribution)

	require.ErrorIs(t, err, ErrEdgeExists)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd backend && go test ./internal/services/ -run TestACabinetRefusesOneBoxPastItsSlots
```
Expected: FAIL — `s.CreateEdge undefined`.

- [ ] **Step 3: Tulis layanan kabel**

Buat `backend/internal/services/mapping_edges.go`:

```go
package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

var (
	// ErrEdgeExists marks a cable id that is already drawn.
	ErrEdgeExists = errors.New("edge id already exists")
	// ErrSlotsFull marks a box asked to hold more than it has ports for.
	ErrSlotsFull = errors.New("slots are full")
)

// CreateEdge draws one cable. A cabinet or box with a stated capacity refuses
// the connection that would overfill it, counted per kind of thing hanging off
// it — a cascade to another box does not eat a customer's slot.
func (s *MappingService) CreateEdge(in models.MappingEdge) (*models.MappingEdge, error) {
	if err := s.checkSlots(in); err != nil {
		return nil, err
	}
	if err := s.db.Create(&in).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("%w: %s", ErrEdgeExists, in.EdgeID)
		}
		return nil, fmt.Errorf("create edge: %w", err)
	}
	return &in, nil
}

// checkSlots counts what already hangs off the source, of the same kind as what
// is being added. A capacity of zero means nobody has counted the ports yet.
func (s *MappingService) checkSlots(in models.MappingEdge) error {
	source, err := s.GetNode(in.Source)
	if err != nil {
		return fmt.Errorf("source %s: %w", in.Source, err)
	}
	if source.Capacity <= 0 {
		return nil
	}
	target, err := s.GetNode(in.Target)
	if err != nil {
		return fmt.Errorf("target %s: %w", in.Target, err)
	}
	kind, counts := slotKind(source.Type, target.Type, in.FiberType)
	if !counts {
		return nil
	}

	var used int64
	q := s.db.Model(&models.MappingEdge{}).
		Where("source = ?", in.Source).
		Where("target IN (SELECT node_id FROM mapping_nodes WHERE type = ?)", kind.target)
	if kind.fiber != "" {
		q = q.Where("fiber_type = ?", kind.fiber)
	}
	if err := q.Count(&used).Error; err != nil {
		return fmt.Errorf("count slots on %s: %w", in.Source, err)
	}
	if used >= int64(source.Capacity) {
		return fmt.Errorf("%w: %q is full (%d/%d)", ErrSlotsFull, in.Source, used, source.Capacity)
	}
	return nil
}

type slotCount struct {
	target models.NodeType
	fiber  models.FiberType
}

// slotKind says which slots this cable consumes, or that it consumes none.
func slotKind(source, target models.NodeType, fiber models.FiberType) (slotCount, bool) {
	switch {
	case source == models.NodeODC && target == models.NodeODP:
		return slotCount{target: models.NodeODP}, true
	case source == models.NodeODC && target == models.NodeODC && fiber == models.FiberODCToODC:
		return slotCount{target: models.NodeODC, fiber: models.FiberODCToODC}, true
	case source == models.NodeODP && target == models.NodeONT:
		return slotCount{target: models.NodeONT}, true
	case source == models.NodeODP && target == models.NodeODP && fiber == models.FiberODPToODP:
		return slotCount{target: models.NodeODP, fiber: models.FiberODPToODP}, true
	}
	return slotCount{}, false
}

func (s *MappingService) ListEdges() ([]models.MappingEdge, error) {
	var edges []models.MappingEdge
	if err := s.db.Order("edge_id").Find(&edges).Error; err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	return edges, nil
}

func (s *MappingService) UpdateEdge(edgeID string, in models.MappingEdge) (*models.MappingEdge, error) {
	var edge models.MappingEdge
	if err := s.db.Where("edge_id = ?", edgeID).First(&edge).Error; err != nil {
		return nil, fmt.Errorf("get edge %s: %w", edgeID, err)
	}
	fields := map[string]any{
		"source": in.Source, "target": in.Target, "fiber_type": in.FiberType,
		"distance": in.Distance, "waypoints": in.Waypoints, "notes": in.Notes,
	}
	if err := s.db.Model(&edge).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("update edge %s: %w", edgeID, err)
	}
	var updated models.MappingEdge
	if err := s.db.Where("edge_id = ?", edgeID).First(&updated).Error; err != nil {
		return nil, fmt.Errorf("get edge %s: %w", edgeID, err)
	}
	return &updated, nil
}

func (s *MappingService) DeleteEdge(edgeID string) error {
	res := s.db.Where("edge_id = ?", edgeID).Delete(&models.MappingEdge{})
	if res.Error != nil {
		return fmt.Errorf("delete edge %s: %w", edgeID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("delete edge %s: %w", edgeID, gorm.ErrRecordNotFound)
	}
	return nil
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

```bash
cd backend && go test ./internal/services/ -run 'TestACabinet|TestACascade|TestACapacity|TestASecondCable' -v
```
Expected: keempat tes PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/mapping_edges.go backend/internal/services/mapping_edges_test.go
git commit -m "feat(map): draw a cable, and refuse the one that overfills a box

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: API node dan kabel

**Files:**
- Create: `backend/internal/api/mapping_dto.go`
- Create: `backend/internal/api/mapping_handler.go`
- Modify: `backend/internal/api/router.go` (grup rute baru)
- Modify: `backend/internal/api/router_handlers.go` (pasang handler)
- Test: `backend/internal/api/mapping_handler_test.go`

**Interfaces:**
- Consumes: `services.NewMappingService`, semua metodenya, `services.ErrNodeExists`, `services.ErrEdgeExists`, `services.ErrSlotsFull` (Task 2, 3); `mapCSError` tidak dipakai — gunakan `ErrorResponse` dari `dto.go`; `middleware.RequireRole`.
- Produces: `func NewMappingHandler(mapping *services.MappingService) *MappingHandler`; rute `GET/POST /api/v1/mapping/nodes`, `PUT/DELETE /api/v1/mapping/nodes/:node_id`, `GET/POST /api/v1/mapping/edges`, `PUT/DELETE /api/v1/mapping/edges/:edge_id`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/api/mapping_handler_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func mappingRouter(t *testing.T) (*gin.Engine, *services.MappingService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := TestDB(t)
	svc := services.NewMappingService(db)
	h := NewMappingHandler(svc)

	r := gin.New()
	g := r.Group("/api/v1/mapping")
	g.GET("/nodes", h.ListNodes)
	g.POST("/nodes", h.CreateNode)
	g.PUT("/nodes/:node_id", h.UpdateNode)
	g.DELETE("/nodes/:node_id", h.DeleteNode)
	g.GET("/edges", h.ListEdges)
	g.POST("/edges", h.CreateEdge)
	return r, svc
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPlacingANodeAnswersWithWhatWasStored(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "ODP Depan Masjid",
		"latitude": -6.21, "longitude": 106.81,
	})

	require.Equal(t, http.StatusCreated, rec.Code)
	var body struct {
		Data models.MappingNode `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ODP-01", body.Data.NodeID)
}

func TestANodeWithoutAPositionIsRefused(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Tanpa Koordinat",
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestARepeatedNodeIDAnswers409(t *testing.T) {
	r, _ := mappingRouter(t)
	first := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Satu",
		"latitude": -6.2, "longitude": 106.8,
	})
	require.Equal(t, http.StatusCreated, first.Code)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Dua",
		"latitude": -6.2, "longitude": 106.8,
	})

	assert.Equal(t, http.StatusConflict, rec.Code)
}

// The refusal has to reach the technician, not just the log.
func TestAFullCabinetAnswers409WithItsNumbers(t *testing.T) {
	r, svc := mappingRouter(t)
	_, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC", Latitude: -6.2, Longitude: 106.8, Capacity: 1,
	})
	require.NoError(t, err)
	for _, id := range []string{"ODP-01", "ODP-02"} {
		_, err := svc.CreateNode(models.MappingNode{
			NodeID: id, Type: models.NodeODP, Name: id, Latitude: -6.2, Longitude: 106.8,
		})
		require.NoError(t, err)
	}
	first := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01", "fiber_type": "distribution",
	})
	require.Equal(t, http.StatusCreated, first.Code)

	rec := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-2", "source": "ODC-01", "target": "ODP-02", "fiber_type": "distribution",
	})

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "1/1")
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd backend && go test ./internal/api/ -run TestPlacingANodeAnswersWithWhatWasStored
```
Expected: FAIL — `undefined: NewMappingHandler`.

- [ ] **Step 3: Tulis DTO**

Buat `backend/internal/api/mapping_dto.go`:

```go
package api

import (
	"gorm.io/datatypes"

	"github.com/tikman/olt-provisioning/internal/models"
)

// nodeRequest is what the map sends when a box is placed or edited. Only the
// name and the position are required — that is the whole point of this map.
type nodeRequest struct {
	NodeID       string          `json:"node_id" binding:"required"`
	Type         models.NodeType `json:"type" binding:"required,oneof=server odc odp ont"`
	Name         string          `json:"name" binding:"required"`
	Latitude     *float64        `json:"latitude" binding:"required"`
	Longitude    *float64        `json:"longitude" binding:"required"`
	Capacity     int             `json:"capacity"`
	Splitter     string          `json:"splitter"`
	PPPoE        string          `json:"pppoe"`
	SerialNumber string          `json:"serial_number"`
	Notes        string          `json:"notes"`
}

func (r nodeRequest) toModel() models.MappingNode {
	return models.MappingNode{
		NodeID: r.NodeID, Type: r.Type, Name: r.Name,
		Latitude: *r.Latitude, Longitude: *r.Longitude,
		Capacity: r.Capacity, Splitter: r.Splitter,
		PPPoE: r.PPPoE, SerialNumber: r.SerialNumber, Notes: r.Notes,
	}
}

// edgeRequest is one cable. Waypoints are the path it actually takes.
type edgeRequest struct {
	EdgeID    string           `json:"edge_id" binding:"required"`
	Source    string           `json:"source" binding:"required"`
	Target    string           `json:"target" binding:"required"`
	FiberType models.FiberType `json:"fiber_type" binding:"omitempty,oneof=feeder distribution drop odp_to_odp odp_to_odp_ratio odc_to_odc odc_to_odc_ratio"`
	Distance  float64          `json:"distance"`
	Waypoints datatypes.JSON   `json:"waypoints"`
	Notes     string           `json:"notes"`
}

func (r edgeRequest) toModel() models.MappingEdge {
	return models.MappingEdge{
		EdgeID: r.EdgeID, Source: r.Source, Target: r.Target,
		FiberType: r.FiberType, Distance: r.Distance,
		Waypoints: r.Waypoints, Notes: r.Notes,
	}
}
```

- [ ] **Step 4: Tulis handler**

Buat `backend/internal/api/mapping_handler.go`:

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/services"
)

// MappingHandler serves the network map.
type MappingHandler struct {
	mapping *services.MappingService
}

func NewMappingHandler(mapping *services.MappingService) *MappingHandler {
	return &MappingHandler{mapping: mapping}
}

// mappingError turns a service error into the answer a technician sees. A full
// box and a repeated id are both 409: the request was understood and refused.
func mappingError(c *gin.Context, err error, notFoundCode string) {
	switch {
	case errors.Is(err, services.ErrNodeExists), errors.Is(err, services.ErrEdgeExists),
		errors.Is(err, services.ErrSlotsFull):
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "MAPPING_CONFLICT"})
	default:
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error(), Code: notFoundCode})
	}
}

func (h *MappingHandler) ListNodes(c *gin.Context) {
	nodes, err := h.mapping.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": nodes})
}

func (h *MappingHandler) CreateNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_NODE"})
		return
	}
	node, err := h.mapping.CreateNode(req.toModel())
	if err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": node})
}

func (h *MappingHandler) UpdateNode(c *gin.Context) {
	var req nodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_NODE"})
		return
	}
	node, err := h.mapping.UpdateNode(c.Param("node_id"), req.toModel())
	if err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": node})
}

func (h *MappingHandler) DeleteNode(c *gin.Context) {
	if err := h.mapping.DeleteNode(c.Param("node_id")); err != nil {
		mappingError(c, err, "NODE_NOT_FOUND")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MappingHandler) ListEdges(c *gin.Context) {
	edges, err := h.mapping.ListEdges()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: "MAPPING_FAILED"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": edges})
}

func (h *MappingHandler) CreateEdge(c *gin.Context) {
	var req edgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_EDGE"})
		return
	}
	edge, err := h.mapping.CreateEdge(req.toModel())
	if err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": edge})
}

func (h *MappingHandler) UpdateEdge(c *gin.Context) {
	var req edgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_EDGE"})
		return
	}
	edge, err := h.mapping.UpdateEdge(c.Param("edge_id"), req.toModel())
	if err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": edge})
}

func (h *MappingHandler) DeleteEdge(c *gin.Context) {
	if err := h.mapping.DeleteEdge(c.Param("edge_id")); err != nil {
		mappingError(c, err, "EDGE_NOT_FOUND")
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 5: Pasang handler dan rute**

Di `backend/internal/api/router_handlers.go`, tambahkan medan ke struct `handlers`:

```go
	mappingHandler *MappingHandler
```

dan pada literal `newHandlers`:

```go
		mappingHandler: NewMappingHandler(services.NewMappingService(db)),
```

Di `backend/internal/api/router.go`, setelah grup `odcs`, tambahkan:

```go
	mapping := api.Group("/mapping")
	mapping.Use(authenticated)
	{
		mapping.GET("/nodes", h.mappingHandler.ListNodes)
		mapping.GET("/edges", h.mappingHandler.ListEdges)

		editor := middleware.RequireRole(models.UserRoleAdmin, models.UserRoleTechnician)
		mapping.POST("/nodes", editor, h.mappingHandler.CreateNode)
		mapping.PUT("/nodes/:node_id", editor, h.mappingHandler.UpdateNode)
		mapping.DELETE("/nodes/:node_id", editor, h.mappingHandler.DeleteNode)
		mapping.POST("/edges", editor, h.mappingHandler.CreateEdge)
		mapping.PUT("/edges/:edge_id", editor, h.mappingHandler.UpdateEdge)
		mapping.DELETE("/edges/:edge_id", editor, h.mappingHandler.DeleteEdge)
	}
```

- [ ] **Step 6: Jalankan tes, pastikan lulus**

```bash
cd backend && go test ./internal/api/ -run 'TestPlacingANode|TestANodeWithout|TestARepeatedNodeID|TestAFullCabinet' -v
```
Expected: keempat tes PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/api/mapping_dto.go backend/internal/api/mapping_handler.go \
        backend/internal/api/mapping_handler_test.go backend/internal/api/router.go \
        backend/internal/api/router_handlers.go
git commit -m "feat(map): serve the map, and say which box is full

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Alihkan ONT dan registrasi ZTE ke model baru

**Files:**
- Modify: `backend/internal/services/zte_register_odp.go`
- Modify: `backend/internal/services/distribution_service.go` (fungsi penugasan ONT)
- Test: `backend/internal/services/mapping_ont_test.go`

**Interfaces:**
- Consumes: `models.MappingNode`, `models.NodeODP` (Task 1).
- Produces: `ONT.ODPID` kini menunjuk `mapping_nodes.id` bertipe `odp`; validasi registrasi ZTE memakai tabel baru.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `backend/internal/services/mapping_ont_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Registration asks which box the drop lands in. After the plant model changed,
// that box is a mapping node — and a registration naming a node that is not a
// distribution box is a mistake worth refusing.
func TestRegisteringAgainstAMappingODPIsAccepted(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMappingService(db)
	node, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Satu",
		Latitude: -6.2, Longitude: 106.8, Capacity: 8,
	})
	require.NoError(t, err)

	port := 3
	err = validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &node.ID, ODPPort: &port})

	require.NoError(t, err)
}

func TestRegisteringAgainstSomethingThatIsNotAnODPIsRefused(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMappingService(db)
	node, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC Satu",
		Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)

	port := 1
	err = validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &node.ID, ODPPort: &port})

	require.Error(t, err)
}

func TestRegisteringAgainstAnODPThatIsNotThereIsRefused(t *testing.T) {
	db := setupTestDB(t)
	missing := uuid.New()

	port := 1
	err := validateRegisterODP(db, models.ZTEGPONRegisterRequest{ODPID: &missing, ODPPort: &port})

	require.Error(t, err)
}
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd backend && go test ./internal/services/ -run TestRegisteringAgainstAMappingODPIsAccepted -v
```
Expected: FAIL — validasi masih mencari di tabel `odps`.

- [ ] **Step 3: Alihkan validasinya**

Di `backend/internal/services/zte_register_odp.go`, ganti bagian yang membaca `models.ODP` menjadi:

```go
	// The plant model moved: a drop lands in a mapping node of type odp, not in
	// the old odps table. Naming anything else is a mistake worth refusing here
	// rather than discovering at the pole.
	var node models.MappingNode
	if err := db.Where("id = ? AND type = ?", *req.ODPID, models.NodeODP).
		First(&node).Error; err != nil {
		return fmt.Errorf("%w: no distribution box with that id", ErrValidation)
	}
	if node.Capacity > 0 && *req.ODPPort > node.Capacity {
		return fmt.Errorf("%w: port %d is past the %d this box has",
			ErrValidation, *req.ODPPort, node.Capacity)
	}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

```bash
cd backend && go test ./internal/services/ -run TestRegisteringAgainst -v
```
Expected: ketiga tes PASS.

- [ ] **Step 5: Jalankan seluruh paket, catat yang rusak**

```bash
cd backend && go test ./internal/services/ ./internal/api/ 2>&1 | grep -E '^--- FAIL' | sort -u
```
Expected: hanya tes WireGuard baseline, plus tes lama distribusi yang akan dihapus di Task 6. Catat namanya di laporan.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add backend/internal/services/zte_register_odp.go backend/internal/services/mapping_ont_test.go
git commit -m "feat(map): point registration at the box on the new map

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Hapus model plant lama

**Files:**
- Delete: `backend/internal/models/distribution.go`, `backend/internal/services/distribution_service.go`, `distribution_queries.go`, `route_store.go`, dan tes-tesnya; `backend/internal/api/distribution_dto.go`, `distribution_handler.go`, dan tesnya.
- Modify: `backend/internal/models/models.go`, `backend/internal/api/router.go`, `backend/internal/api/router_handlers.go`
- Create: `backend/migrations/54_drop_old_plant.sql`

**Interfaces:**
- Consumes: Task 5 sudah memindahkan satu-satunya pemakai `models.ODP` di luar paket distribusi.
- Produces: tidak ada; tugas ini hanya menghapus.

- [ ] **Step 1: Cari semua yang masih menyebut model lama**

```bash
cd backend && grep -rn "models.ODP\b\|models.ODC\b\|models.ODCFeed\|distributionHandler\|DistributionService" --include="*.go" . | grep -v "_test.go" | grep -v "^./internal/services/distribution\|^./internal/api/distribution"
```
Expected: kosong setelah Task 5. Bila ada sisa, perbaiki sebelum menghapus.

- [ ] **Step 2: Hapus berkasnya**

```bash
cd backend
rm internal/models/distribution.go
rm internal/services/distribution_service.go internal/services/distribution_queries.go internal/services/route_store.go
rm internal/services/distribution_service_test.go internal/services/distribution_queries_test.go internal/services/distribution_postgres_test.go
rm internal/api/distribution_dto.go internal/api/distribution_handler.go internal/api/distribution_handler_test.go
```

- [ ] **Step 3: Bersihkan daftar AutoMigrate**

Di `backend/internal/models/models.go`, hapus baris `&ODC{},`, `&ODCFeed{},`, dan `&ODP{},`.

- [ ] **Step 4: Bersihkan router**

Di `backend/internal/api/router.go`, hapus grup `odcs`, grup `odcFeeds`, grup `odps` bila ada, dan dua rute `onts.PUT/DELETE("/:id/odp", ...)`. Di `router_handlers.go`, hapus medan `distributionHandler` dan barisnya di `newHandlers`.

- [ ] **Step 5: Tulis migrasi penghapusan**

Buat `backend/migrations/54_drop_old_plant.sql`:

```sql
-- Model plant lama digantikan mapping_nodes dan mapping_edges di migrasi 53,
-- yang sudah memindahkan isinya dengan id yang sama, jadi odp_id milik ONT
-- tetap sahih dan kolomnya tidak ikut dihapus.
--
-- Kunci asing ini harus dilepas lebih dulu: onts tidak ikut dihapus, jadi
-- fk_onts_odp (migrasi 39) masih menggantung ke odps dan membuat DROP TABLE
-- gagal -- yang berarti API menolak menyala setelah deploy.
ALTER TABLE onts DROP CONSTRAINT IF EXISTS fk_onts_odp;

DROP TABLE IF EXISTS odc_feeds;
DROP TABLE IF EXISTS odps;
DROP TABLE IF EXISTS odcs;
```

- [ ] **Step 6: Pastikan tetap terkompilasi dan tes lulus**

```bash
cd backend && go build ./... && go vet ./... && gofmt -s -l . && go test ./internal/services/ ./internal/api/ 2>&1 | tail -3
```
Expected: build dan vet bersih; gagal hanya tes WireGuard baseline.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add -A backend/
git commit -m "refactor(map): drop the plant model nobody filled in

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Lapisan data frontend

**Files:**
- Create: `frontend/src/domain/entities/Mapping.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Create: `frontend/src/infrastructure/repositories/MappingRepository.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Create: `frontend/src/application/hooks/useMapping.ts`
- Modify: `frontend/src/application/hooks/index.ts`
- Test: `frontend/src/infrastructure/repositories/MappingRepository.test.ts`

**Interfaces:**
- Consumes: bentuk JSON Task 4 (`{data: ...}`).
- Produces: tipe `NodeType`, `FiberType`, `MappingNode`, `MappingEdge`, `Waypoint`; kelas `MappingRepository` dengan `listNodes()`, `createNode(node)`, `updateNode(nodeId, node)`, `deleteNode(nodeId)`, `listEdges()`, `createEdge(edge)`, `updateEdge(edgeId, edge)`, `deleteEdge(edgeId)`; hook `useMappingNodes()`, `useMappingEdges()`, `useCreateNode()`, `useUpdateNode()`, `useDeleteNode()`, `useCreateEdge()`, `useDeleteEdge()`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/infrastructure/repositories/MappingRepository.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MappingRepository } from "./MappingRepository";

vi.mock("../http/apiClient", () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}));

const { apiClient } = await import("../http/apiClient");

describe("MappingRepository", () => {
  beforeEach(() => vi.clearAllMocks());

  it("unwraps the node list from its envelope", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { data: [{ nodeId: "ODP-01", type: "odp", name: "Satu" }] },
    } as never);

    const nodes = await new MappingRepository().listNodes();

    expect(nodes).toHaveLength(1);
    expect(nodes[0].nodeId).toBe("ODP-01");
  });

  it("addresses a node by its node id, not its uuid", async () => {
    vi.mocked(apiClient.delete).mockResolvedValue({ data: {} } as never);

    await new MappingRepository().deleteNode("ODP-01");

    expect(apiClient.delete).toHaveBeenCalledWith("/mapping/nodes/ODP-01");
  });

  it("returns an empty list rather than undefined when there is no map yet", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: {} } as never);

    await expect(new MappingRepository().listEdges()).resolves.toEqual([]);
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd frontend && npx vitest run src/infrastructure/repositories/MappingRepository.test.ts
```
Expected: FAIL — `Failed to resolve import "./MappingRepository"`.

- [ ] **Step 3: Tulis entity**

Buat `frontend/src/domain/entities/Mapping.ts`:

```ts
export type NodeType = "server" | "odc" | "odp" | "ont";

export type FiberType =
  | "feeder"
  | "distribution"
  | "drop"
  | "odp_to_odp"
  | "odp_to_odp_ratio"
  | "odc_to_odc"
  | "odc_to_odc_ratio";

export interface Waypoint {
  lat: number;
  lng: number;
}

export interface MappingNode {
  id?: string;
  nodeId: string;
  type: NodeType;
  name: string;
  latitude: number;
  longitude: number;
  capacity: number;
  splitter: string;
  pppoe: string;
  serialNumber: string;
  notes: string;
}

export interface MappingEdge {
  id?: string;
  edgeId: string;
  source: string;
  target: string;
  fiberType: FiberType;
  distance: number;
  waypoints: Waypoint[] | null;
  notes: string;
}
```

Tambahkan `export * from "./Mapping";` ke `frontend/src/domain/entities/index.ts`.

- [ ] **Step 4: Tulis endpoint dan repository**

Di `frontend/src/infrastructure/http/endpoints.ts`, tambahkan:

```ts
  MAPPING_NODES: "/mapping/nodes",
  MAPPING_NODE: (nodeId: string) => `/mapping/nodes/${nodeId}`,
  MAPPING_EDGES: "/mapping/edges",
  MAPPING_EDGE: (edgeId: string) => `/mapping/edges/${edgeId}`,
```

Buat `frontend/src/infrastructure/repositories/MappingRepository.ts`:

```ts
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";

export class MappingRepository {
  async listNodes(): Promise<MappingNode[]> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_NODES);
    return res.data.data ?? [];
  }

  async createNode(node: MappingNode): Promise<MappingNode> {
    const res = await apiClient.post(API_ENDPOINTS.MAPPING_NODES, node);
    return res.data.data;
  }

  async updateNode(nodeId: string, node: MappingNode): Promise<MappingNode> {
    const res = await apiClient.put(API_ENDPOINTS.MAPPING_NODE(nodeId), node);
    return res.data.data;
  }

  async deleteNode(nodeId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MAPPING_NODE(nodeId));
  }

  async listEdges(): Promise<MappingEdge[]> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_EDGES);
    return res.data.data ?? [];
  }

  async createEdge(edge: MappingEdge): Promise<MappingEdge> {
    const res = await apiClient.post(API_ENDPOINTS.MAPPING_EDGES, edge);
    return res.data.data;
  }

  async updateEdge(edgeId: string, edge: MappingEdge): Promise<MappingEdge> {
    const res = await apiClient.put(API_ENDPOINTS.MAPPING_EDGE(edgeId), edge);
    return res.data.data;
  }

  async deleteEdge(edgeId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MAPPING_EDGE(edgeId));
  }
}
```

Tambahkan `export * from "./MappingRepository";` ke `frontend/src/infrastructure/repositories/index.ts`.

- [ ] **Step 5: Tulis hook**

Buat `frontend/src/application/hooks/useMapping.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { MappingRepository } from "@/infrastructure/repositories";

const repo = new MappingRepository();
const NODES = ["mapping", "nodes"];
const EDGES = ["mapping", "edges"];

export function useMappingNodes() {
  return useQuery({ queryKey: NODES, queryFn: () => repo.listNodes() });
}

export function useMappingEdges() {
  return useQuery({ queryKey: EDGES, queryFn: () => repo.listEdges() });
}

export function useCreateNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (node: MappingNode) => repo.createNode(node),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useUpdateNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ nodeId, node }: { nodeId: string; node: MappingNode }) =>
      repo.updateNode(nodeId, node),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useDeleteNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (nodeId: string) => repo.deleteNode(nodeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useCreateEdge() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (edge: MappingEdge) => repo.createEdge(edge),
    onSuccess: () => qc.invalidateQueries({ queryKey: EDGES }),
  });
}

export function useDeleteEdge() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (edgeId: string) => repo.deleteEdge(edgeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: EDGES }),
  });
}
```

Tambahkan `export * from "./useMapping";` ke `frontend/src/application/hooks/index.ts`.

- [ ] **Step 6: Jalankan tes, pastikan lulus**

```bash
cd frontend && npx vitest run src/infrastructure/repositories/MappingRepository.test.ts && npx tsc --noEmit
```
Expected: ketiga tes PASS; tsc bersih.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/domain/entities frontend/src/infrastructure frontend/src/application/hooks
git commit -m "feat(map): read and write the map from the browser

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Jarak jalur dan label

**Files:**
- Create: `frontend/src/presentation/components/netmap/cableMath.ts`
- Create: `frontend/src/presentation/components/netmap/mappingLabels.ts`
- Test: `frontend/src/presentation/components/netmap/cableMath.test.ts`
- Test: `frontend/src/presentation/components/netmap/mappingLabels.test.ts`

**Interfaces:**
- Consumes: `Waypoint`, `NodeType`, `FiberType` (Task 7).
- Produces: `metersAlong(points: Waypoint[]): number`, `formatMeters(m: number): string`, `NODE_LABELS: Record<NodeType, string>`, `FIBER_LABELS: Record<FiberType, string>`, `NODE_COLORS: Record<NodeType, string>`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/components/netmap/cableMath.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { formatMeters, metersAlong } from "./cableMath";

describe("metersAlong", () => {
  // The length shown has to be the cable that was pulled, not the straight line
  // between its ends — that is the whole reason waypoints are stored.
  it("measures the path drawn, not the line between its ends", () => {
    const straight = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    const detoured = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.2, lng: 106.81 },
      { lat: -6.21, lng: 106.81 },
      { lat: -6.21, lng: 106.8 },
    ]);

    expect(detoured).toBeGreaterThan(straight);
  });

  it("is zero for a path that goes nowhere", () => {
    expect(metersAlong([])).toBe(0);
    expect(metersAlong([{ lat: -6.2, lng: 106.8 }])).toBe(0);
  });

  it("measures about a kilometre for a hundredth of a degree of latitude", () => {
    const m = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    expect(m).toBeGreaterThan(1050);
    expect(m).toBeLessThan(1160);
  });
});

describe("formatMeters", () => {
  it("reads in metres under a kilometre and in kilometres above it", () => {
    expect(formatMeters(240)).toBe("240 m");
    expect(formatMeters(1250)).toBe("1,25 km");
  });
});
```

Buat `frontend/src/presentation/components/netmap/mappingLabels.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { FIBER_LABELS, NODE_LABELS } from "./mappingLabels";

describe("labels", () => {
  it("names every kind of node in Indonesian", () => {
    expect(NODE_LABELS.server).toBe("Server / OLT");
    expect(NODE_LABELS.odc).toBe("ODC");
    expect(NODE_LABELS.odp).toBe("ODP");
    expect(NODE_LABELS.ont).toBe("ONT");
  });

  it("names every kind of cable, cascades included", () => {
    expect(FIBER_LABELS.feeder).toBe("Feeder");
    expect(FIBER_LABELS.distribution).toBe("Distribusi");
    expect(FIBER_LABELS.drop).toBe("Drop");
    expect(FIBER_LABELS.odp_to_odp).toBe("ODP ke ODP");
    expect(FIBER_LABELS.odc_to_odc).toBe("ODC ke ODC");
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/
```
Expected: FAIL — modul belum ada.

- [ ] **Step 3: Tulis keduanya**

Buat `frontend/src/presentation/components/netmap/cableMath.ts`:

```ts
import type { Waypoint } from "@/domain/entities";

const EARTH_RADIUS_M = 6_371_000;

const toRad = (deg: number) => (deg * Math.PI) / 180;

function metersBetween(a: Waypoint, b: Waypoint): number {
  const dLat = toRad(b.lat - a.lat);
  const dLng = toRad(b.lng - a.lng);
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(a.lat)) * Math.cos(toRad(b.lat)) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_RADIUS_M * Math.asin(Math.sqrt(h));
}

/** The length of the path as drawn, corner by corner. */
export function metersAlong(points: Waypoint[]): number {
  return points
    .slice(1)
    .reduce((total, point, i) => total + metersBetween(points[i], point), 0);
}

export function formatMeters(m: number): string {
  if (m >= 1000) {
    return `${(m / 1000).toFixed(2).replace(".", ",")} km`;
  }
  return `${Math.round(m)} m`;
}
```

Buat `frontend/src/presentation/components/netmap/mappingLabels.ts`:

```ts
import type { FiberType, NodeType } from "@/domain/entities";

export const NODE_LABELS: Record<NodeType, string> = {
  server: "Server / OLT",
  odc: "ODC",
  odp: "ODP",
  ont: "ONT",
};

// One colour per kind, so a glance at the map says what is where.
export const NODE_COLORS: Record<NodeType, string> = {
  server: "#8b5cf6",
  odc: "#3b82f6",
  odp: "#06b6d4",
  ont: "#22c55e",
};

export const FIBER_LABELS: Record<FiberType, string> = {
  feeder: "Feeder",
  distribution: "Distribusi",
  drop: "Drop",
  odp_to_odp: "ODP ke ODP",
  odp_to_odp_ratio: "ODP ke ODP (splitter)",
  odc_to_odc: "ODC ke ODC",
  odc_to_odc_ratio: "ODC ke ODC (splitter)",
};
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/
```
Expected: semua PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/netmap
git commit -m "feat(map): measure the cable that was pulled

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Toolbar dan formulir node

**Files:**
- Create: `frontend/src/presentation/components/netmap/MapToolbar.tsx`
- Create: `frontend/src/presentation/components/netmap/NodeFormModal.tsx`
- Test: `frontend/src/presentation/components/netmap/MapToolbar.test.tsx`
- Test: `frontend/src/presentation/components/netmap/NodeFormModal.test.tsx`

**Interfaces:**
- Consumes: `NODE_LABELS`, `NODE_COLORS` (Task 8); `MappingNode`, `NodeType` (Task 7).
- Produces: `<MapToolbar placing={...} onPlace={(type: NodeType) => void} onDrawCable={() => void} onCancel={() => void} view={...} onView={...} />`; `<NodeFormModal open type position onCancel onSubmit={(node: MappingNode) => void} />`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/components/netmap/MapToolbar.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MapToolbar } from "./MapToolbar";

const noop = () => {};

describe("MapToolbar", () => {
  it("offers one button per kind of thing that goes on a map", () => {
    render(
      <MapToolbar
        placing={undefined}
        onPlace={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    expect(screen.getByRole("button", { name: /Server/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODC/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ODP/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ONT/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Tarik kabel/ })).toBeInTheDocument();
  });

  it("says which kind is being placed", async () => {
    const onPlace = vi.fn();
    render(
      <MapToolbar
        placing={undefined}
        onPlace={onPlace}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /ODP/ }));

    expect(onPlace).toHaveBeenCalledWith("odp");
  });

  // While a box is being placed the other kinds are noise; what is needed is a
  // way out.
  it("offers a way to cancel once placing has started", () => {
    render(
      <MapToolbar
        placing="odp"
        onPlace={noop}
        onDrawCable={noop}
        onCancel={noop}
        view="map"
        onView={noop}
      />,
    );

    expect(screen.getByRole("button", { name: /Batal/ })).toBeInTheDocument();
  });
});
```

Buat `frontend/src/presentation/components/netmap/NodeFormModal.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NodeFormModal } from "./NodeFormModal";

describe("NodeFormModal", () => {
  // The position comes from the tap on the map, so the technician never types
  // a coordinate.
  it("fills the position from where the map was tapped", () => {
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByLabelText("Latitude")).toHaveValue("-6.21");
    expect(screen.getByLabelText("Longitude")).toHaveValue("106.81");
  });

  it("will not save a box with no name", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("asks an ONT for what only an ONT has", () => {
    render(
      <NodeFormModal
        open
        type="ont"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByLabelText("PPPoE")).toBeInTheDocument();
    expect(screen.getByLabelText("Serial")).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/MapToolbar.test.tsx
```
Expected: FAIL — modul belum ada.

- [ ] **Step 3: Tulis toolbar**

Buat `frontend/src/presentation/components/netmap/MapToolbar.tsx`:

```tsx
import { Button, Segmented, Space } from "antd";
import type { NodeType } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

export type MapView = "map" | "list";

interface MapToolbarProps {
  placing: NodeType | "cable" | undefined;
  onPlace: (type: NodeType) => void;
  onDrawCable: () => void;
  onCancel: () => void;
  view: MapView;
  onView: (view: MapView) => void;
}

const PLACEABLE: NodeType[] = ["server", "odc", "odp", "ont"];

export function MapToolbar({
  placing,
  onPlace,
  onDrawCable,
  onCancel,
  view,
  onView,
}: MapToolbarProps) {
  return (
    <Space
      wrap
      style={{ width: "100%", justifyContent: "space-between", padding: 8 }}
    >
      <Space wrap>
        {placing ? (
          <Button danger onClick={onCancel}>
            Batal
          </Button>
        ) : (
          <>
            {PLACEABLE.map((type) => (
              <Button
                key={type}
                type="primary"
                style={{ background: NODE_COLORS[type] }}
                onClick={() => onPlace(type)}
              >
                + {NODE_LABELS[type]}
              </Button>
            ))}
            <Button onClick={onDrawCable}>Tarik kabel</Button>
          </>
        )}
      </Space>
      <Segmented
        value={view}
        onChange={(v) => onView(v as MapView)}
        options={[
          { label: "Peta", value: "map" },
          { label: "Daftar", value: "list" },
        ]}
      />
    </Space>
  );
}
```

- [ ] **Step 4: Tulis formulir node**

Buat `frontend/src/presentation/components/netmap/NodeFormModal.tsx`:

```tsx
import { Form, Input, InputNumber, Modal } from "antd";
import { useEffect } from "react";
import type { MappingNode, NodeType } from "@/domain/entities";
import { NODE_LABELS } from "./mappingLabels";

interface NodeFormModalProps {
  open: boolean;
  type: NodeType;
  position: { lat: number; lng: number };
  onCancel: () => void;
  onSubmit: (node: MappingNode) => void;
}

export function NodeFormModal({
  open,
  type,
  position,
  onCancel,
  onSubmit,
}: NodeFormModalProps) {
  const [form] = Form.useForm();

  // The tap on the map is the position; retyping it by hand is how mistakes
  // get in.
  useEffect(() => {
    form.setFieldsValue({
      latitude: String(position.lat),
      longitude: String(position.lng),
    });
  }, [form, position]);

  const submit = async () => {
    const values = await form.validateFields();
    onSubmit({
      nodeId: values.nodeId || `${type.toUpperCase()}-${Date.now()}`,
      type,
      name: values.name,
      latitude: Number(values.latitude),
      longitude: Number(values.longitude),
      capacity: values.capacity ?? 0,
      splitter: values.splitter ?? "",
      pppoe: values.pppoe ?? "",
      serialNumber: values.serialNumber ?? "",
      notes: values.notes ?? "",
    });
  };

  return (
    <Modal
      open={open}
      title={`Tambah ${NODE_LABELS[type]}`}
      onCancel={onCancel}
      onOk={submit}
      okText="Simpan"
      cancelText="Batal"
      destroyOnClose
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="name"
          label="Nama"
          rules={[{ required: true, message: "Nama harus diisi" }]}
        >
          <Input placeholder="mis. ODP Depan Masjid" />
        </Form.Item>
        <Form.Item name="nodeId" label="Kode">
          <Input placeholder="dibuat otomatis bila dikosongkan" />
        </Form.Item>
        {type !== "ont" && (
          <>
            <Form.Item name="capacity" label="Jumlah slot">
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item name="splitter" label="Rasio splitter">
              <Input placeholder="mis. 1:8" />
            </Form.Item>
          </>
        )}
        {type === "ont" && (
          <>
            <Form.Item name="pppoe" label="PPPoE">
              <Input />
            </Form.Item>
            <Form.Item name="serialNumber" label="Serial">
              <Input />
            </Form.Item>
          </>
        )}
        <Form.Item name="latitude" label="Latitude">
          <Input readOnly />
        </Form.Item>
        <Form.Item name="longitude" label="Longitude">
          <Input readOnly />
        </Form.Item>
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 5: Jalankan tes, pastikan lulus**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/
```
Expected: semua PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/netmap
git commit -m "feat(map): place a box in two taps

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Kanvas peta dan penggambaran kabel

**Files:**
- Create: `frontend/src/presentation/components/netmap/useCableDraw.ts`
- Create: `frontend/src/presentation/components/netmap/MapCanvas.tsx`
- Test: `frontend/src/presentation/components/netmap/useCableDraw.test.ts`

**Interfaces:**
- Consumes: `metersAlong` (Task 8); `MappingNode`, `MappingEdge`, `Waypoint`, `NodeType` (Task 7); `NODE_COLORS` (Task 8).
- Produces: hook `useCableDraw()` mengembalikan `{ from, points, meters, start, addPoint, undoPoint, finish, cancel }`; komponen `<MapCanvas nodes edges placing onDrop onNodeClick />`.

**Catatan pustaka:** peta memakai `@vis.gl/react-google-maps` v1.9.0 yang sudah ada di `package.json` — `APIProvider`, `Map`, `AdvancedMarker`. Tidak ada dependensi baru.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/components/netmap/useCableDraw.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useCableDraw } from "./useCableDraw";

describe("useCableDraw", () => {
  // A misplaced corner on a twelve-point trace should cost one tap to fix, not
  // the whole cable.
  it("takes back one corner, not the whole path", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.addPoint({ lat: -6.21, lng: 106.81 }));
    act(() => result.current.undoPoint());

    expect(result.current.points).toHaveLength(1);
    expect(result.current.from).toBe("ODC-01");
  });

  it("measures the path as it is traced", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.addPoint({ lat: -6.21, lng: 106.8 }));

    expect(result.current.meters).toBeGreaterThan(1000);
  });

  it("forgets everything when the drawing is abandoned", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.cancel());

    expect(result.current.from).toBeUndefined();
    expect(result.current.points).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/useCableDraw.test.ts
```
Expected: FAIL — `Failed to resolve import "./useCableDraw"`.

- [ ] **Step 3: Tulis hook**

Buat `frontend/src/presentation/components/netmap/useCableDraw.ts`:

```ts
import { useMemo, useState } from "react";
import type { Waypoint } from "@/domain/entities";
import { metersAlong } from "./cableMath";

/** Tracing one cable: where it starts, the corners so far, and how long it is. */
export function useCableDraw() {
  const [from, setFrom] = useState<string>();
  const [points, setPoints] = useState<Waypoint[]>([]);

  const meters = useMemo(() => metersAlong(points), [points]);

  return {
    from,
    points,
    meters,
    start: (nodeId: string) => {
      setFrom(nodeId);
      setPoints([]);
    },
    addPoint: (point: Waypoint) => setPoints((p) => [...p, point]),
    undoPoint: () => setPoints((p) => p.slice(0, -1)),
    finish: () => {
      const traced = points;
      setFrom(undefined);
      setPoints([]);
      return traced;
    },
    cancel: () => {
      setFrom(undefined);
      setPoints([]);
    },
  };
}
```

- [ ] **Step 4: Jalankan tes, pastikan lulus**

```bash
cd frontend && npx vitest run src/presentation/components/netmap/useCableDraw.test.ts -v
```
Expected: ketiga tes PASS.

- [ ] **Step 5: Tulis kanvas**

Baca dulu `frontend/src/presentation/components/map/OltMap.tsx` baris 1-100 untuk melihat bagaimana `APIProvider` diberi `apiKey` dan `mapId` di repo ini, lalu ikuti persis. Buat `frontend/src/presentation/components/netmap/MapCanvas.tsx`:

```tsx
import { AdvancedMarker, APIProvider, Map } from "@vis.gl/react-google-maps";
import { useEffect, useRef } from "react";
import type {
  MappingEdge,
  MappingNode,
  NodeType,
  Waypoint,
} from "@/domain/entities";
import { NODE_COLORS } from "./mappingLabels";

// Where the map opens when there is nothing on it yet.
const FALLBACK_CENTER = { lat: -6.2, lng: 106.816 };
const FALLBACK_ZOOM = 13;

interface MapCanvasProps {
  nodes: MappingNode[];
  edges: MappingEdge[];
  draft: Waypoint[];
  placing: NodeType | "cable" | undefined;
  apiKey: string;
  mapId?: string;
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
}

/** Opens on what is already mapped, so a technician is never sent to the sea. */
function opening(nodes: MappingNode[]) {
  if (nodes.length === 0) {
    return { defaultCenter: FALLBACK_CENTER, defaultZoom: FALLBACK_ZOOM };
  }
  return {
    defaultCenter: { lat: nodes[0].latitude, lng: nodes[0].longitude },
    defaultZoom: 16,
  };
}

export function MapCanvas({
  nodes,
  edges,
  draft,
  placing,
  apiKey,
  mapId,
  onDrop,
  onNodeClick,
}: MapCanvasProps) {
  return (
    <APIProvider apiKey={apiKey} libraries={["marker"]}>
      <Map
        mapId={mapId}
        {...opening(nodes)}
        mapTypeId="satellite"
        style={{ width: "100%", height: "60vh" }}
        gestureHandling="greedy"
        disableDefaultUI={false}
        onClick={(event) => {
          const latLng = event.detail.latLng;
          if (!latLng || !placing) {
            return;
          }
          onDrop({ lat: latLng.lat, lng: latLng.lng });
        }}
      >
        {nodes.map((node) => (
          <AdvancedMarker
            key={node.nodeId}
            position={{ lat: node.latitude, lng: node.longitude }}
            title={node.name}
            onClick={() => onNodeClick(node.nodeId)}
          >
            <span
              style={{
                display: "block",
                width: 14,
                height: 14,
                borderRadius: "50%",
                background: NODE_COLORS[node.type],
                border: "2px solid #fff",
              }}
            />
          </AdvancedMarker>
        ))}
        <CableLines edges={edges} draft={draft} />
      </Map>
    </APIProvider>
  );
}

/**
 * Cables are drawn with the Maps Polyline API rather than a React component:
 * the library has no polyline of its own, and a path is a list of points the
 * map draws, not a thing that needs its own state.
 */
function CableLines({
  edges,
  draft,
}: {
  edges: MappingEdge[];
  draft: Waypoint[];
}) {
  const drawn = useRef<google.maps.Polyline[]>([]);

  useEffect(() => {
    const paths = edges
      .map((edge) => edge.waypoints ?? [])
      .filter((points) => points.length > 1);
    if (draft.length > 1) {
      paths.push(draft);
    }
    drawn.current.forEach((line) => line.setMap(null));
    drawn.current = paths.map(
      (path) =>
        new google.maps.Polyline({
          path: path.map((p) => ({ lat: p.lat, lng: p.lng })),
          strokeColor: "#f59e0b",
          strokeWeight: 3,
        }),
    );
    return () => drawn.current.forEach((line) => line.setMap(null));
  }, [edges, draft]);

  return null;
}
```

- [ ] **Step 6: Pastikan terkompilasi**

```bash
cd frontend && npx tsc --noEmit && npm run build
```
Expected: keduanya bersih.

- [ ] **Step 7: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation/components/netmap
git commit -m "feat(map): trace a cable corner by corner

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Halaman peta, kartu jumlah, daftar, dan rute

**Files:**
- Create: `frontend/src/presentation/components/netmap/CountCards.tsx`
- Create: `frontend/src/presentation/components/netmap/NodeList.tsx`
- Create: `frontend/src/presentation/pages/NetworkMapPage.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.tsx`
- Modify: `frontend/src/presentation/components/layout/navigationRoutes.test.tsx`
- Test: `frontend/src/presentation/pages/__tests__/NetworkMapPage.test.tsx`

**Interfaces:**
- Consumes: hook Task 7; `MapToolbar`, `NodeFormModal` (Task 9); `MapCanvas`, `useCableDraw` (Task 10); `NODE_LABELS`, `NODE_COLORS` (Task 8).
- Produces: rute `/network-map`, entri menu `Peta Jaringan`.

- [ ] **Step 1: Tulis tes yang gagal**

Buat `frontend/src/presentation/pages/__tests__/NetworkMapPage.test.tsx`:

```tsx
import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { NetworkMapPage } from "../NetworkMapPage";

vi.mock("@/application/hooks", () => ({
  useMappingNodes: () => ({
    data: [
      { nodeId: "ODC-01", type: "odc", name: "ODC Satu", latitude: -6.2, longitude: 106.8, capacity: 8 },
      { nodeId: "ODP-01", type: "odp", name: "ODP Satu", latitude: -6.21, longitude: 106.81, capacity: 8 },
      { nodeId: "ODP-02", type: "odp", name: "ODP Dua", latitude: -6.22, longitude: 106.82, capacity: 8 },
    ],
    isLoading: false,
  }),
  useMappingEdges: () => ({ data: [], isLoading: false }),
  useCreateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useGoogleMapsKey: () => ({ key: "test-key", mapId: "test-map", isLoading: false }),
}));

// The canvas needs a Google Maps key and a live network; the page's own job is
// counting and wiring, and that is what this test is about.
vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: () => <div data-testid="map-canvas" />,
}));

describe("NetworkMapPage", () => {
  it("counts what is on the map, by kind", () => {
    render(<NetworkMapPage />);

    expect(screen.getByTestId("count-odc")).toHaveTextContent("1");
    expect(screen.getByTestId("count-odp")).toHaveTextContent("2");
    expect(screen.getByTestId("count-ont")).toHaveTextContent("0");
  });

  it("offers the button that starts a cable", () => {
    render(<NetworkMapPage />);

    expect(
      screen.getByRole("button", { name: /Tarik kabel/ }),
    ).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Jalankan, pastikan gagal**

```bash
cd frontend && npx vitest run src/presentation/pages/__tests__/NetworkMapPage.test.tsx
```
Expected: FAIL — `Failed to resolve import "../NetworkMapPage"`.

- [ ] **Step 3: Tulis kartu jumlah**

Buat `frontend/src/presentation/components/netmap/CountCards.tsx`:

```tsx
import { Card, Col, Row, Statistic } from "antd";
import type { MappingNode, NodeType } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

const ORDER: NodeType[] = ["server", "odc", "odp", "ont"];

export function CountCards({ nodes }: { nodes: MappingNode[] }) {
  return (
    <Row gutter={[12, 12]}>
      {ORDER.map((type) => {
        const count = nodes.filter((n) => n.type === type).length;
        return (
          <Col key={type} xs={12} md={6}>
            <Card size="small">
              <Statistic
                title={NODE_LABELS[type]}
                value={count}
                valueStyle={{ color: NODE_COLORS[type] }}
              />
              <span data-testid={`count-${type}`} hidden>
                {count}
              </span>
            </Card>
          </Col>
        );
      })}
    </Row>
  );
}
```

- [ ] **Step 4: Tulis daftar**

Buat `frontend/src/presentation/components/netmap/NodeList.tsx`:

```tsx
import { Button, Table } from "antd";
import type { MappingNode } from "@/domain/entities";
import { NODE_LABELS } from "./mappingLabels";

interface NodeListProps {
  nodes: MappingNode[];
  onDelete: (nodeId: string) => void;
}

export function NodeList({ nodes, onDelete }: NodeListProps) {
  return (
    <Table
      rowKey="nodeId"
      dataSource={nodes}
      size="small"
      pagination={{ pageSize: 20 }}
      columns={[
        { title: "Kode", dataIndex: "nodeId" },
        { title: "Nama", dataIndex: "name" },
        {
          title: "Jenis",
          dataIndex: "type",
          render: (t: MappingNode["type"]) => NODE_LABELS[t],
        },
        { title: "Slot", dataIndex: "capacity" },
        {
          title: "",
          render: (_: unknown, node: MappingNode) => (
            <Button danger size="small" onClick={() => onDelete(node.nodeId)}>
              Hapus
            </Button>
          ),
        },
      ]}
    />
  );
}
```

- [ ] **Step 5: Tulis halaman**

Kunci Google Maps datang dari hook `useGoogleMapsKey()` di `@/application/hooks` — bukan dari env. Ia mengembalikan `{ key, mapId, isLoading }`, sama seperti yang dipakai `MapPage.tsx` baris 27. Buat `frontend/src/presentation/pages/NetworkMapPage.tsx`:

```tsx
import { useState } from "react";
import { Space, message } from "antd";
import type { MappingNode, NodeType, Waypoint } from "@/domain/entities";
import {
  useCreateEdge,
  useCreateNode,
  useDeleteNode,
  useGoogleMapsKey,
  useMappingEdges,
  useMappingNodes,
} from "@/application/hooks";
import { PageHeader } from "../components/common/PageHeader";
import { CountCards } from "../components/netmap/CountCards";
import { MapCanvas } from "../components/netmap/MapCanvas";
import { MapToolbar, type MapView } from "../components/netmap/MapToolbar";
import { NodeFormModal } from "../components/netmap/NodeFormModal";
import { NodeList } from "../components/netmap/NodeList";
import { useCableDraw } from "../components/netmap/useCableDraw";

export function NetworkMapPage() {
  const { data: nodes = [] } = useMappingNodes();
  const { data: edges = [] } = useMappingEdges();
  const createNode = useCreateNode();
  const deleteNode = useDeleteNode();
  const createEdge = useCreateEdge();
  const { key, mapId } = useGoogleMapsKey();
  const cable = useCableDraw();

  const [view, setView] = useState<MapView>("map");
  const [placing, setPlacing] = useState<NodeType | "cable">();
  const [dropped, setDropped] = useState<Waypoint>();

  const saveNode = async (node: MappingNode) => {
    await createNode.mutateAsync(node);
    setPlacing(undefined);
    setDropped(undefined);
  };

  // A tap on the map means "put it here" while placing, and "one more corner"
  // while a cable is being traced.
  const tapped = (point: Waypoint) => {
    if (placing === "cable") {
      cable.addPoint(point);
      return;
    }
    setDropped(point);
  };

  // The first node tapped starts the cable; the second ends it.
  const nodeTapped = async (nodeId: string) => {
    if (placing !== "cable") {
      return;
    }
    if (!cable.from) {
      cable.start(nodeId);
      return;
    }
    const traced = cable.finish();
    await createEdge.mutateAsync({
      edgeId: `${cable.from}--${nodeId}`,
      source: cable.from,
      target: nodeId,
      fiberType: "distribution",
      distance: Math.round(cable.meters),
      waypoints: traced,
      notes: "",
    });
    setPlacing(undefined);
    message.success("Kabel tersimpan");
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <PageHeader title="Peta Jaringan" />
      <MapToolbar
        placing={placing}
        onPlace={setPlacing}
        onDrawCable={() => setPlacing("cable")}
        onCancel={() => {
          setPlacing(undefined);
          setDropped(undefined);
          cable.cancel();
        }}
        view={view}
        onView={setView}
      />
      {view === "map" ? (
        <MapCanvas
          nodes={nodes}
          edges={edges}
          draft={cable.points}
          placing={placing}
          apiKey={key}
          mapId={mapId}
          onDrop={tapped}
          onNodeClick={nodeTapped}
        />
      ) : (
        <NodeList
          nodes={nodes}
          onDelete={(nodeId) => deleteNode.mutateAsync(nodeId)}
        />
      )}
      <CountCards nodes={nodes} />
      {dropped && placing && placing !== "cable" && (
        <NodeFormModal
          open
          type={placing}
          position={dropped}
          onCancel={() => setDropped(undefined)}
          onSubmit={saveNode}
        />
      )}
    </Space>
  );
}
```

- [ ] **Step 6: Daftarkan rute**

Buka `frontend/src/presentation/routes/index.tsx` dan lihat bagaimana `GraphsPage` didaftarkan — ia dimuat lazy supaya pustaka beratnya tidak ikut bundel utama. Tambahkan dengan pola yang persis sama:

```tsx
const NetworkMapPage = lazy(() =>
  import("../pages/NetworkMapPage").then((m) => ({ default: m.NetworkMapPage })),
);
```

dan di daftar rute, sebagai anak dari rute yang sama dengan `graphs`:

```tsx
        {
          path: "network-map",
          element: (
            <Suspense fallback={<Spin />}>
              <NetworkMapPage />
            </Suspense>
          ),
        },
```

- [ ] **Step 7: Daftarkan menu**

Di `frontend/src/presentation/components/layout/navigationRoutes.tsx`, di blok peran yang sama dengan entri peta lama, tambahkan:

```tsx
          {
            path: "/network-map",
            name: "Peta Jaringan",
          },
```

Lalu di `navigationRoutes.test.tsx`, tambahkan satu assert di dalam `describe` yang sudah ada:

```tsx
  it("offers the network map to the roles that may see plant", () => {
    const routes = buildNavigationRoutes("technician");
    const names = JSON.stringify(routes);
    expect(names).toContain("Peta Jaringan");
  });
```

Bila nama fungsi pembangun menu di berkas itu berbeda, pakai yang ada di sana — jangan mengubahnya.

- [ ] **Step 8: Jalankan tes, pastikan lulus**

```bash
cd frontend && npx vitest run src/presentation/ && npx tsc --noEmit && npm run build
```
Expected: semua PASS; tsc dan build bersih.

- [ ] **Step 9: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add frontend/src/presentation
git commit -m "feat(map): a map that counts what is on it

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Hapus peta lama dan verifikasi menyeluruh

**Files:**
- Delete: seluruh `frontend/src/presentation/components/map/`
- Modify: `frontend/src/presentation/pages/MapPage.tsx` (dihapus), `routes/index.tsx`, `navigationRoutes.tsx`

- [ ] **Step 1: Hapus peta lama**

```bash
cd frontend
rm -rf src/presentation/components/map
rm src/presentation/pages/MapPage.tsx
```
Lalu hapus rutenya di `routes/index.tsx` dan entri menunya di `navigationRoutes.tsx`.

- [ ] **Step 2: Gerbang frontend**

```bash
cd frontend && npx tsc --noEmit && npm run lint && npm run format:check && npm run build && npm test -- --run
```
Expected: semua bersih; tidak ada tes yang sebelumnya lulus kini gagal.

- [ ] **Step 3: Gerbang backend dengan Postgres**

```bash
docker run -d --rm --name tikman-test-pg -e POSTGRES_PASSWORD=test -e POSTGRES_DB=tikman_test -p 5439:5432 timescale/timescaledb:latest-pg15
docker exec tikman-test-pg sh -c 'until pg_isready -h 127.0.0.1 -U postgres -q; do sleep 1; done'
cd backend && TEST_POSTGRES_DSN="host=localhost port=5439 user=postgres password=test dbname=tikman_test sslmode=disable" go test ./... -race
docker stop tikman-test-pg
```
Expected: gagal hanya 30 tes WireGuard baseline; nol `DATA RACE`.

- [ ] **Step 4: Gerbang backend lainnya**

```bash
cd backend && gofmt -s -l . && go vet ./... && go build ./... && go mod verify
```

- [ ] **Step 5: Perbarui graphify**

```bash
cd /Users/rohadimraja/Documents/tikman && graphify update .
```

- [ ] **Step 6: Commit**

```bash
cd /Users/rohadimraja/Documents/tikman
git add -A frontend/
git commit -m "refactor(map): remove the map that asked too much

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
