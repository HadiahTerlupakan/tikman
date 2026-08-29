# Modul VPN WireGuard — Rencana Implementasi

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menjadikan VPS TikMan sebagai server WireGuard sehingga `api` dan `worker` dapat menjangkau OLT di site yang tidak punya IP publik.

**Architecture:** Interface `wg0` hidup di network namespace container `api`; `worker` bergabung ke namespace itu lewat `network_mode: service:api`. Database adalah sumber kebenaran, dan satu fungsi `Reconcile` menerapkan seluruh isi database ke kernel dalam satu operasi. Seluruh keputusan (validasi, alokasi, render konfigurasi, penilaian status) berada di kode murni yang diuji; lapisan yang menyentuh kernel dibuat setipis mungkin dan hanya dibangun untuk Linux.

**Tech Stack:** Go 1.25, Gin, GORM, PostgreSQL, `golang.zx2c4.com/wireguard/wgctrl`, `github.com/vishvananda/netlink`, React 18 + TypeScript + Ant Design + React Query.

**Spec:** `docs/superpowers/specs/2026-08-29-wireguard-vpn-module-design.md`

## Global Constraints

- Go 1.25.x, Node.js 24. Modul Go: `github.com/tikman/olt-provisioning`.
- Dua dependensi baru saja, versinya dipin: `golang.zx2c4.com/wireguard/wgctrl` dan `github.com/vishvananda/netlink`. Tidak boleh menambah yang lain.
- `github.com/vishvananda/netlink` hanya bisa dibangun di Linux. Setiap berkas yang mengimpornya wajib memakai build tag `//go:build linux` dan punya pasangan `//go:build !linux`, karena pengembangan berjalan di macOS.
- Tidak ada berkas melebihi 350 baris; tidak ada fungsi melebihi 50 baris; maksimum 3 tingkat indentasi. Pengecualian hanya untuk berkas yang menyentuh perangkat jaringan (`internal/connectivity/wireguard_device_linux.go`) dan berkas test.
- Kunci privat dienkripsi dengan `utils.Encrypt(plaintext, key)` dan dibaca dengan `utils.Decrypt(ciphertext, key)`; kunci enkripsi berasal dari `cfg.EncryptionKey` dan panjangnya 32 byte.
- Tipe kolom: alamat IP sebagai `varchar(45)`, daftar sebagai `jsonb` melalui `datatypes.JSON`. Jangan memakai `inet`, `cidr`, atau `text[]`: test backend memakai SQLite in-memory melalui `models.AutoMigrate`.
- Perubahan skema ditulis sebagai migration SQL bernomor di `backend/migrations/`, dan modelnya juga didaftarkan di `models.AutoMigrate` agar test SQLite punya tabelnya.
- Komentar menjelaskan alasan, bukan mengulang kode. Pesan error dan komentar ditulis dalam bahasa Inggris, mengikuti kode yang ada.
- Setiap task diakhiri commit. Sebelum commit: `cd backend && gofmt -s -l . && go vet ./... && go test ./... -race`; untuk task frontend: `cd frontend && npm run lint && npm run format:check && npm test -- --run && npm run build`.

## Struktur berkas

Backend, dibuat baru:

| Berkas | Tanggung jawab |
|---|---|
| `internal/models/wireguard.go` | Dua model dan helper daftar `allowed_ips`. |
| `migrations/25_add_wireguard.sql` | Skema PostgreSQL. |
| `internal/services/wireguard_validate.go` | Seluruh aturan validasi CIDR dan alamat. |
| `internal/services/wireguard_alloc.go` | Alokasi alamat tunnel dan saran subnet dari alamat OLT. |
| `internal/services/wireguard_render.go` | Generator konfigurasi wg-quick dan MikroTik. |
| `internal/services/wireguard_status.go` | Aturan "terhubung" dan goroutine pembaruan status. |
| `internal/services/wireguard_service.go` | CRUD, orkestrasi, dan `Reconcile`. |
| `internal/connectivity/wireguard_device.go` | Definisi tipe dan interface `TunnelDevice`. |
| `internal/connectivity/wireguard_device_memory.go` | Implementasi in-memory yang dipakai test. |
| `internal/connectivity/wireguard_device_linux.go` | Netlink dan wgctrl. Hanya Linux. |
| `internal/connectivity/wireguard_device_other.go` | Pengganti non-Linux yang mengembalikan error. |
| `internal/api/wireguard_dto.go` | Request dan response. |
| `internal/api/wireguard_handler.go` | Handler HTTP. |
| `cmd/worker/wireguard_gate.go` | Penentuan OLT yang tunnelnya mati. |

Backend, diubah: `internal/models/models.go`, `internal/api/router.go`, `internal/api/test_helpers.go`, `cmd/api/main.go`, `cmd/worker/main.go`, `docker-compose.yml`.

Frontend, dibuat baru: `src/domain/entities/Wireguard.ts`, `src/infrastructure/repositories/WireguardRepository.ts`, `src/application/hooks/useWireguard.ts`, `src/presentation/pages/VpnPage.tsx`, `src/presentation/pages/vpn/VpnServerCard.tsx`, `src/presentation/pages/vpn/VpnPeerFormModal.tsx`, `src/presentation/pages/vpn/VpnConfigModal.tsx`.

Frontend, diubah: `src/domain/entities/index.ts`, `src/infrastructure/repositories/index.ts`, `src/infrastructure/http/endpoints.ts`, `src/application/hooks/index.ts`, `src/presentation/routes/index.tsx`, `src/presentation/components/layout/Sidebar.tsx`.

---

### Task 1: Model dan migration

**Files:**
- Create: `backend/internal/models/wireguard.go`
- Create: `backend/migrations/25_add_wireguard.sql`
- Modify: `backend/internal/models/models.go`
- Test: `backend/internal/models/wireguard_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces: `models.WireGuardServer`, `models.WireGuardPeer`, `(*WireGuardPeer).AllowedIPsList() ([]string, error)`, `(*WireGuardPeer).SetAllowedIPs([]string) error`.

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/models/wireguard_test.go`:

```go
package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWireGuardTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestWireGuardPeerAllowedIPsRoundTrip(t *testing.T) {
	db := newWireGuardTestDB(t)

	peer := &WireGuardPeer{
		SiteID:        uuid.New(),
		Name:          "Site A",
		PublicKey:     "pub",
		PrivateKey:    "enc",
		TunnelAddress: "10.88.0.5",
	}
	require.NoError(t, peer.SetAllowedIPs([]string{"10.10.10.0/24", "192.168.88.0/24"}))
	require.NoError(t, db.Create(peer).Error)
	require.NotEqual(t, uuid.Nil, peer.ID, "BeforeCreate must assign an ID")

	var loaded WireGuardPeer
	require.NoError(t, db.First(&loaded, "id = ?", peer.ID).Error)

	list, err := loaded.AllowedIPsList()
	require.NoError(t, err)
	require.Equal(t, []string{"10.10.10.0/24", "192.168.88.0/24"}, list)
}

func TestWireGuardPeerAllowedIPsEmptyWhenUnset(t *testing.T) {
	peer := &WireGuardPeer{}
	list, err := peer.AllowedIPsList()
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestWireGuardServerDefaultsPersist(t *testing.T) {
	db := newWireGuardTestDB(t)

	server := &WireGuardServer{
		InterfaceName: "wg0",
		ListenPort:    51820,
		PrivateKey:    "enc",
		PublicKey:     "pub",
		EndpointHost:  "vpn.example.id",
		TunnelSubnet:  "10.88.0.0/24",
		Address:       "10.88.0.1",
	}
	require.NoError(t, db.Create(server).Error)

	var loaded WireGuardServer
	require.NoError(t, db.First(&loaded, "id = ?", server.ID).Error)
	require.Equal(t, "10.88.0.0/24", loaded.TunnelSubnet)
	require.Equal(t, 51820, loaded.ListenPort)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/models/ -run TestWireGuard -v`
Expected: FAIL, kompilasi gagal dengan `undefined: WireGuardPeer`.

- [ ] **Step 3: Tulis model**

Buat `backend/internal/models/wireguard.go`:

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WireGuardServer is the VPS side of the tunnel. Exactly one row exists. The
// service generates the keypair itself, so a private key never arrives from
// user input.
type WireGuardServer struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	InterfaceName string    `gorm:"type:varchar(15);not null;uniqueIndex;default:wg0"`
	ListenPort    int       `gorm:"not null;default:51820"`
	PrivateKey    string    `gorm:"type:text;not null"` // encrypted
	PublicKey     string    `gorm:"type:text;not null"`
	EndpointHost  string    `gorm:"type:varchar(255);not null"`
	TunnelSubnet  string    `gorm:"type:varchar(45);not null;default:10.88.0.0/24"`
	Address       string    `gorm:"type:varchar(45);not null;default:10.88.0.1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s *WireGuardServer) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (s *WireGuardServer) TableName() string {
	return "wireguard_server"
}

// WireGuardPeer is one site. It carries no endpoint: sites sit behind NAT and
// are the side that initiates, so the server learns their address from the
// handshake.
type WireGuardPeer struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey"`
	SiteID              uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex"`
	Name                string         `gorm:"type:varchar(255);not null"`
	PublicKey           string         `gorm:"type:text;not null"`
	PrivateKey          string         `gorm:"type:text;not null"` // encrypted
	PresharedKey        string         `gorm:"type:text"`          // encrypted, optional
	TunnelAddress       string         `gorm:"type:varchar(45);not null;uniqueIndex"`
	AllowedIPs          datatypes.JSON `gorm:"column:allowed_ips;type:jsonb;not null"`
	PersistentKeepalive int            `gorm:"not null;default:25"`
	Enabled             bool           `gorm:"not null;default:true"`
	LastHandshakeAt     *time.Time
	RxBytes             int64 `gorm:"not null;default:0"`
	TxBytes             int64 `gorm:"not null;default:0"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (p *WireGuardPeer) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (p *WireGuardPeer) TableName() string {
	return "wireguard_peers"
}

// AllowedIPsList decodes the stored subnets. An unset column reads as an empty
// list rather than an error, because a peer row is written before its subnets
// are known only in tests.
func (p *WireGuardPeer) AllowedIPsList() ([]string, error) {
	if len(p.AllowedIPs) == 0 {
		return nil, nil
	}
	var list []string
	if err := json.Unmarshal(p.AllowedIPs, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (p *WireGuardPeer) SetAllowedIPs(list []string) error {
	encoded, err := json.Marshal(list)
	if err != nil {
		return err
	}
	p.AllowedIPs = encoded
	return nil
}
```

- [ ] **Step 4: Daftarkan di AutoMigrate**

Di `backend/internal/models/models.go`, tambahkan dua baris terakhir sebelum penutup:

```go
		&BatchJob{},
		&WireGuardServer{},
		&WireGuardPeer{},
	)
```

- [ ] **Step 5: Tulis migration SQL**

Buat `backend/migrations/25_add_wireguard.sql`:

```sql
CREATE TABLE IF NOT EXISTS wireguard_server (
    id UUID PRIMARY KEY,
    interface_name VARCHAR(15) NOT NULL DEFAULT 'wg0' UNIQUE,
    listen_port INTEGER NOT NULL DEFAULT 51820,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    endpoint_host VARCHAR(255) NOT NULL,
    tunnel_subnet VARCHAR(45) NOT NULL DEFAULT '10.88.0.0/24',
    address VARCHAR(45) NOT NULL DEFAULT '10.88.0.1',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS wireguard_peers (
    id UUID PRIMARY KEY,
    site_id UUID NOT NULL UNIQUE REFERENCES sites(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,
    preshared_key TEXT,
    tunnel_address VARCHAR(45) NOT NULL UNIQUE,
    allowed_ips JSONB NOT NULL DEFAULT '[]'::jsonb,
    persistent_keepalive INTEGER NOT NULL DEFAULT 25,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_handshake_at TIMESTAMPTZ,
    rx_bytes BIGINT NOT NULL DEFAULT 0,
    tx_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wireguard_peers_enabled ON wireguard_peers(enabled);
```

`ON DELETE RESTRICT` disengaja: menghapus site yang masih punya tunnel harus gagal dengan jelas, bukan diam-diam memutus akses.

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/models/ -run TestWireGuard -v`
Expected: PASS, tiga test.

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/models/ -race
git add backend/internal/models/wireguard.go backend/internal/models/wireguard_test.go backend/internal/models/models.go backend/migrations/25_add_wireguard.sql
git commit -m "feat(vpn): add WireGuard server and peer models"
```

---

### Task 2: Validasi jaringan

**Files:**
- Create: `backend/internal/services/wireguard_validate.go`
- Test: `backend/internal/services/wireguard_validate_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces:
  - `type PeerNetwork struct { PeerID uuid.UUID; SiteName string; AllowedIPs []string }`
  - `func ValidateAllowedIPs(candidate []string, others []PeerNetwork, tunnelSubnet string, reserved []string) error`
  - `func ValidateTunnelAddress(address, tunnelSubnet, serverAddress string, taken []string) error`
  - `func ValidateKeepalive(seconds int) error`
  - `var DefaultReservedSubnets = []string{"172.16.0.0/12"}`

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/services/wireguard_validate_test.go`:

```go
package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateAllowedIPsRejectsDefaultRoute(t *testing.T) {
	err := ValidateAllowedIPs([]string{"0.0.0.0/0"}, nil, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "default route")
}

func TestValidateAllowedIPsRejectsOverlapWithAnotherSite(t *testing.T) {
	others := []PeerNetwork{
		{PeerID: uuid.New(), SiteName: "Site Bandung", AllowedIPs: []string{"192.168.1.0/24"}},
	}
	err := ValidateAllowedIPs([]string{"192.168.1.128/25"}, others, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Site Bandung",
		"the message must name the conflicting site, since identical private subnets across sites are the common case")
}

func TestValidateAllowedIPsRejectsOverlapWithTunnelSubnet(t *testing.T) {
	err := ValidateAllowedIPs([]string{"10.88.0.0/25"}, nil, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tunnel subnet")
}

func TestValidateAllowedIPsRejectsOverlapWithReservedSubnet(t *testing.T) {
	err := ValidateAllowedIPs([]string{"172.18.0.0/16"}, nil, "10.88.0.0/24", []string{"172.16.0.0/12"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
}

func TestValidateAllowedIPsRejectsEmptyAndMalformed(t *testing.T) {
	require.Error(t, ValidateAllowedIPs(nil, nil, "10.88.0.0/24", nil))
	require.Error(t, ValidateAllowedIPs([]string{"10.10.10.5"}, nil, "10.88.0.0/24", nil))
}

func TestValidateAllowedIPsAcceptsDistinctSubnets(t *testing.T) {
	others := []PeerNetwork{
		{PeerID: uuid.New(), SiteName: "Site Bandung", AllowedIPs: []string{"192.168.1.0/24"}},
	}
	require.NoError(t, ValidateAllowedIPs([]string{"10.10.10.0/24", "192.168.88.0/24"}, others, "10.88.0.0/24", nil))
}

func TestValidateTunnelAddress(t *testing.T) {
	require.NoError(t, ValidateTunnelAddress("10.88.0.5", "10.88.0.0/24", "10.88.0.1", []string{"10.88.0.4"}))

	err := ValidateTunnelAddress("10.99.0.5", "10.88.0.0/24", "10.88.0.1", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside")

	err = ValidateTunnelAddress("10.88.0.1", "10.88.0.0/24", "10.88.0.1", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "server")

	err = ValidateTunnelAddress("10.88.0.4", "10.88.0.0/24", "10.88.0.1", []string{"10.88.0.4"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already")
}

func TestValidateKeepalive(t *testing.T) {
	require.NoError(t, ValidateKeepalive(25))
	require.Error(t, ValidateKeepalive(5))
	require.Error(t, ValidateKeepalive(200))
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run "TestValidateAllowedIPs|TestValidateTunnelAddress|TestValidateKeepalive" -v`
Expected: FAIL, `undefined: ValidateAllowedIPs`.

- [ ] **Step 3: Tulis implementasi**

Buat `backend/internal/services/wireguard_validate.go`:

```go
package services

import (
	"fmt"
	"net"

	"github.com/google/uuid"
)

const (
	minKeepaliveSeconds = 10
	maxKeepaliveSeconds = 120
)

// DefaultReservedSubnets covers the Docker bridge range. Routing it into a
// tunnel would cut the API off from postgres and redis.
var DefaultReservedSubnets = []string{"172.16.0.0/12"}

// PeerNetwork is the subset of an existing peer that validation needs. The site
// name travels with it so a conflict can be reported by name.
type PeerNetwork struct {
	PeerID     uuid.UUID
	SiteName   string
	AllowedIPs []string
}

func ValidateAllowedIPs(candidate []string, others []PeerNetwork, tunnelSubnet string, reserved []string) error {
	if len(candidate) == 0 {
		return fmt.Errorf("at least one local subnet is required")
	}

	parsed := make([]*net.IPNet, 0, len(candidate))
	for _, entry := range candidate {
		network, err := parseCIDR(entry)
		if err != nil {
			return err
		}
		if ones, bits := network.Mask.Size(); ones == 0 && bits > 0 {
			return fmt.Errorf("%s is a default route: it would send all VPS traffic into one site", entry)
		}
		parsed = append(parsed, network)
	}

	if err := rejectOverlapWith(parsed, []string{tunnelSubnet}, "tunnel subnet"); err != nil {
		return err
	}
	if err := rejectOverlapWith(parsed, reserved, "reserved subnet"); err != nil {
		return err
	}
	return rejectOverlapWithPeers(parsed, others)
}

func rejectOverlapWith(candidate []*net.IPNet, subnets []string, label string) error {
	for _, subnet := range subnets {
		network, err := parseCIDR(subnet)
		if err != nil {
			return err
		}
		for _, entry := range candidate {
			if networksOverlap(entry, network) {
				return fmt.Errorf("%s overlaps the %s %s", entry.String(), label, subnet)
			}
		}
	}
	return nil
}

func rejectOverlapWithPeers(candidate []*net.IPNet, others []PeerNetwork) error {
	for _, other := range others {
		for _, entry := range other.AllowedIPs {
			network, err := parseCIDR(entry)
			if err != nil {
				continue // a stored value that no longer parses cannot be routed anyway
			}
			for _, own := range candidate {
				if networksOverlap(own, network) {
					return fmt.Errorf("%s overlaps %s, already used by site %s", own.String(), entry, other.SiteName)
				}
			}
		}
	}
	return nil
}

func ValidateTunnelAddress(address, tunnelSubnet, serverAddress string, taken []string) error {
	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("%q is not a valid IP address", address)
	}

	network, err := parseCIDR(tunnelSubnet)
	if err != nil {
		return err
	}
	if !network.Contains(ip) {
		return fmt.Errorf("%s is outside the tunnel subnet %s", address, tunnelSubnet)
	}
	if address == serverAddress {
		return fmt.Errorf("%s is the server address", address)
	}
	for _, used := range taken {
		if used == address {
			return fmt.Errorf("%s is already assigned to another site", address)
		}
	}
	return nil
}

func ValidateKeepalive(seconds int) error {
	if seconds < minKeepaliveSeconds || seconds > maxKeepaliveSeconds {
		return fmt.Errorf("keepalive must be between %d and %d seconds", minKeepaliveSeconds, maxKeepaliveSeconds)
	}
	return nil
}

func parseCIDR(value string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid subnet in CIDR form, for example 10.10.10.0/24", value)
	}
	return network, nil
}

func networksOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run "TestValidateAllowedIPs|TestValidateTunnelAddress|TestValidateKeepalive" -v`
Expected: PASS, delapan test.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/services/ -run TestValidate -race
git add backend/internal/services/wireguard_validate.go backend/internal/services/wireguard_validate_test.go
git commit -m "feat(vpn): validate peer subnets and tunnel addresses"
```

---

### Task 3: Alokasi alamat dan saran subnet

**Files:**
- Create: `backend/internal/services/wireguard_alloc.go`
- Test: `backend/internal/services/wireguard_alloc_test.go`

**Interfaces:**
- Consumes: `parseCIDR` dari Task 2.
- Produces:
  - `func AllocateTunnelAddress(tunnelSubnet, serverAddress string, taken []string) (string, error)`
  - `func SuggestAllowedIPs(oltAddresses []string) []string`

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/services/wireguard_alloc_test.go`:

```go
package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocateTunnelAddressTakesFirstFree(t *testing.T) {
	address, err := AllocateTunnelAddress("10.88.0.0/24", "10.88.0.1", nil)
	require.NoError(t, err)
	require.Equal(t, "10.88.0.2", address)
}

func TestAllocateTunnelAddressFillsGaps(t *testing.T) {
	address, err := AllocateTunnelAddress("10.88.0.0/24", "10.88.0.1", []string{"10.88.0.2", "10.88.0.4"})
	require.NoError(t, err)
	require.Equal(t, "10.88.0.3", address, "a released address must be reused before the range grows")
}

func TestAllocateTunnelAddressFailsWhenSubnetIsFull(t *testing.T) {
	taken := []string{}
	for i := 2; i <= 6; i++ {
		taken = append(taken, fmt.Sprintf("10.88.0.%d", i))
	}
	_, err := AllocateTunnelAddress("10.88.0.0/29", "10.88.0.1", taken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no free address")
}

func TestSuggestAllowedIPsDerivesSlash24(t *testing.T) {
	require.Equal(t, []string{"10.10.10.0/24"}, SuggestAllowedIPs([]string{"10.10.10.5"}))
}

func TestSuggestAllowedIPsDeduplicates(t *testing.T) {
	got := SuggestAllowedIPs([]string{"10.10.10.5", "10.10.10.9", "192.168.88.2"})
	require.Equal(t, []string{"10.10.10.0/24", "192.168.88.0/24"}, got)
}

func TestSuggestAllowedIPsIgnoresUnusableAddresses(t *testing.T) {
	require.Empty(t, SuggestAllowedIPs([]string{"", "not-an-ip"}))
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run "TestAllocateTunnelAddress|TestSuggestAllowedIPs" -v`
Expected: FAIL, `undefined: AllocateTunnelAddress`.

- [ ] **Step 3: Tulis implementasi**

Buat `backend/internal/services/wireguard_alloc.go`:

```go
package services

import (
	"fmt"
	"net"
	"sort"
)

// AllocateTunnelAddress returns the lowest address in the tunnel subnet that is
// neither the server's nor already assigned. Gaps are reused so a deleted site
// does not push the range upward forever.
func AllocateTunnelAddress(tunnelSubnet, serverAddress string, taken []string) (string, error) {
	network, err := parseCIDR(tunnelSubnet)
	if err != nil {
		return "", err
	}

	used := make(map[string]bool, len(taken)+1)
	used[serverAddress] = true
	for _, address := range taken {
		used[address] = true
	}

	for candidate := nextIP(network.IP); network.Contains(candidate); candidate = nextIP(candidate) {
		if isBroadcast(candidate, network) {
			break
		}
		if !used[candidate.String()] {
			return candidate.String(), nil
		}
	}
	return "", fmt.Errorf("no free address left in tunnel subnet %s", tunnelSubnet)
}

// SuggestAllowedIPs turns the OLT addresses already registered for a site into
// /24 subnets. It is a suggestion the operator confirms, not a discovery: the
// real prefix length is only known at the site.
func SuggestAllowedIPs(oltAddresses []string) []string {
	seen := make(map[string]bool)
	for _, address := range oltAddresses {
		ip := net.ParseIP(address).To4()
		if ip == nil {
			continue
		}
		masked := ip.Mask(net.CIDRMask(24, 32))
		seen[fmt.Sprintf("%s/24", masked.String())] = true
	}

	subnets := make([]string, 0, len(seen))
	for subnet := range seen {
		subnets = append(subnets, subnet)
	}
	sort.Strings(subnets)
	return subnets
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}

func isBroadcast(ip net.IP, network *net.IPNet) bool {
	broadcast := make(net.IP, len(network.IP))
	for i := range network.IP {
		broadcast[i] = network.IP[i] | ^network.Mask[i]
	}
	return ip.Equal(broadcast)
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run "TestAllocateTunnelAddress|TestSuggestAllowedIPs" -v`
Expected: PASS, enam test.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/services/ -run "TestAllocate|TestSuggest" -race
git add backend/internal/services/wireguard_alloc.go backend/internal/services/wireguard_alloc_test.go
git commit -m "feat(vpn): allocate tunnel addresses and suggest site subnets"
```

---

### Task 4: Generator konfigurasi sisi site

**Files:**
- Create: `backend/internal/services/wireguard_render.go`
- Test: `backend/internal/services/wireguard_render_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces:
  - `type PeerConfigInput struct { PeerPrivateKey, PeerAddress, ServerPublicKey, EndpointHost, TunnelSubnet string; ListenPort, Keepalive int; AllowedIPs []string }`
  - `func RenderWGQuickConfig(in PeerConfigInput) string`
  - `func RenderMikroTikConfig(in PeerConfigInput) string`
  - `const MikroTikInterfaceName = "wg-tikman"`

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/services/wireguard_render_test.go`:

```go
package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func renderInput() PeerConfigInput {
	return PeerConfigInput{
		PeerPrivateKey:  "PEERPRIV",
		PeerAddress:     "10.88.0.5",
		ServerPublicKey: "SERVERPUB",
		EndpointHost:    "vpn.contoh.id",
		TunnelSubnet:    "10.88.0.0/24",
		ListenPort:      51820,
		Keepalive:       25,
		AllowedIPs:      []string{"10.10.10.0/24"},
	}
}

func TestRenderWGQuickConfig(t *testing.T) {
	expected := `[Interface]
PrivateKey = PEERPRIV
Address = 10.88.0.5/24
PostUp = iptables -t nat -A POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE
PostDown = iptables -t nat -D POSTROUTING -s 10.88.0.0/24 -d 10.10.10.0/24 -j MASQUERADE

[Peer]
PublicKey = SERVERPUB
Endpoint = vpn.contoh.id:51820
AllowedIPs = 10.88.0.0/24
PersistentKeepalive = 25
`
	require.Equal(t, expected, RenderWGQuickConfig(renderInput()))
}

func TestRenderWGQuickConfigOneRulePerSubnet(t *testing.T) {
	in := renderInput()
	in.AllowedIPs = []string{"10.10.10.0/24", "192.168.88.0/24"}

	output := RenderWGQuickConfig(in)
	require.Equal(t, 2, strings.Count(output, "PostUp ="))
	require.Equal(t, 2, strings.Count(output, "PostDown ="))
	require.Contains(t, output, "-d 192.168.88.0/24 -j MASQUERADE")
}

func TestRenderWGQuickConfigNeverTunnelsAllTraffic(t *testing.T) {
	output := RenderWGQuickConfig(renderInput())
	require.NotContains(t, output, "0.0.0.0/0",
		"the site must keep its own internet path; only the tunnel subnet is routed to the VPS")
}

func TestRenderMikroTikConfig(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())

	require.Contains(t, output, `/interface/wireguard/add name=wg-tikman private-key="PEERPRIV" listen-port=13231`)
	require.Contains(t, output, "/ip/address/add address=10.88.0.5/24 interface=wg-tikman")
	require.Contains(t, output, `public-key="SERVERPUB"`)
	require.Contains(t, output, "endpoint-address=vpn.contoh.id endpoint-port=51820")
	require.Contains(t, output, "allowed-address=10.88.0.0/24")
	require.Contains(t, output, "persistent-keepalive=25s")
	require.Contains(t, output, "chain=srcnat src-address=10.88.0.0/24 dst-address=10.10.10.0/24 action=masquerade")
}

func TestRenderMikroTikConfigNeedsNoInterfaceName(t *testing.T) {
	output := RenderMikroTikConfig(renderInput())
	require.NotContains(t, output, "out-interface=",
		"the NAT rule must not ask the operator to know the name of the LAN interface")
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestRender -v`
Expected: FAIL, `undefined: PeerConfigInput`.

- [ ] **Step 3: Tulis implementasi**

Buat `backend/internal/services/wireguard_render.go`:

```go
package services

import (
	"fmt"
	"strings"
)

// MikroTikInterfaceName is the interface the generated MikroTik commands create.
// It is fixed so a regenerated config overwrites the same interface instead of
// leaving a second one behind.
const MikroTikInterfaceName = "wg-tikman"

// mikroTikListenPort is RouterOS's own default. The site never receives an
// inbound handshake, so the value only has to be free.
const mikroTikListenPort = 13231

// PeerConfigInput carries everything the site side needs. The private key is
// the peer's own, decrypted only while rendering.
type PeerConfigInput struct {
	PeerPrivateKey  string
	PeerAddress     string
	ServerPublicKey string
	EndpointHost    string
	TunnelSubnet    string
	ListenPort      int
	Keepalive       int
	AllowedIPs      []string
}

func RenderWGQuickConfig(in PeerConfigInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\nAddress = %s\n", in.PeerPrivateKey, addressWithSubnetPrefix(in.PeerAddress, in.TunnelSubnet))
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "PostUp = iptables -t nat -A POSTROUTING -s %s -d %s -j MASQUERADE\n", in.TunnelSubnet, subnet)
	}
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "PostDown = iptables -t nat -D POSTROUTING -s %s -d %s -j MASQUERADE\n", in.TunnelSubnet, subnet)
	}
	fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\nEndpoint = %s:%d\nAllowedIPs = %s\nPersistentKeepalive = %d\n",
		in.ServerPublicKey, in.EndpointHost, in.ListenPort, in.TunnelSubnet, in.Keepalive)

	return b.String()
}

func RenderMikroTikConfig(in PeerConfigInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "/interface/wireguard/add name=%s private-key=%q listen-port=%d\n",
		MikroTikInterfaceName, in.PeerPrivateKey, mikroTikListenPort)
	fmt.Fprintf(&b, "/ip/address/add address=%s interface=%s\n",
		addressWithSubnetPrefix(in.PeerAddress, in.TunnelSubnet), MikroTikInterfaceName)
	fmt.Fprintf(&b, "/interface/wireguard/peers/add interface=%s public-key=%q endpoint-address=%s endpoint-port=%d allowed-address=%s persistent-keepalive=%ds\n",
		MikroTikInterfaceName, in.ServerPublicKey, in.EndpointHost, in.ListenPort, in.TunnelSubnet, in.Keepalive)

	// Source NAT written without an interface name: the operator would have to
	// look up the LAN interface otherwise, and without it the OLT needs a route
	// back to the tunnel subnet that it almost never has.
	for _, subnet := range in.AllowedIPs {
		fmt.Fprintf(&b, "/ip/firewall/nat/add chain=srcnat src-address=%s dst-address=%s action=masquerade comment=\"TikMan VPN\"\n",
			in.TunnelSubnet, subnet)
	}

	return b.String()
}

func addressWithSubnetPrefix(address, subnet string) string {
	parts := strings.SplitN(subnet, "/", 2)
	if len(parts) != 2 {
		return address
	}
	return fmt.Sprintf("%s/%s", address, parts[1])
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestRender -v`
Expected: PASS, lima test.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/services/ -run TestRender -race
git add backend/internal/services/wireguard_render.go backend/internal/services/wireguard_render_test.go
git commit -m "feat(vpn): render site configs for wg-quick and MikroTik"
```

---

### Task 5: Aturan status peer

**Files:**
- Create: `backend/internal/services/wireguard_status.go`
- Test: `backend/internal/services/wireguard_status_test.go`

**Interfaces:**
- Consumes: tidak ada.
- Produces:
  - `const PeerHandshakeGrace = 3 * time.Minute`
  - `func PeerConnected(lastHandshake *time.Time, now time.Time) bool`

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/services/wireguard_status_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPeerConnected(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)

	recent := now.Add(-90 * time.Second)
	require.True(t, PeerConnected(&recent, now))

	stale := now.Add(-10 * time.Minute)
	require.False(t, PeerConnected(&stale, now))

	require.False(t, PeerConnected(nil, now), "a peer that never handshook is not connected")
}

func TestPeerConnectedToleratesOneMissedRehandshake(t *testing.T) {
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	// WireGuard rehandshakes about every two minutes, so a peer seen 150s ago
	// is healthy and must not be reported as down.
	seen := now.Add(-150 * time.Second)
	require.True(t, PeerConnected(&seen, now))
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run TestPeerConnected -v`
Expected: FAIL, `undefined: PeerConnected`.

- [ ] **Step 3: Tulis implementasi**

Buat `backend/internal/services/wireguard_status.go`:

```go
package services

import "time"

// PeerHandshakeGrace is how long after the last handshake a peer still counts as
// connected. WireGuard rehandshakes roughly every two minutes, so this leaves
// room for one missed exchange before a healthy site is called down.
const PeerHandshakeGrace = 3 * time.Minute

func PeerConnected(lastHandshake *time.Time, now time.Time) bool {
	if lastHandshake == nil || lastHandshake.IsZero() {
		return false
	}
	return now.Sub(*lastHandshake) < PeerHandshakeGrace
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run TestPeerConnected -v`
Expected: PASS, dua test.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/services/ -run TestPeerConnected -race
git add backend/internal/services/wireguard_status.go backend/internal/services/wireguard_status_test.go
git commit -m "feat(vpn): decide peer liveness from the last handshake"
```

---

### Task 6: Service, interface perangkat, dan rekonsiliasi

**Files:**
- Create: `backend/internal/connectivity/wireguard_device.go`
- Create: `backend/internal/connectivity/wireguard_device_memory.go`
- Create: `backend/internal/services/wireguard_service.go`
- Test: `backend/internal/services/wireguard_service_test.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Consumes: seluruh fungsi dari Task 2 sampai 5, dan model dari Task 1.
- Produces:
  - Dalam paket `connectivity`: `TunnelPeerConfig{PublicKey, PresharedKey string; AllowedIPs []string; Keepalive time.Duration}`, `TunnelConfig{InterfaceName, PrivateKey, Address string; ListenPort int; Peers []TunnelPeerConfig}`, `TunnelPeerStatus{PublicKey string; LastHandshakeAt *time.Time; RxBytes, TxBytes int64}`, `TunnelDevice interface { Apply(TunnelConfig) error; Status(interfaceName string) ([]TunnelPeerStatus, error) }`, dan `MemoryTunnelDevice{Applied TunnelConfig; Statuses []TunnelPeerStatus; ApplyErr error}`.
  - `func NewWireGuardService(db *gorm.DB, encryptionKey string, device connectivity.TunnelDevice) *WireGuardService`
  - `func (s *WireGuardService) EnsureServer(endpointHost string) (*models.WireGuardServer, error)`
  - `func (s *WireGuardService) GetServer() (*models.WireGuardServer, error)`
  - `func (s *WireGuardService) UpdateServer(endpointHost string, listenPort int) (*models.WireGuardServer, error)`
  - `func (s *WireGuardService) ListPeers() ([]models.WireGuardPeer, error)`
  - `func (s *WireGuardService) CreatePeer(siteID uuid.UUID, name string, allowedIPs []string, tunnelAddress string) (*models.WireGuardPeer, error)`
  - `func (s *WireGuardService) UpdatePeer(id uuid.UUID, name *string, allowedIPs []string, enabled *bool) (*models.WireGuardPeer, error)`
  - `func (s *WireGuardService) DeletePeer(id uuid.UUID) error`
  - `func (s *WireGuardService) PeerConfig(id uuid.UUID, format string) (string, error)`
  - `func (s *WireGuardService) SuggestAllowedIPsForSite(siteID uuid.UUID) ([]string, error)`
  - `func (s *WireGuardService) Reconcile() error`
  - `func (s *WireGuardService) RefreshStatus() error`

- [ ] **Step 1: Tambahkan dependensi wgctrl**

```bash
cd backend
go get golang.zx2c4.com/wireguard/wgctrl@v0.0.0-20230429144221-925a1e7659e6
go mod tidy
```

Hanya paket `wgtypes` yang dipakai di task ini, dan paket itu murni Go sehingga test tetap jalan di macOS.

- [ ] **Step 2: Tulis test yang gagal**

Buat `backend/internal/services/wireguard_service_test.go`:

```go
package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const wgTestEncryptionKey = "12345678901234567890123456789012"

func newWireGuardService(t *testing.T) (*WireGuardService, *connectivity.MemoryTunnelDevice, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	device := &connectivity.MemoryTunnelDevice{}
	return NewWireGuardService(db, wgTestEncryptionKey, device), device, db
}

func createTestSite(t *testing.T, db *gorm.DB, name string) models.Site {
	site := models.Site{Name: name}
	require.NoError(t, db.Create(&site).Error)
	return site
}

func TestEnsureServerGeneratesKeypairOnce(t *testing.T) {
	service, _, _ := newWireGuardService(t)

	first, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	require.NotEmpty(t, first.PublicKey)
	require.NotEmpty(t, first.PrivateKey)
	require.NotEqual(t, first.PrivateKey, first.PublicKey)

	second, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.PublicKey, second.PublicKey, "an existing server keypair must never be regenerated")
}

func TestCreatePeerAllocatesAddressAndAppliesToDevice(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)
	require.Equal(t, "10.88.0.2", peer.TunnelAddress)
	require.NotEmpty(t, peer.PublicKey)

	require.Len(t, device.Applied.Peers, 1)
	require.Equal(t, peer.PublicKey, device.Applied.Peers[0].PublicKey)
	require.Equal(t, []string{"10.10.10.0/24"}, device.Applied.Peers[0].AllowedIPs)
	require.Equal(t, "10.88.0.1/24", device.Applied.Address)
}

func TestCreatePeerStoresPrivateKeyEncrypted(t *testing.T) {
	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	var stored models.WireGuardPeer
	require.NoError(t, db.First(&stored, "id = ?", peer.ID).Error)

	config, err := service.PeerConfig(peer.ID, "wg-quick")
	require.NoError(t, err)
	require.NotContains(t, config, stored.PrivateKey,
		"the config must carry the decrypted key, proving the column is not plaintext")
}

func TestCreatePeerRejectsOverlappingSubnet(t *testing.T) {
	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	first := createTestSite(t, db, "Site Bandung")
	second := createTestSite(t, db, "Site Bogor")

	_, err = service.CreatePeer(first.ID, "Site Bandung", []string{"192.168.1.0/24"}, "")
	require.NoError(t, err)

	_, err = service.CreatePeer(second.ID, "Site Bogor", []string{"192.168.1.0/24"}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Site Bandung")
}

func TestCreatePeerRejectsSecondPeerForSameSite(t *testing.T) {
	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	_, err = service.CreatePeer(site.ID, "Site A lagi", []string{"10.20.20.0/24"}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "already has a tunnel")
}

func TestDisabledPeerIsNotAppliedToDevice(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	disabled := false
	_, err = service.UpdatePeer(peer.ID, nil, nil, &disabled)
	require.NoError(t, err)
	require.Empty(t, device.Applied.Peers)
}

func TestDeletePeerRemovesItFromDevice(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)
	require.NoError(t, service.DeletePeer(peer.ID))
	require.Empty(t, device.Applied.Peers)
}

func TestRefreshStatusStoresHandshake(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	seen := time.Now().Add(-30 * time.Second).UTC()
	device.Statuses = []connectivity.TunnelPeerStatus{
		{PublicKey: peer.PublicKey, LastHandshakeAt: &seen, RxBytes: 1024, TxBytes: 2048},
	}
	require.NoError(t, service.RefreshStatus())

	var stored models.WireGuardPeer
	require.NoError(t, db.First(&stored, "id = ?", peer.ID).Error)
	require.NotNil(t, stored.LastHandshakeAt)
	require.Equal(t, int64(1024), stored.RxBytes)
	require.True(t, PeerConnected(stored.LastHandshakeAt, time.Now()))
}

func TestSuggestAllowedIPsForSiteUsesRegisteredOLTs(t *testing.T) {
	service, _, db := newWireGuardService(t)
	site := createTestSite(t, db, "Site A")
	require.NoError(t, db.Create(&models.OLT{
		SiteID: site.ID, Name: "OLT 1", IPAddress: "10.10.10.5",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}).Error)

	suggestion, err := service.SuggestAllowedIPsForSite(site.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"10.10.10.0/24"}, suggestion)
}

func TestReconcileRebuildsDeviceStateFromDatabase(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")
	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	device.Applied = connectivity.TunnelConfig{}
	require.NoError(t, service.Reconcile())
	require.Len(t, device.Applied.Peers, 1, "reconcile must restore the full state, not a delta")
	require.Equal(t, "wg0", device.Applied.InterfaceName)
}

func TestPeerConfigRejectsUnknownFormat(t *testing.T) {
	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	_, err = service.PeerConfig(peer.ID, "openvpn")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported format")
}

func TestPeerConfigContainsSiteSubnetInBothFormats(t *testing.T) {
	service, _, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	for _, format := range []string{"wg-quick", "mikrotik"} {
		config, err := service.PeerConfig(peer.ID, format)
		require.NoError(t, err)
		require.Contains(t, config, "10.10.10.0/24")
		require.Contains(t, config, "vpn.contoh.id")
	}
}

func TestCreatePeerFailsWithoutServer(t *testing.T) {
	service, _, db := newWireGuardService(t)
	site := createTestSite(t, db, "Site A")

	_, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "server is not configured")
}

func TestCreatePeerRollsBackWhenDeviceRejects(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	device.ApplyErr = errTunnelApplyForTest
	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.WireGuardPeer{}).Count(&count).Error)
	require.Zero(t, count, "a peer the kernel refused must not stay in the database")
}

var errTunnelApplyForTest = errTunnelApply{}

type errTunnelApply struct{}

func (errTunnelApply) Error() string { return "device refused" }
```

- [ ] **Step 3: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/services/ -run "TestEnsureServer|TestCreatePeer|TestReconcile|TestRefreshStatus|TestDeletePeer|TestDisabledPeer|TestPeerConfig|TestSuggestAllowedIPsForSite" -v`
Expected: FAIL, `undefined: NewWireGuardService`.

- [ ] **Step 4: Tulis tipe perangkat**

Tipe ini berada di paket `connectivity`, bukan `services`. `services` sudah
mengimpor `connectivity`; menaruh interface di `services` dan implementasinya di
`connectivity` akan membuat import cycle yang ditolak Go.

Buat `backend/internal/connectivity/wireguard_device.go`:

```go
package connectivity

import "time"

// TunnelPeerConfig is one peer as the kernel needs it.
type TunnelPeerConfig struct {
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
	Keepalive    time.Duration
}

// TunnelConfig is the complete desired state of the interface. There is no
// partial form on purpose: the service always applies everything.
type TunnelConfig struct {
	InterfaceName string
	PrivateKey    string
	Address       string
	ListenPort    int
	Peers         []TunnelPeerConfig
}

type TunnelPeerStatus struct {
	PublicKey       string
	LastHandshakeAt *time.Time
	RxBytes         int64
	TxBytes         int64
}

// TunnelDevice is the boundary between decisions and the kernel. Everything
// above it is tested; the implementation below it needs root and a Linux host.
type TunnelDevice interface {
	Apply(cfg TunnelConfig) error
	Status(interfaceName string) ([]TunnelPeerStatus, error)
}
```

Buat `backend/internal/connectivity/wireguard_device_memory.go`:

```go
package connectivity

// MemoryTunnelDevice records what would have been applied. It is the device the
// tests use, so service behaviour can be asserted without a kernel.
type MemoryTunnelDevice struct {
	Applied  TunnelConfig
	Statuses []TunnelPeerStatus
	ApplyErr error
}

func (d *MemoryTunnelDevice) Apply(cfg TunnelConfig) error {
	if d.ApplyErr != nil {
		return d.ApplyErr
	}
	d.Applied = cfg
	return nil
}

func (d *MemoryTunnelDevice) Status(string) ([]TunnelPeerStatus, error) {
	return d.Statuses, nil
}
```

- [ ] **Step 5: Tulis service**

Buat `backend/internal/services/wireguard_service.go`:

```go
package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

const (
	defaultInterfaceName = "wg0"
	defaultListenPort    = 51820
	defaultTunnelSubnet  = "10.88.0.0/24"
	defaultServerAddress = "10.88.0.1"
	defaultKeepalive     = 25

	ConfigFormatWGQuick  = "wg-quick"
	ConfigFormatMikroTik = "mikrotik"
)

// ErrServerNotConfigured is returned before the operator has completed the
// one-time server setup.
var ErrServerNotConfigured = errors.New("wireguard server is not configured")

type WireGuardService struct {
	db            *gorm.DB
	encryptionKey string
	device        connectivity.TunnelDevice
}

func NewWireGuardService(db *gorm.DB, encryptionKey string, device connectivity.TunnelDevice) *WireGuardService {
	return &WireGuardService{db: db, encryptionKey: encryptionKey, device: device}
}

func (s *WireGuardService) GetServer() (*models.WireGuardServer, error) {
	var server models.WireGuardServer
	if err := s.db.First(&server).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServerNotConfigured
		}
		return nil, fmt.Errorf("failed to load wireguard server: %w", err)
	}
	return &server, nil
}

// EnsureServer creates the single server row on first use, generating the
// keypair itself so no private key ever arrives from user input.
func (s *WireGuardService) EnsureServer(endpointHost string) (*models.WireGuardServer, error) {
	server, err := s.GetServer()
	if err == nil {
		return server, nil
	}
	if !errors.Is(err, ErrServerNotConfigured) {
		return nil, err
	}

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}
	encrypted, err := utils.Encrypt(key.String(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt server key: %w", err)
	}

	created := &models.WireGuardServer{
		InterfaceName: defaultInterfaceName,
		ListenPort:    defaultListenPort,
		PrivateKey:    encrypted,
		PublicKey:     key.PublicKey().String(),
		EndpointHost:  endpointHost,
		TunnelSubnet:  defaultTunnelSubnet,
		Address:       defaultServerAddress,
	}
	if err := s.db.Create(created).Error; err != nil {
		return nil, fmt.Errorf("failed to create wireguard server: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *WireGuardService) UpdateServer(endpointHost string, listenPort int) (*models.WireGuardServer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}

	server.EndpointHost = endpointHost
	server.ListenPort = listenPort
	if err := s.db.Save(server).Error; err != nil {
		return nil, fmt.Errorf("failed to update wireguard server: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return server, nil
}

func (s *WireGuardService) ListPeers() ([]models.WireGuardPeer, error) {
	var peers []models.WireGuardPeer
	if err := s.db.Order("name").Find(&peers).Error; err != nil {
		return nil, fmt.Errorf("failed to list wireguard peers: %w", err)
	}
	return peers, nil
}

func (s *WireGuardService) GetPeer(id uuid.UUID) (*models.WireGuardPeer, error) {
	var peer models.WireGuardPeer
	if err := s.db.First(&peer, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("peer not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &peer, nil
}

func (s *WireGuardService) SuggestAllowedIPsForSite(siteID uuid.UUID) ([]string, error) {
	var addresses []string
	if err := s.db.Model(&models.OLT{}).Where("site_id = ?", siteID).Pluck("ip_address", &addresses).Error; err != nil {
		return nil, fmt.Errorf("failed to read OLT addresses: %w", err)
	}
	return SuggestAllowedIPs(addresses), nil
}

func (s *WireGuardService) CreatePeer(siteID uuid.UUID, name string, allowedIPs []string, tunnelAddress string) (*models.WireGuardPeer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}

	peers, err := s.ListPeers()
	if err != nil {
		return nil, err
	}
	for _, existing := range peers {
		if existing.SiteID == siteID {
			return nil, fmt.Errorf("this site already has a tunnel")
		}
	}

	networks, err := s.peerNetworks(peers, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if err := ValidateAllowedIPs(allowedIPs, networks, server.TunnelSubnet, DefaultReservedSubnets); err != nil {
		return nil, err
	}

	if tunnelAddress == "" {
		tunnelAddress, err = AllocateTunnelAddress(server.TunnelSubnet, server.Address, takenAddresses(peers))
		if err != nil {
			return nil, err
		}
	}
	if err := ValidateTunnelAddress(tunnelAddress, server.TunnelSubnet, server.Address, takenAddresses(peers)); err != nil {
		return nil, err
	}

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate peer key: %w", err)
	}
	encrypted, err := utils.Encrypt(key.String(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt peer key: %w", err)
	}

	peer := &models.WireGuardPeer{
		SiteID:              siteID,
		Name:                name,
		PublicKey:           key.PublicKey().String(),
		PrivateKey:          encrypted,
		TunnelAddress:       tunnelAddress,
		PersistentKeepalive: defaultKeepalive,
		Enabled:             true,
	}
	if err := peer.SetAllowedIPs(allowedIPs); err != nil {
		return nil, err
	}
	if err := s.db.Create(peer).Error; err != nil {
		return nil, fmt.Errorf("failed to create wireguard peer: %w", err)
	}

	// A peer the kernel refuses must not linger in the database, or the next
	// reconcile would keep trying to apply it.
	if err := s.Reconcile(); err != nil {
		s.db.Delete(&models.WireGuardPeer{}, "id = ?", peer.ID)
		return nil, err
	}
	return peer, nil
}

func (s *WireGuardService) UpdatePeer(id uuid.UUID, name *string, allowedIPs []string, enabled *bool) (*models.WireGuardPeer, error) {
	server, err := s.GetServer()
	if err != nil {
		return nil, err
	}
	peer, err := s.GetPeer(id)
	if err != nil {
		return nil, err
	}

	if allowedIPs != nil {
		peers, err := s.ListPeers()
		if err != nil {
			return nil, err
		}
		networks, err := s.peerNetworks(peers, id)
		if err != nil {
			return nil, err
		}
		if err := ValidateAllowedIPs(allowedIPs, networks, server.TunnelSubnet, DefaultReservedSubnets); err != nil {
			return nil, err
		}
		if err := peer.SetAllowedIPs(allowedIPs); err != nil {
			return nil, err
		}
	}
	if name != nil {
		peer.Name = *name
	}
	if enabled != nil {
		peer.Enabled = *enabled
	}

	if err := s.db.Save(peer).Error; err != nil {
		return nil, fmt.Errorf("failed to update wireguard peer: %w", err)
	}
	if err := s.Reconcile(); err != nil {
		return nil, err
	}
	return peer, nil
}

func (s *WireGuardService) DeletePeer(id uuid.UUID) error {
	if err := s.db.Delete(&models.WireGuardPeer{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete wireguard peer: %w", err)
	}
	return s.Reconcile()
}

func (s *WireGuardService) PeerConfig(id uuid.UUID, format string) (string, error) {
	server, err := s.GetServer()
	if err != nil {
		return "", err
	}
	peer, err := s.GetPeer(id)
	if err != nil {
		return "", err
	}

	privateKey, err := utils.Decrypt(peer.PrivateKey, s.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt peer key: %w", err)
	}
	allowedIPs, err := peer.AllowedIPsList()
	if err != nil {
		return "", err
	}

	input := PeerConfigInput{
		PeerPrivateKey:  privateKey,
		PeerAddress:     peer.TunnelAddress,
		ServerPublicKey: server.PublicKey,
		EndpointHost:    server.EndpointHost,
		TunnelSubnet:    server.TunnelSubnet,
		ListenPort:      server.ListenPort,
		Keepalive:       peer.PersistentKeepalive,
		AllowedIPs:      allowedIPs,
	}

	switch format {
	case ConfigFormatWGQuick:
		return RenderWGQuickConfig(input), nil
	case ConfigFormatMikroTik:
		return RenderMikroTikConfig(input), nil
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
}

// Reconcile applies the whole database to the interface in one call. There is
// no incremental path, so kernel state cannot drift from what is stored.
func (s *WireGuardService) Reconcile() error {
	server, err := s.GetServer()
	if err != nil {
		if errors.Is(err, ErrServerNotConfigured) {
			return nil
		}
		return err
	}

	privateKey, err := utils.Decrypt(server.PrivateKey, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt server key: %w", err)
	}
	peers, err := s.ListPeers()
	if err != nil {
		return err
	}

	cfg := connectivity.TunnelConfig{
		InterfaceName: server.InterfaceName,
		PrivateKey:    privateKey,
		Address:       addressWithSubnetPrefix(server.Address, server.TunnelSubnet),
		ListenPort:    server.ListenPort,
	}
	for _, peer := range peers {
		if !peer.Enabled {
			continue
		}
		allowedIPs, err := peer.AllowedIPsList()
		if err != nil {
			return err
		}
		cfg.Peers = append(cfg.Peers, connectivity.TunnelPeerConfig{
			PublicKey:  peer.PublicKey,
			AllowedIPs: allowedIPs,
			Keepalive:  time.Duration(peer.PersistentKeepalive) * time.Second,
		})
	}

	if err := s.device.Apply(cfg); err != nil {
		return fmt.Errorf("failed to apply wireguard configuration: %w", err)
	}
	return nil
}

func (s *WireGuardService) RefreshStatus() error {
	server, err := s.GetServer()
	if err != nil {
		if errors.Is(err, ErrServerNotConfigured) {
			return nil
		}
		return err
	}

	statuses, err := s.device.Status(server.InterfaceName)
	if err != nil {
		return fmt.Errorf("failed to read wireguard status: %w", err)
	}

	for _, status := range statuses {
		updates := map[string]interface{}{
			"last_handshake_at": status.LastHandshakeAt,
			"rx_bytes":          status.RxBytes,
			"tx_bytes":          status.TxBytes,
		}
		if err := s.db.Model(&models.WireGuardPeer{}).
			Where("public_key = ?", status.PublicKey).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to store peer status: %w", err)
		}
	}
	return nil
}

func (s *WireGuardService) peerNetworks(peers []models.WireGuardPeer, exclude uuid.UUID) ([]PeerNetwork, error) {
	networks := make([]PeerNetwork, 0, len(peers))
	for _, peer := range peers {
		if peer.ID == exclude {
			continue
		}
		allowedIPs, err := peer.AllowedIPsList()
		if err != nil {
			return nil, err
		}
		networks = append(networks, PeerNetwork{
			PeerID:     peer.ID,
			SiteName:   peer.Name,
			AllowedIPs: allowedIPs,
		})
	}
	return networks, nil
}

func takenAddresses(peers []models.WireGuardPeer) []string {
	addresses := make([]string, 0, len(peers))
	for _, peer := range peers {
		addresses = append(addresses, peer.TunnelAddress)
	}
	return addresses
}
```

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/services/ -run "TestEnsureServer|TestCreatePeer|TestReconcile|TestRefreshStatus|TestDeletePeer|TestDisabledPeer|TestPeerConfig|TestSuggestAllowedIPsForSite" -v`
Expected: PASS, empat belas test.

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/services/ -race
git add backend/internal/connectivity/wireguard_device.go backend/internal/connectivity/wireguard_device_memory.go backend/internal/services/wireguard_service.go backend/internal/services/wireguard_service_test.go backend/go.mod backend/go.sum
git commit -m "feat(vpn): manage peers and reconcile tunnel state from the database"
```

---

### Task 7: Lapisan kernel

**Files:**
- Create: `backend/internal/connectivity/wireguard_device_linux.go`
- Create: `backend/internal/connectivity/wireguard_device_other.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Consumes: `TunnelConfig` dan `TunnelPeerStatus` dari Task 6, yang berada di paket yang sama (`connectivity`).
- Produces: `func NewWireGuardDevice() *WireGuardDevice`, dengan metode `Apply(TunnelConfig) error` dan `Status(string) ([]TunnelPeerStatus, error)`.

Task ini adalah satu-satunya yang menyentuh kernel. Tidak ada unit test: `netlink` menuntut `CAP_NET_ADMIN` dan hanya dibangun di Linux. Verifikasinya adalah kompilasi silang dan pemasangan nyata. Ini pengecualian kode network-bound yang sudah tertulis di CLAUDE.md.

- [ ] **Step 1: Tambahkan dependensi netlink**

```bash
cd backend
go get github.com/vishvananda/netlink@v1.3.0
go mod tidy
```

- [ ] **Step 2: Tulis implementasi Linux**

Buat `backend/internal/connectivity/wireguard_device_linux.go`:

```go
//go:build linux

package connectivity

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardDevice drives the kernel interface. It needs CAP_NET_ADMIN, which the
// api container is granted in docker-compose.yml.
type WireGuardDevice struct{}

func NewWireGuardDevice() *WireGuardDevice {
	return &WireGuardDevice{}
}

func (d *WireGuardDevice) Apply(cfg TunnelConfig) error {
	link, err := ensureLink(cfg.InterfaceName)
	if err != nil {
		return err
	}
	if err := ensureAddress(link, cfg.Address); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring %s up: %w", cfg.InterfaceName, err)
	}
	if err := applyWireGuardConfig(cfg); err != nil {
		return err
	}
	return syncRoutes(link, cfg)
}

func ensureLink(name string) (netlink.Link, error) {
	link, err := netlink.LinkByName(name)
	if err == nil {
		return link, nil
	}

	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	wg := &netlink.Wireguard{LinkAttrs: attrs}
	if err := netlink.LinkAdd(wg); err != nil {
		return nil, fmt.Errorf("create %s: %w", name, err)
	}
	return netlink.LinkByName(name)
}

func ensureAddress(link netlink.Link, address string) error {
	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("parse address %s: %w", address, err)
	}

	existing, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("list addresses: %w", err)
	}
	for _, current := range existing {
		if current.Equal(*addr) {
			return nil
		}
		if err := netlink.AddrDel(link, &current); err != nil {
			return fmt.Errorf("remove stale address: %w", err)
		}
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("add address %s: %w", address, err)
	}
	return nil
}

func applyWireGuardConfig(cfg TunnelConfig) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("open wireguard control: %w", err)
	}
	defer client.Close()

	key, err := wgtypes.ParseKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("parse server key: %w", err)
	}

	peers := make([]wgtypes.PeerConfig, 0, len(cfg.Peers))
	for _, peer := range cfg.Peers {
		peerKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			return fmt.Errorf("parse peer key: %w", err)
		}
		allowed, err := parseAllowedIPs(peer.AllowedIPs)
		if err != nil {
			return err
		}
		keepalive := peer.Keepalive
		peers = append(peers, wgtypes.PeerConfig{
			PublicKey:                   peerKey,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  allowed,
			PersistentKeepaliveInterval: &keepalive,
		})
	}

	port := cfg.ListenPort
	return client.ConfigureDevice(cfg.InterfaceName, wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	})
}

func parseAllowedIPs(entries []string) ([]net.IPNet, error) {
	allowed := make([]net.IPNet, 0, len(entries))
	for _, entry := range entries {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("parse allowed ip %s: %w", entry, err)
		}
		allowed = append(allowed, *network)
	}
	return allowed, nil
}

// syncRoutes makes the routing table match the peers. The kernel's allowed-ips
// do not create routes by themselves, and a route left behind by a deleted peer
// would blackhole that subnet.
func syncRoutes(link netlink.Link, cfg TunnelConfig) error {
	wanted := make(map[string]*net.IPNet)
	for _, peer := range cfg.Peers {
		for _, entry := range peer.AllowedIPs {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				return fmt.Errorf("parse route %s: %w", entry, err)
			}
			wanted[network.String()] = network
		}
	}

	existing, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}
	for i := range existing {
		route := existing[i]
		if route.Dst == nil {
			continue
		}
		if _, keep := wanted[route.Dst.String()]; keep {
			delete(wanted, route.Dst.String())
			continue
		}
		if err := netlink.RouteDel(&route); err != nil {
			return fmt.Errorf("remove stale route %s: %w", route.Dst, err)
		}
	}

	for _, network := range wanted {
		route := &netlink.Route{LinkIndex: link.Attrs().Index, Dst: network}
		if err := netlink.RouteAdd(route); err != nil {
			return fmt.Errorf("add route %s: %w", network, err)
		}
	}
	return nil
}

func (d *WireGuardDevice) Status(interfaceName string) ([]TunnelPeerStatus, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("open wireguard control: %w", err)
	}
	defer client.Close()

	device, err := client.Device(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("read device %s: %w", interfaceName, err)
	}

	statuses := make([]TunnelPeerStatus, 0, len(device.Peers))
	for _, peer := range device.Peers {
		status := TunnelPeerStatus{
			PublicKey: peer.PublicKey.String(),
			RxBytes:   peer.ReceiveBytes,
			TxBytes:   peer.TransmitBytes,
		}
		if !peer.LastHandshakeTime.IsZero() {
			handshake := peer.LastHandshakeTime.UTC()
			status.LastHandshakeAt = &handshake
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}
```

- [ ] **Step 3: Tulis pengganti non-Linux**

Buat `backend/internal/connectivity/wireguard_device_other.go`:

```go
//go:build !linux

package connectivity

import "errors"

// errWireGuardUnsupported keeps development on macOS building. The kernel
// interface only exists on the Linux host the API is deployed to.
var errWireGuardUnsupported = errors.New("wireguard requires linux")

type WireGuardDevice struct{}

func NewWireGuardDevice() *WireGuardDevice {
	return &WireGuardDevice{}
}

func (d *WireGuardDevice) Apply(TunnelConfig) error {
	return errWireGuardUnsupported
}

func (d *WireGuardDevice) Status(string) ([]TunnelPeerStatus, error) {
	return nil, errWireGuardUnsupported
}
```

- [ ] **Step 4: Verifikasi kompilasi untuk kedua platform**

```bash
cd backend
go build ./...
GOOS=linux GOARCH=amd64 go build ./...
go vet ./...
```
Expected: keduanya berhasil tanpa keluaran.

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l . && go test ./... -race
git add backend/internal/connectivity/wireguard_device_linux.go backend/internal/connectivity/wireguard_device_other.go backend/go.mod backend/go.sum
git commit -m "feat(vpn): drive the kernel WireGuard interface on linux"
```

---

### Task 8: DTO dan handler HTTP

**Files:**
- Create: `backend/internal/api/wireguard_dto.go`
- Create: `backend/internal/api/wireguard_handler.go`
- Modify: `backend/internal/api/test_helpers.go`
- Test: `backend/internal/api/wireguard_handler_test.go`

**Interfaces:**
- Consumes: seluruh metode `services.WireGuardService` dari Task 6.
- Produces:
  - `func NewWireGuardHandler(service *services.WireGuardService, auditService *services.AuditService) *WireGuardHandler`
  - Metode: `GetServer`, `SaveServer`, `ListPeers`, `CreatePeer`, `UpdatePeer`, `DeletePeer`, `GetPeerConfig`, `SuggestSubnets`
  - `func SetupWireGuardHandlerTest(t *testing.T) (*WireGuardHandler, *services.WireGuardService, *gorm.DB)`

Catatan: `name` pada peer diisi nama site oleh frontend, dan response tidak membawa nama site tersendiri. Frontend sudah memuat daftar site lewat `useSites`, sehingga join di backend tidak diperlukan.

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/internal/api/wireguard_handler_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func createHandlerTestSite(t *testing.T, db *gorm.DB, name string) models.Site {
	site := models.Site{Name: name}
	require.NoError(t, db.Create(&site).Error)
	return site
}

func TestSaveServerCreatesConfigurationOnFirstCall(t *testing.T) {
	handler, _, _ := SetupWireGuardHandlerTest(t)

	w, c := SetupTestContext(http.MethodPut, "/api/v1/wireguard/server", SaveWireguardServerRequest{
		EndpointHost: "vpn.contoh.id",
		ListenPort:   51820,
	})
	handler.SaveServer(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response WireguardServerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "vpn.contoh.id", response.EndpointHost)
	require.NotEmpty(t, response.PublicKey)
}

func TestGetServerReportsNotConfigured(t *testing.T) {
	handler, _, _ := SetupWireGuardHandlerTest(t)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/server", nil)
	handler.GetServer(c)

	require.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "NOT_CONFIGURED", response.Code)
}

func TestCreatePeerReturnsPeerWithoutPrivateKey(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers", CreateWireguardPeerRequest{
		SiteID:     site.ID.String(),
		Name:       "Site A",
		AllowedIPs: []string{"10.10.10.0/24"},
	})
	handler.CreatePeer(c)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotContains(t, w.Body.String(), "private",
		"a peer response must never expose key material")

	var response WireguardPeerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "10.88.0.2", response.TunnelAddress)
	require.False(t, response.Connected)
}

func TestCreatePeerRejectsOverlapWithReadableMessage(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	first := createHandlerTestSite(t, db, "Site Bandung")
	second := createHandlerTestSite(t, db, "Site Bogor")

	_, err = service.CreatePeer(first.ID, "Site Bandung", []string{"192.168.1.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodPost, "/api/v1/wireguard/peers", CreateWireguardPeerRequest{
		SiteID:     second.ID.String(),
		Name:       "Site Bogor",
		AllowedIPs: []string{"192.168.1.0/24"},
	})
	handler.CreatePeer(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "Site Bandung")
}

func TestListPeersMarksConnectionState(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	_, err = service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers", nil)
	handler.ListPeers(c)

	require.Equal(t, http.StatusOK, w.Code)

	var peers []WireguardPeerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &peers))
	require.Len(t, peers, 1)
	require.False(t, peers[0].Connected, "a peer that never handshook must read as disconnected")
}

func TestGetPeerConfigReturnsRequestedFormat(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers/"+peer.ID.String()+"/config?format=mikrotik", nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.GetPeerConfig(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response WireguardPeerConfigResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, "mikrotik", response.Format)
	require.Contains(t, response.Config, "/interface/wireguard/add")
}

func TestGetPeerConfigDefaultsToWGQuick(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/peers/"+peer.ID.String()+"/config", nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.GetPeerConfig(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "[Interface]")
}

func TestSuggestSubnetsUsesRegisteredOLTs(t *testing.T) {
	handler, _, db := SetupWireGuardHandlerTest(t)
	site := createHandlerTestSite(t, db, "Site A")
	require.NoError(t, db.Create(&models.OLT{
		SiteID: site.ID, Name: "OLT 1", IPAddress: "10.10.10.5",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}).Error)

	w, c := SetupTestContext(http.MethodGet, "/api/v1/wireguard/sites/"+site.ID.String()+"/suggested-subnets", nil)
	c.Params = gin.Params{{Key: "site_id", Value: site.ID.String()}}
	handler.SuggestSubnets(c)

	require.Equal(t, http.StatusOK, w.Code)

	var response SuggestedSubnetsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Equal(t, []string{"10.10.10.0/24"}, response.Subnets)
}

func TestDeletePeerReturnsNoContent(t *testing.T) {
	handler, service, db := SetupWireGuardHandlerTest(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createHandlerTestSite(t, db, "Site A")
	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	w, c := SetupTestContext(http.MethodDelete, "/api/v1/wireguard/peers/"+peer.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: peer.ID.String()}}
	handler.DeletePeer(c)

	require.Equal(t, http.StatusNoContent, w.Code)

	var count int64
	require.NoError(t, db.Model(&models.WireGuardPeer{}).Count(&count).Error)
	require.Zero(t, count)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run "TestSaveServer|TestGetServer|TestCreatePeer|TestListPeers|TestGetPeerConfig|TestSuggestSubnets|TestDeletePeer" -v`
Expected: FAIL, `undefined: SetupWireGuardHandlerTest`.

- [ ] **Step 3: Tulis DTO**

Buat `backend/internal/api/wireguard_dto.go`:

```go
package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

type SaveWireguardServerRequest struct {
	EndpointHost string `json:"endpoint_host" binding:"required,min=1,max=255"`
	ListenPort   int    `json:"listen_port" binding:"required,min=1,max=65535"`
}

type CreateWireguardPeerRequest struct {
	SiteID        string   `json:"site_id" binding:"required,uuid"`
	Name          string   `json:"name" binding:"required,min=2,max=255"`
	AllowedIPs    []string `json:"allowed_ips" binding:"required,min=1"`
	TunnelAddress string   `json:"tunnel_address"`
}

type UpdateWireguardPeerRequest struct {
	Name       *string  `json:"name" binding:"omitempty,min=2,max=255"`
	AllowedIPs []string `json:"allowed_ips"`
	Enabled    *bool    `json:"enabled"`
}

// WireguardServerResponse deliberately omits the private key. The only way out
// for key material is the peer config endpoint.
type WireguardServerResponse struct {
	ID            uuid.UUID `json:"id"`
	InterfaceName string    `json:"interface_name"`
	ListenPort    int       `json:"listen_port"`
	PublicKey     string    `json:"public_key"`
	EndpointHost  string    `json:"endpoint_host"`
	TunnelSubnet  string    `json:"tunnel_subnet"`
	Address       string    `json:"address"`
}

type WireguardPeerResponse struct {
	ID                  uuid.UUID  `json:"id"`
	SiteID              uuid.UUID  `json:"site_id"`
	Name                string     `json:"name"`
	TunnelAddress       string     `json:"tunnel_address"`
	AllowedIPs          []string   `json:"allowed_ips"`
	PersistentKeepalive int        `json:"persistent_keepalive"`
	Enabled             bool       `json:"enabled"`
	Connected           bool       `json:"connected"`
	LastHandshakeAt     *time.Time `json:"last_handshake_at"`
	RxBytes             int64      `json:"rx_bytes"`
	TxBytes             int64      `json:"tx_bytes"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type WireguardPeerConfigResponse struct {
	Format string `json:"format"`
	Config string `json:"config"`
}

type SuggestedSubnetsResponse struct {
	Subnets []string `json:"subnets"`
}

func ToWireguardServerResponse(server *models.WireGuardServer) WireguardServerResponse {
	return WireguardServerResponse{
		ID:            server.ID,
		InterfaceName: server.InterfaceName,
		ListenPort:    server.ListenPort,
		PublicKey:     server.PublicKey,
		EndpointHost:  server.EndpointHost,
		TunnelSubnet:  server.TunnelSubnet,
		Address:       server.Address,
	}
}

func ToWireguardPeerResponse(peer *models.WireGuardPeer, now time.Time) WireguardPeerResponse {
	allowedIPs, err := peer.AllowedIPsList()
	if err != nil {
		allowedIPs = nil
	}
	return WireguardPeerResponse{
		ID:                  peer.ID,
		SiteID:              peer.SiteID,
		Name:                peer.Name,
		TunnelAddress:       peer.TunnelAddress,
		AllowedIPs:          allowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
		Enabled:             peer.Enabled,
		Connected:           peer.Enabled && services.PeerConnected(peer.LastHandshakeAt, now),
		LastHandshakeAt:     peer.LastHandshakeAt,
		RxBytes:             peer.RxBytes,
		TxBytes:             peer.TxBytes,
		CreatedAt:           peer.CreatedAt,
		UpdatedAt:           peer.UpdatedAt,
	}
}
```

- [ ] **Step 4: Tulis handler**

Buat `backend/internal/api/wireguard_handler.go`:

```go
package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/services"
)

const auditResourceWireguardPeer = "wireguard_peer"

type WireGuardHandler struct {
	service      *services.WireGuardService
	auditService *services.AuditService
}

func NewWireGuardHandler(service *services.WireGuardService, auditService *services.AuditService) *WireGuardHandler {
	return &WireGuardHandler{service: service, auditService: auditService}
}

func (h *WireGuardHandler) GetServer(c *gin.Context) {
	server, err := h.service.GetServer()
	if err != nil {
		if errors.Is(err, services.ErrServerNotConfigured) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Error: "WireGuard server is not configured yet",
				Code:  "NOT_CONFIGURED",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to load server", Code: "LOAD_FAILED"})
		return
	}
	c.JSON(http.StatusOK, ToWireguardServerResponse(server))
}

// SaveServer performs the one-time setup and later edits with the same call, so
// the UI needs a single form rather than a create/update distinction.
func (h *WireGuardHandler) SaveServer(c *gin.Context) {
	var req SaveWireguardServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}

	if _, err := h.service.EnsureServer(req.EndpointHost); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to initialise server", Code: "SETUP_FAILED", Details: err.Error()})
		return
	}
	server, err := h.service.UpdateServer(req.EndpointHost, req.ListenPort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to save server", Code: "SAVE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "update", "wireguard_server", server.ID, map[string]interface{}{
		"endpoint_host": server.EndpointHost,
		"listen_port":   server.ListenPort,
	})
	c.JSON(http.StatusOK, ToWireguardServerResponse(server))
}

func (h *WireGuardHandler) ListPeers(c *gin.Context) {
	peers, err := h.service.ListPeers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to list peers", Code: "LIST_FAILED"})
		return
	}

	now := time.Now()
	responses := make([]WireguardPeerResponse, len(peers))
	for i := range peers {
		responses[i] = ToWireguardPeerResponse(&peers[i], now)
	}
	c.JSON(http.StatusOK, responses)
}

func (h *WireGuardHandler) CreatePeer(c *gin.Context) {
	var req CreateWireguardPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}
	siteID, err := uuid.Parse(req.SiteID)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid site ID", Code: "INVALID_ID"})
		return
	}

	peer, err := h.service.CreatePeer(siteID, req.Name, req.AllowedIPs, req.TunnelAddress)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to create peer", Code: "CREATE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "create", auditResourceWireguardPeer, peer.ID, map[string]interface{}{
		"site_id":        peer.SiteID.String(),
		"tunnel_address": peer.TunnelAddress,
	})
	c.JSON(http.StatusCreated, ToWireguardPeerResponse(peer, time.Now()))
}

func (h *WireGuardHandler) UpdatePeer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	var req UpdateWireguardPeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body", Code: "INVALID_REQUEST", Details: err.Error()})
		return
	}

	peer, err := h.service.UpdatePeer(id, req.Name, req.AllowedIPs, req.Enabled)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to update peer", Code: "UPDATE_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "update", auditResourceWireguardPeer, peer.ID, map[string]interface{}{
		"enabled": peer.Enabled,
	})
	c.JSON(http.StatusOK, ToWireguardPeerResponse(peer, time.Now()))
}

func (h *WireGuardHandler) DeletePeer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	if err := h.service.DeletePeer(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to delete peer", Code: "DELETE_FAILED"})
		return
	}

	h.audit(c, "delete", auditResourceWireguardPeer, id, nil)
	c.JSON(http.StatusNoContent, nil)
}

// GetPeerConfig is the only endpoint that returns key material, so every call is
// audited.
func (h *WireGuardHandler) GetPeerConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid peer ID", Code: "INVALID_ID"})
		return
	}

	format := c.DefaultQuery("format", services.ConfigFormatWGQuick)
	config, err := h.service.PeerConfig(id, format)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to render config", Code: "CONFIG_FAILED", Details: err.Error()})
		return
	}

	h.audit(c, "download_config", auditResourceWireguardPeer, id, map[string]interface{}{"format": format})
	c.JSON(http.StatusOK, WireguardPeerConfigResponse{Format: format, Config: config})
}

func (h *WireGuardHandler) SuggestSubnets(c *gin.Context) {
	siteID, err := uuid.Parse(c.Param("site_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid site ID", Code: "INVALID_ID"})
		return
	}

	subnets, err := h.service.SuggestAllowedIPsForSite(siteID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to read OLT addresses", Code: "SUGGEST_FAILED"})
		return
	}
	c.JSON(http.StatusOK, SuggestedSubnetsResponse{Subnets: subnets})
}

func (h *WireGuardHandler) audit(c *gin.Context, action, resource string, id uuid.UUID, newValue map[string]interface{}) {
	if h.auditService == nil {
		return
	}
	actorID, _ := middleware.GetUserID(c)
	_ = h.auditService.Log(actorID, action, resource, id, nil, newValue, c.ClientIP(), c.Request.UserAgent())
}
```

- [ ] **Step 5: Tambahkan helper test**

Tambahkan di akhir `backend/internal/api/test_helpers.go`:

```go
// SetupWireGuardHandlerTest creates the VPN handler with an in-memory tunnel
// device, so peer behaviour can be asserted without a kernel interface.
func SetupWireGuardHandlerTest(t *testing.T) (*WireGuardHandler, *services.WireGuardService, *gorm.DB) {
	db := TestDB(t)
	service := services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{})
	handler := NewWireGuardHandler(service, nil) // nil audit service for tests
	return handler, service, db
}
```

Berkas ini sudah mengimpor `services`, `gorm`, `testing`, dan `utils`; tambahkan impor `github.com/tikman/olt-provisioning/internal/connectivity`.

- [ ] **Step 6: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -run "TestSaveServer|TestGetServer|TestCreatePeer|TestListPeers|TestGetPeerConfig|TestSuggestSubnets|TestDeletePeer" -v`
Expected: PASS, sembilan test.

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./internal/api/ -race
git add backend/internal/api/wireguard_dto.go backend/internal/api/wireguard_handler.go backend/internal/api/wireguard_handler_test.go backend/internal/api/test_helpers.go
git commit -m "feat(vpn): expose WireGuard server and peers over HTTP"
```

---

### Task 9: Route, RBAC, dan wiring startup

**Files:**
- Modify: `backend/internal/api/router.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `backend/internal/api/router_test.go`
- Modify: `backend/internal/api/router_middleware_test.go`
- Create: `backend/internal/services/wireguard_refresher.go`
- Test: `backend/internal/api/wireguard_route_test.go`

**Interfaces:**
- Consumes: `NewWireGuardHandler` dari Task 8, `NewWireGuardService` dari Task 6, `connectivity.NewWireGuardDevice` dari Task 7.
- Produces:
  - `func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger, wgService *services.WireGuardService) *gin.Engine`
  - `func (s *WireGuardService) RunStatusRefresher(ctx context.Context, interval time.Duration, logger *zap.Logger)`

- [ ] **Step 1: Tulis test route yang gagal**

Buat `backend/internal/api/wireguard_route_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWireguardRouter(t *testing.T) (*gin.Engine, *auth.Store) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		LogLevel:       "release",
		EncryptionKey:  testEncryptionKey,
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}
	sessionStore := auth.NewMemoryStore(24 * time.Hour)
	wgService := services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{})

	return Setup(gin.New(), cfg, db, sessionStore, logger, wgService), sessionStore
}

func wireguardSessionCookie(t *testing.T, store *auth.Store, role models.UserRole) *http.Cookie {
	t.Helper()
	token, err := store.Create(uuid.New(), role)
	require.NoError(t, err)
	return &http.Cookie{Name: "session_token", Value: token}
}

// A 404 here would mean the path and its only caller have drifted apart; 401
// proves the route exists and merely wants a session.
func TestWireguardRoutesRequireAuthentication(t *testing.T) {
	router, _ := newWireguardRouter(t)

	for _, path := range []string{"/api/v1/wireguard/server", "/api/v1/wireguard/peers"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code, path)
	}
}

func TestWireguardMutationsRequireAdmin(t *testing.T) {
	router, store := newWireguardRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/wireguard/peers", nil)
	req.AddCookie(wireguardSessionCookie(t, store, models.UserRoleTechnician))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code,
		"a wrong subnet on a peer can break routing for other sites, so technicians may only read")
}

func TestWireguardListIsReadableByTechnician(t *testing.T) {
	router, store := newWireguardRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/wireguard/peers", nil)
	req.AddCookie(wireguardSessionCookie(t, store, models.UserRoleTechnician))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./internal/api/ -run TestWireguardRoutes -v`
Expected: FAIL, 404 karena route belum terdaftar.

- [ ] **Step 3: Tulis status refresher**

Buat `backend/internal/services/wireguard_refresher.go`:

```go
package services

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// RunStatusRefresher keeps the peer status columns current. The worker reads
// those columns instead of the kernel, so only this process needs privileges.
func (s *WireGuardService) RunStatusRefresher(ctx context.Context, interval time.Duration, logger *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RefreshStatus(); err != nil {
				logger.Warn("Failed to refresh WireGuard status", zap.Error(err))
			}
		}
	}
}
```

- [ ] **Step 4: Daftarkan route**

Di `backend/internal/api/router.go`, ubah tanda tangan `Setup` menjadi:

```go
func Setup(ginEngine *gin.Engine, cfg *config.Config, db *gorm.DB, authStore *auth.Store, logger *zap.Logger, wgService *services.WireGuardService) *gin.Engine {
```

Tambahkan handler bersama handler lain:

```go
	wireguardHandler := NewWireGuardHandler(wgService, auditService)
```

Tambahkan grup route setelah grup `configTemplates`:

```go
		wireguard := api.Group("/wireguard")
		wireguard.Use(middleware.AuthMiddleware(authStore, logger))
		{
			wireguard.GET("/server", wireguardHandler.GetServer)
			wireguard.PUT("/server", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.SaveServer)
			wireguard.GET("/peers", wireguardHandler.ListPeers)
			wireguard.POST("/peers", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.CreatePeer)
			wireguard.PUT("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.UpdatePeer)
			wireguard.DELETE("/peers/:id", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.DeletePeer)
			wireguard.GET("/peers/:id/config", middleware.RequireRole(models.UserRoleAdmin), wireguardHandler.GetPeerConfig)
			wireguard.GET("/sites/:site_id/suggested-subnets", wireguardHandler.SuggestSubnets)
		}
```

- [ ] **Step 5: Perbarui pemanggil Setup di test**

Di `internal/api/router_test.go` (baris 38, 75, 114, 138) dan `internal/api/router_middleware_test.go` (baris 36), tambahkan argumen terakhir:

```go
	services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{})
```

Contoh untuk `router_middleware_test.go` (tambahkan impor `services` dan
`connectivity` di berkas itu):

```go
	return Setup(gin.New(), cfg, db, auth.NewMemoryStore(24*time.Hour), logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}))
```

- [ ] **Step 6: Wiring di main**

Di `backend/cmd/api/main.go`, sebelum `api.Setup` dipanggil (baris 93), tambahkan:

```go
	wgService := services.NewWireGuardService(db, cfg.EncryptionKey, connectivity.NewWireGuardDevice())
	// A tunnel that cannot come up must not stop the API: the operator needs the
	// UI precisely to fix it.
	if err := wgService.Reconcile(); err != nil {
		log.Warn("Failed to apply WireGuard configuration at startup", zap.Error(err))
	}
	go wgService.RunStatusRefresher(context.Background(), wireguardStatusInterval, log)
```

Ubah pemanggilan menjadi:

```go
	router := api.Setup(engine, cfg, db, sessionStore, log, wgService)
```

Tambahkan konstanta di dekat konstanta lain di berkas itu:

```go
// wireguardStatusInterval is comfortably below the three-minute liveness grace,
// so a peer that drops is noticed within one cycle.
const wireguardStatusInterval = 30 * time.Second
```

Pastikan impor `context`, `time`, `services`, dan `connectivity` sudah ada; tambahkan yang belum.

- [ ] **Step 7: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./internal/api/ -v && go build ./...`
Expected: PASS, termasuk tiga test route baru.

- [ ] **Step 8: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./... -race
git add backend/internal/api/router.go backend/internal/api/router_test.go backend/internal/api/router_middleware_test.go backend/internal/api/wireguard_route_test.go backend/internal/services/wireguard_refresher.go backend/cmd/api/main.go
git commit -m "feat(vpn): register VPN routes and refresh tunnel status"
```

---

### Task 10: Worker melewati site yang tunnelnya mati

**Files:**
- Create: `backend/cmd/worker/wireguard_gate.go`
- Modify: `backend/cmd/worker/main.go`
- Test: `backend/cmd/worker/wireguard_gate_test.go`

**Interfaces:**
- Consumes: `models.WireGuardPeer`, `services.PeerConnected`.
- Produces: `func oltsBehindDownTunnel(db *gorm.DB, now time.Time, logger *zap.Logger) map[uuid.UUID]bool`

- [ ] **Step 1: Tulis test yang gagal**

Buat `backend/cmd/worker/wireguard_gate_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newGateTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	return db
}

func createGateSiteWithOLT(t *testing.T, db *gorm.DB) (models.Site, models.OLT) {
	site := models.Site{Name: "Site A"}
	require.NoError(t, db.Create(&site).Error)

	olt := models.OLT{
		SiteID: site.ID, Name: "OLT 1", IPAddress: "10.10.10.5",
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}
	require.NoError(t, db.Create(&olt).Error)
	return site, olt
}

func createGatePeer(t *testing.T, db *gorm.DB, siteID uuid.UUID, handshake *time.Time, enabled bool) {
	peer := models.WireGuardPeer{
		SiteID:          siteID,
		Name:            "Site A",
		PublicKey:       "pub-" + siteID.String(),
		PrivateKey:      "enc",
		TunnelAddress:   "10.88.0.2",
		Enabled:         enabled,
		LastHandshakeAt: handshake,
	}
	require.NoError(t, peer.SetAllowedIPs([]string{"10.10.10.0/24"}))
	require.NoError(t, db.Create(&peer).Error)
}

func TestOLTsBehindDownTunnelBlocksStalePeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	stale := now.Add(-30 * time.Minute)
	createGatePeer(t, db, site.ID, &stale, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.True(t, blocked[olt.ID])
}

func TestOLTsBehindDownTunnelAllowsLivePeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	recent := now.Add(-20 * time.Second)
	createGatePeer(t, db, site.ID, &recent, true)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID])
}

func TestOLTsBehindDownTunnelIgnoresSitesWithoutPeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	_, olt := createGateSiteWithOLT(t, db)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID],
		"a site reachable without a tunnel must keep being polled")
}

func TestOLTsBehindDownTunnelIgnoresDisabledPeer(t *testing.T) {
	db := newGateTestDB(t)
	now := time.Now()
	site, olt := createGateSiteWithOLT(t, db)

	stale := now.Add(-30 * time.Minute)
	createGatePeer(t, db, site.ID, &stale, false)

	blocked := oltsBehindDownTunnel(db, now, zap.NewNop())
	require.False(t, blocked[olt.ID],
		"a disabled peer means the operator turned the tunnel off, not that the site is unreachable")
}
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd backend && go test ./cmd/worker/ -run TestOLTsBehindDownTunnel -v`
Expected: FAIL, `undefined: oltsBehindDownTunnel`.

- [ ] **Step 3: Tulis implementasi**

Buat `backend/cmd/worker/wireguard_gate.go`:

```go
package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// oltsBehindDownTunnel names the OLTs that cannot be reached this cycle because
// their site's tunnel is down. Polling them anyway would mark every ONT in the
// site offline at once, which is the false alarm commit 9d8c9ee removed.
func oltsBehindDownTunnel(db *gorm.DB, now time.Time, logger *zap.Logger) map[uuid.UUID]bool {
	blocked := make(map[uuid.UUID]bool)

	var peers []models.WireGuardPeer
	if err := db.Where("enabled = ?", true).Find(&peers).Error; err != nil {
		logger.Error("Failed to read WireGuard peers", zap.Error(err))
		return blocked
	}

	downSites := make([]uuid.UUID, 0, len(peers))
	for _, peer := range peers {
		if !services.PeerConnected(peer.LastHandshakeAt, now) {
			downSites = append(downSites, peer.SiteID)
		}
	}
	if len(downSites) == 0 {
		return blocked
	}

	var olts []models.OLT
	if err := db.Where("site_id IN ?", downSites).Find(&olts).Error; err != nil {
		logger.Error("Failed to read OLTs behind down tunnels", zap.Error(err))
		return blocked
	}
	for _, olt := range olts {
		blocked[olt.ID] = true
		logger.Info("Skipping OLT this cycle: site tunnel is down", zap.String("olt", olt.Name))
	}
	return blocked
}
```

- [ ] **Step 4: Pakai di siklus polling**

Di `backend/cmd/worker/main.go`, di dalam `collectMetrics`, tepat setelah baris log pembuka, tambahkan:

```go
	blockedOLTs := oltsBehindDownTunnel(db, time.Now(), logger)
```

Pada loop auto-discover, ganti isi loop menjadi:

```go
		for i := range olts {
			if blockedOLTs[olts[i].ID] {
				continue
			}
			go oltService.AutoDiscoverONTMetrics(&olts[i])
		}
```

Pada loop ONT, tambahkan sebagai pernyataan pertama di dalam `for _, ont := range onts {`:

```go
		if blockedOLTs[ont.OLTID] {
			continue
		}
```

Pengecekan diletakkan sebelum `getOrInitOLT` karena fungsi itulah yang melakukan walk SNMP; memeriksa sesudahnya berarti tetap menunggu timeout.

- [ ] **Step 5: Jalankan test, pastikan lulus**

Run: `cd backend && go test ./cmd/worker/ -v`
Expected: PASS, empat test baru dan seluruh test worker yang sudah ada.

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l . && go vet ./... && go test ./... -race
git add backend/cmd/worker/wireguard_gate.go backend/cmd/worker/wireguard_gate_test.go backend/cmd/worker/main.go
git commit -m "fix(worker): skip polling sites whose tunnel is down"
```

---

### Task 11: Deployment

**Files:**
- Modify: `docker-compose.yml`
- Modify: `CLAUDE.md`

**Interfaces:**
- Consumes: interface `wg0` yang dibuat oleh Task 7.
- Produces: container `api` dengan NET_ADMIN dan port UDP; `worker` yang berbagi namespace dengannya.

- [ ] **Step 1: Beri privilege dan port pada api**

Di `docker-compose.yml`, pada service `api`, tambahkan setelah blok `ports`:

```yaml
    ports:
      - "8080:8080"
      - "51820:51820/udp"
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
```

- [ ] **Step 2: Gabungkan worker ke namespace api**

Pada service `worker`, hapus blok `networks:` dan tambahkan `network_mode`, lalu jadikan `api` sebagai dependensi:

```yaml
  worker:
    build:
      context: ./backend
      dockerfile: Dockerfile.worker
    container_name: tikman-worker
    network_mode: "service:api"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=tikman
      - DB_PASSWORD=${POSTGRES_PASSWORD}
      - DB_NAME=tikman
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - SESSION_SECRET=${SESSION_SECRET}
      - LOG_LEVEL=info
    depends_on:
      postgres:
        condition: service_healthy
      api:
        condition: service_healthy
    restart: unless-stopped
```

`network_mode: service:api` tidak boleh digabung dengan `networks:`; Compose menolaknya. Worker mewarisi resolusi DNS milik `api`, sehingga `DB_HOST=postgres` tetap bekerja.

- [ ] **Step 3: Verifikasi konfigurasi Compose**

Run: `docker compose config >/dev/null && echo ok`
Expected: `ok`, tanpa peringatan.

- [ ] **Step 4: Catat kebutuhan operasional**

Di `CLAUDE.md`, pada bagian **Infrastructure**, tambahkan setelah blok health check:

```markdown
**VPN WireGuard:** container `api` menjalankan interface `wg0` dan karena itu
punya `NET_ADMIN` serta `/dev/net/tun`; `worker` berbagi network namespace
dengannya lewat `network_mode: service:api`, sehingga restart `api` ikut
me-restart `worker`. Port UDP 51820 harus dibuka di firewall penyedia VPS,
karena site yang menginisiasi koneksi tidak akan bisa handshake tanpa itu.
```

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml CLAUDE.md
git commit -m "chore(vpn): give the api container the tunnel and share it with the worker"
```

---

### Task 12: Entity, repository, dan hook frontend

**Files:**
- Create: `frontend/src/domain/entities/Wireguard.ts`
- Create: `frontend/src/infrastructure/repositories/WireguardRepository.ts`
- Create: `frontend/src/application/hooks/useWireguard.ts`
- Modify: `frontend/src/domain/entities/index.ts`
- Modify: `frontend/src/infrastructure/repositories/index.ts`
- Modify: `frontend/src/infrastructure/http/endpoints.ts`
- Modify: `frontend/src/application/hooks/index.ts`
- Test: `frontend/src/infrastructure/repositories/WireguardRepository.test.ts`

**Interfaces:**
- Consumes: endpoint dari Task 9.
- Produces: `WireguardServer`, `WireguardPeer`, `SaveWireguardServerDto`, `CreateWireguardPeerDto`, `UpdateWireguardPeerDto`, `WireguardRepository`, dan hook `useWireguardServer`, `useSaveWireguardServer`, `useWireguardPeers`, `useCreateWireguardPeer`, `useUpdateWireguardPeer`, `useDeleteWireguardPeer`, `useSuggestedSubnets`, `usePeerConfig`.

`apiClient` sudah mengubah snake_case menjadi camelCase lewat humps, jadi entity ditulis dalam camelCase dan tidak perlu pemetaan manual.

- [ ] **Step 1: Tulis test yang gagal**

Buat `frontend/src/infrastructure/repositories/WireguardRepository.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WireguardRepository } from "./WireguardRepository";

const get = vi.fn();
const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
    post: (...args: unknown[]) => post(...args),
  },
}));

describe("WireguardRepository", () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
  });

  it("requests the peer config in the format the caller asked for", async () => {
    get.mockResolvedValue({ data: { format: "mikrotik", config: "/interface" } });

    const config = await new WireguardRepository().getPeerConfig("peer-1", "mikrotik");

    expect(get).toHaveBeenCalledWith(
      "/api/v1/wireguard/peers/peer-1/config?format=mikrotik",
    );
    expect(config.format).toBe("mikrotik");
  });

  it("reads the suggested subnets for a site", async () => {
    get.mockResolvedValue({ data: { subnets: ["10.10.10.0/24"] } });

    const subnets = await new WireguardRepository().getSuggestedSubnets("site-1");

    expect(get).toHaveBeenCalledWith(
      "/api/v1/wireguard/sites/site-1/suggested-subnets",
    );
    expect(subnets).toEqual(["10.10.10.0/24"]);
  });
});
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd frontend && npm test -- --run src/infrastructure/repositories/WireguardRepository.test.ts`
Expected: FAIL, modul `./WireguardRepository` tidak ditemukan.

- [ ] **Step 3: Tulis entity**

Buat `frontend/src/domain/entities/Wireguard.ts`:

```ts
export interface WireguardServer {
  id: string;
  interfaceName: string;
  listenPort: number;
  publicKey: string;
  endpointHost: string;
  tunnelSubnet: string;
  address: string;
}

export interface WireguardPeer {
  id: string;
  siteId: string;
  name: string;
  tunnelAddress: string;
  allowedIps: string[];
  persistentKeepalive: number;
  enabled: boolean;
  connected: boolean;
  lastHandshakeAt: string | null;
  rxBytes: number;
  txBytes: number;
  createdAt: string;
  updatedAt: string;
}

export interface WireguardPeerConfig {
  format: PeerConfigFormat;
  config: string;
}

export type PeerConfigFormat = "wg-quick" | "mikrotik";

export interface SaveWireguardServerDto {
  endpointHost: string;
  listenPort: number;
}

export interface CreateWireguardPeerDto {
  siteId: string;
  name: string;
  allowedIps: string[];
  tunnelAddress?: string;
}

export interface UpdateWireguardPeerDto {
  name?: string;
  allowedIps?: string[];
  enabled?: boolean;
}
```

Tambahkan di `frontend/src/domain/entities/index.ts`:

```ts
export * from "./Wireguard";
```

- [ ] **Step 4: Tambahkan endpoint**

Di `frontend/src/infrastructure/http/endpoints.ts`, tambahkan sebelum penutup objek:

```ts
  // WireGuard VPN
  WIREGUARD_SERVER: "/api/v1/wireguard/server",
  WIREGUARD_PEERS: "/api/v1/wireguard/peers",
  WIREGUARD_PEER_BY_ID: (id: string) => `/api/v1/wireguard/peers/${id}`,
  WIREGUARD_PEER_CONFIG: (id: string, format: string) =>
    `/api/v1/wireguard/peers/${id}/config?format=${format}`,
  WIREGUARD_SUGGESTED_SUBNETS: (siteId: string) =>
    `/api/v1/wireguard/sites/${siteId}/suggested-subnets`,
```

- [ ] **Step 5: Tulis repository**

Buat `frontend/src/infrastructure/repositories/WireguardRepository.ts`:

```ts
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  CreateWireguardPeerDto,
  PeerConfigFormat,
  SaveWireguardServerDto,
  UpdateWireguardPeerDto,
  WireguardPeer,
  WireguardPeerConfig,
  WireguardServer,
} from "@/domain/entities";

export class WireguardRepository {
  async getServer(): Promise<WireguardServer> {
    const response = await apiClient.get(API_ENDPOINTS.WIREGUARD_SERVER);
    return response.data;
  }

  async saveServer(data: SaveWireguardServerDto): Promise<WireguardServer> {
    const response = await apiClient.put(API_ENDPOINTS.WIREGUARD_SERVER, data);
    return response.data;
  }

  async getPeers(): Promise<WireguardPeer[]> {
    const response = await apiClient.get(API_ENDPOINTS.WIREGUARD_PEERS);
    return response.data;
  }

  async createPeer(data: CreateWireguardPeerDto): Promise<WireguardPeer> {
    const response = await apiClient.post(API_ENDPOINTS.WIREGUARD_PEERS, data);
    return response.data;
  }

  async updatePeer(
    id: string,
    data: UpdateWireguardPeerDto,
  ): Promise<WireguardPeer> {
    const response = await apiClient.put(
      API_ENDPOINTS.WIREGUARD_PEER_BY_ID(id),
      data,
    );
    return response.data;
  }

  async deletePeer(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.WIREGUARD_PEER_BY_ID(id));
  }

  async getPeerConfig(
    id: string,
    format: PeerConfigFormat,
  ): Promise<WireguardPeerConfig> {
    const response = await apiClient.get(
      API_ENDPOINTS.WIREGUARD_PEER_CONFIG(id, format),
    );
    return response.data;
  }

  async getSuggestedSubnets(siteId: string): Promise<string[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.WIREGUARD_SUGGESTED_SUBNETS(siteId),
    );
    return response.data.subnets;
  }
}
```

Tambahkan di `frontend/src/infrastructure/repositories/index.ts`:

```ts
export * from "./WireguardRepository";
```

- [ ] **Step 6: Tulis hook**

Buat `frontend/src/application/hooks/useWireguard.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { WireguardRepository } from "@/infrastructure/repositories";
import type {
  CreateWireguardPeerDto,
  PeerConfigFormat,
  SaveWireguardServerDto,
  UpdateWireguardPeerDto,
} from "@/domain/entities";

const repository = new WireguardRepository();

export function useWireguardServer() {
  return useQuery({
    queryKey: ["wireguard", "server"],
    queryFn: () => repository.getServer(),
    retry: false, // a missing server is the expected first-run state, not a failure
  });
}

export function useSaveWireguardServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SaveWireguardServerDto) => repository.saveServer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "server"] });
    },
  });
}

export function useWireguardPeers() {
  return useQuery({
    queryKey: ["wireguard", "peers"],
    queryFn: () => repository.getPeers(),
    refetchInterval: 30_000, // matches the API's status refresh cycle
  });
}

export function useCreateWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateWireguardPeerDto) => repository.createPeer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useUpdateWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWireguardPeerDto }) =>
      repository.updatePeer(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useDeleteWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => repository.deletePeer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useSuggestedSubnets(siteId: string | undefined) {
  return useQuery({
    queryKey: ["wireguard", "suggested-subnets", siteId],
    queryFn: () => repository.getSuggestedSubnets(siteId as string),
    enabled: !!siteId,
  });
}

export function usePeerConfig() {
  return useMutation({
    mutationFn: ({ id, format }: { id: string; format: PeerConfigFormat }) =>
      repository.getPeerConfig(id, format),
  });
}
```

Tambahkan di `frontend/src/application/hooks/index.ts`:

```ts
export * from "./useWireguard";
```

- [ ] **Step 7: Jalankan test, pastikan lulus**

Run: `cd frontend && npm test -- --run src/infrastructure/repositories/WireguardRepository.test.ts`
Expected: PASS, dua test.

- [ ] **Step 8: Commit**

```bash
cd frontend && npm run lint && npm run format:check && npm run build
git add frontend/src/domain/entities/Wireguard.ts frontend/src/domain/entities/index.ts frontend/src/infrastructure/repositories/WireguardRepository.ts frontend/src/infrastructure/repositories/WireguardRepository.test.ts frontend/src/infrastructure/repositories/index.ts frontend/src/infrastructure/http/endpoints.ts frontend/src/application/hooks/useWireguard.ts frontend/src/application/hooks/index.ts
git commit -m "feat(vpn): add frontend data layer for WireGuard peers"
```

---

### Task 13: Halaman VPN

**Files:**
- Create: `frontend/src/presentation/pages/vpn/vpnStatus.ts`
- Create: `frontend/src/presentation/pages/vpn/VpnServerCard.tsx`
- Create: `frontend/src/presentation/pages/vpn/VpnPeerFormModal.tsx`
- Create: `frontend/src/presentation/pages/vpn/VpnConfigModal.tsx`
- Create: `frontend/src/presentation/pages/VpnPage.tsx`
- Modify: `frontend/src/presentation/routes/index.tsx`
- Modify: `frontend/src/presentation/components/layout/Sidebar.tsx`
- Test: `frontend/src/presentation/pages/vpn/vpnStatus.test.ts`

**Interfaces:**
- Consumes: hook dari Task 12.
- Produces: `describeTunnel(peer, now)`, `defaultEndpointHost()`, komponen `VpnServerCard`, `VpnPeerFormModal`, `VpnConfigModal`, dan halaman `VpnPage`.

- [ ] **Step 1: Tulis test yang gagal**

Buat `frontend/src/presentation/pages/vpn/vpnStatus.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { WireguardPeer } from "@/domain/entities";
import { describeTunnel } from "./vpnStatus";

const peer = (overrides: Partial<WireguardPeer>): WireguardPeer =>
  ({
    connected: false,
    enabled: true,
    lastHandshakeAt: null,
    ...overrides,
  }) as WireguardPeer;

const now = new Date("2026-08-29T10:00:00Z");

describe("describeTunnel", () => {
  it("reports a connected tunnel without asking the reader to interpret a timestamp", () => {
    const described = describeTunnel(
      peer({ connected: true, lastHandshakeAt: "2026-08-29T09:59:30Z" }),
      now,
    );

    expect(described.tone).toBe("success");
    expect(described.label).toBe("Terhubung");
  });

  it("says how long a tunnel has been down", () => {
    const described = describeTunnel(
      peer({ connected: false, lastHandshakeAt: "2026-08-29T09:48:00Z" }),
      now,
    );

    expect(described.tone).toBe("error");
    expect(described.label).toBe("Tidak terhubung sejak 12 menit lalu");
  });

  it("distinguishes a tunnel that has never connected", () => {
    const described = describeTunnel(peer({}), now);

    expect(described.label).toBe("Belum pernah terhubung");
    expect(described.hint).toContain("konfigurasi");
  });

  it("marks a peer the operator switched off rather than calling it broken", () => {
    const described = describeTunnel(peer({ enabled: false }), now);

    expect(described.tone).toBe("default");
    expect(described.label).toBe("Dinonaktifkan");
  });
});
```

- [ ] **Step 2: Jalankan test, pastikan gagal**

Run: `cd frontend && npm test -- --run src/presentation/pages/vpn/vpnStatus.test.ts`
Expected: FAIL, modul `./vpnStatus` tidak ditemukan.

- [ ] **Step 3: Tulis helper status**

Buat `frontend/src/presentation/pages/vpn/vpnStatus.ts`:

```ts
import type { WireguardPeer } from "@/domain/entities";

export type TunnelTone = "success" | "error" | "default";

export interface DescribedTunnel {
  tone: TunnelTone;
  label: string;
  hint: string;
}

const MINUTE_MS = 60_000;

// The operator should not have to read a handshake timestamp to know whether a
// site is up, so every state carries its own next step.
export function describeTunnel(
  peer: WireguardPeer,
  now: Date,
): DescribedTunnel {
  if (!peer.enabled) {
    return {
      tone: "default",
      label: "Dinonaktifkan",
      hint: "Tunnel dimatikan dari halaman ini. Aktifkan kembali untuk memantau site.",
    };
  }

  if (peer.connected) {
    return { tone: "success", label: "Terhubung", hint: "" };
  }

  if (!peer.lastHandshakeAt) {
    return {
      tone: "error",
      label: "Belum pernah terhubung",
      hint: "Salin konfigurasi ke perangkat di site, lalu pastikan port UDP server terbuka.",
    };
  }

  return {
    tone: "error",
    label: `Tidak terhubung sejak ${formatSince(peer.lastHandshakeAt, now)}`,
    hint: "Periksa koneksi internet site dan apakah perangkat di lokasi masih menyala.",
  };
}

function formatSince(timestamp: string, now: Date): string {
  const minutes = Math.floor(
    (now.getTime() - new Date(timestamp).getTime()) / MINUTE_MS,
  );
  if (minutes < 60) {
    return `${minutes} menit lalu`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours} jam lalu`;
  }
  return `${Math.floor(hours / 24)} hari lalu`;
}
```

- [ ] **Step 4: Jalankan test, pastikan lulus**

Run: `cd frontend && npm test -- --run src/presentation/pages/vpn/vpnStatus.test.ts`
Expected: PASS, empat test.

- [ ] **Step 5: Tulis kartu server**

Buat `frontend/src/presentation/pages/vpn/VpnServerCard.tsx`:

```tsx
import { Alert, Button, Card, Descriptions, Form, Input, InputNumber } from "antd";
import { useSaveWireguardServer, useWireguardServer } from "@/application/hooks";
import type { SaveWireguardServerDto } from "@/domain/entities";

const DEFAULT_LISTEN_PORT = 51820;

export function VpnServerCard() {
  const { data: server, isLoading } = useWireguardServer();
  const saveServer = useSaveWireguardServer();
  const [form] = Form.useForm<SaveWireguardServerDto>();

  if (isLoading) {
    return <Card loading title="Server VPN" />;
  }

  if (!server) {
    return (
      <Card title="Aktifkan VPN">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="Isi sekali saja"
          description="Kunci server dibuat otomatis. Pastikan port UDP di bawah terbuka di firewall VPS."
        />
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            endpointHost: window.location.hostname,
            listenPort: DEFAULT_LISTEN_PORT,
          }}
          onFinish={(values) => saveServer.mutate(values)}
        >
          <Form.Item
            name="endpointHost"
            label="Alamat publik VPS"
            rules={[{ required: true, message: "Alamat publik wajib diisi" }]}
          >
            <Input placeholder="vpn.contoh.id" />
          </Form.Item>
          <Form.Item
            name="listenPort"
            label="Port UDP"
            rules={[{ required: true, message: "Port wajib diisi" }]}
          >
            <InputNumber min={1} max={65535} style={{ width: "100%" }} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={saveServer.isPending}>
            Aktifkan
          </Button>
        </Form>
      </Card>
    );
  }

  return (
    <Card title="Server VPN">
      <Descriptions column={2} size="small">
        <Descriptions.Item label="Alamat publik">
          {server.endpointHost}:{server.listenPort}
        </Descriptions.Item>
        <Descriptions.Item label="Subnet tunnel">
          {server.tunnelSubnet}
        </Descriptions.Item>
        <Descriptions.Item label="Public key" span={2}>
          <code>{server.publicKey}</code>
        </Descriptions.Item>
      </Descriptions>
    </Card>
  );
}
```

- [ ] **Step 6: Tulis modal peer**

Buat `frontend/src/presentation/pages/vpn/VpnPeerFormModal.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Alert, Collapse, Form, Input, Modal, Select } from "antd";
import {
  useCreateWireguardPeer,
  useSites,
  useSuggestedSubnets,
} from "@/application/hooks";

interface Props {
  open: boolean;
  onClose: () => void;
}

interface FormValues {
  siteId: string;
  allowedIps: string;
  tunnelAddress?: string;
}

export function VpnPeerFormModal({ open, onClose }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [siteId, setSiteId] = useState<string | undefined>();
  const { data: sites } = useSites();
  const { data: suggested } = useSuggestedSubnets(siteId);
  const createPeer = useCreateWireguardPeer();

  // The suggestion comes from the OLT addresses already registered for the site,
  // so the operator confirms a value instead of inventing one.
  useEffect(() => {
    if (suggested?.length) {
      form.setFieldValue("allowedIps", suggested.join(", "));
    }
  }, [suggested, form]);

  const submit = async () => {
    const values = await form.validateFields();
    const site = sites?.find((candidate) => candidate.id === values.siteId);
    await createPeer.mutateAsync({
      siteId: values.siteId,
      name: site?.name ?? "Site",
      allowedIps: values.allowedIps.split(",").map((entry) => entry.trim()),
      tunnelAddress: values.tunnelAddress || undefined,
    });
    form.resetFields();
    setSiteId(undefined);
    onClose();
  };

  return (
    <Modal
      open={open}
      title="Tambah site ke VPN"
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={createPeer.isPending}
      onOk={submit}
      onCancel={onClose}
    >
      {createPeer.isError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="Gagal menyimpan"
          description={(createPeer.error as Error).message}
        />
      )}
      <Form form={form} layout="vertical">
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: "Pilih site" }]}
        >
          <Select
            placeholder="Pilih site"
            onChange={setSiteId}
            options={sites?.map((site) => ({ value: site.id, label: site.name }))}
          />
        </Form.Item>
        <Form.Item
          name="allowedIps"
          label="Subnet lokal di site"
          extra="Dipisah koma bila lebih dari satu. Nilai ini disarankan dari alamat OLT yang sudah terdaftar."
          rules={[{ required: true, message: "Subnet wajib diisi" }]}
        >
          <Input placeholder="10.10.10.0/24" />
        </Form.Item>
        <Collapse
          ghost
          items={[
            {
              key: "advanced",
              label: "Lanjutan",
              children: (
                <Form.Item
                  name="tunnelAddress"
                  label="Alamat tunnel"
                  extra="Kosongkan agar dipilih otomatis."
                >
                  <Input placeholder="10.88.0.2" />
                </Form.Item>
              ),
            },
          ]}
        />
      </Form>
    </Modal>
  );
}
```

- [ ] **Step 7: Tulis modal konfigurasi**

Buat `frontend/src/presentation/pages/vpn/VpnConfigModal.tsx`:

```tsx
import { useEffect, useState } from "react";
import { Alert, Modal, Tabs, Typography } from "antd";
import { usePeerConfig } from "@/application/hooks";
import type { PeerConfigFormat } from "@/domain/entities";

interface Props {
  peerId: string | null;
  onClose: () => void;
}

export function VpnConfigModal({ peerId, onClose }: Props) {
  const [format, setFormat] = useState<PeerConfigFormat>("mikrotik");
  const peerConfig = usePeerConfig();
  const { mutate } = peerConfig;

  useEffect(() => {
    if (peerId) {
      mutate({ id: peerId, format });
    }
  }, [peerId, format, mutate]);

  return (
    <Modal
      open={!!peerId}
      title="Konfigurasi untuk perangkat di site"
      footer={null}
      width={720}
      onCancel={onClose}
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Berisi kunci privat"
        description="Tempel hanya ke perangkat di site tersebut, jangan dibagikan lewat kanal terbuka."
      />
      <Tabs
        activeKey={format}
        onChange={(key) => setFormat(key as PeerConfigFormat)}
        items={[
          { key: "mikrotik", label: "MikroTik" },
          { key: "wg-quick", label: "Linux (wg-quick)" },
        ]}
      />
      <Typography.Paragraph copyable={{ text: peerConfig.data?.config ?? "" }}>
        <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>
          {peerConfig.isPending ? "Menyiapkan..." : peerConfig.data?.config}
        </pre>
      </Typography.Paragraph>
    </Modal>
  );
}
```

- [ ] **Step 8: Tulis halaman**

Buat `frontend/src/presentation/pages/VpnPage.tsx`:

```tsx
import { useState } from "react";
import { Badge, Button, Card, Popconfirm, Space, Table, Tooltip } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  useDeleteWireguardPeer,
  useUpdateWireguardPeer,
  useWireguardPeers,
} from "@/application/hooks";
import type { WireguardPeer } from "@/domain/entities";
import { PageHeader } from "../components/common/PageHeader";
import { VpnConfigModal } from "./vpn/VpnConfigModal";
import { VpnPeerFormModal } from "./vpn/VpnPeerFormModal";
import { VpnServerCard } from "./vpn/VpnServerCard";
import { describeTunnel } from "./vpn/vpnStatus";

export default function VpnPage() {
  const [formOpen, setFormOpen] = useState(false);
  const [configPeerId, setConfigPeerId] = useState<string | null>(null);
  const { data: peers, isLoading } = useWireguardPeers();
  const updatePeer = useUpdateWireguardPeer();
  const deletePeer = useDeleteWireguardPeer();

  const columns = [
    { title: "Site", dataIndex: "name", key: "name" },
    { title: "Alamat tunnel", dataIndex: "tunnelAddress", key: "tunnelAddress" },
    {
      title: "Subnet site",
      dataIndex: "allowedIps",
      key: "allowedIps",
      render: (allowedIps: string[]) => allowedIps.join(", "),
    },
    {
      title: "Status",
      key: "status",
      render: (_: unknown, peer: WireguardPeer) => {
        const described = describeTunnel(peer, new Date());
        return (
          <Tooltip title={described.hint}>
            <Badge status={described.tone} text={described.label} />
          </Tooltip>
        );
      },
    },
    {
      title: "Aksi",
      key: "actions",
      render: (_: unknown, peer: WireguardPeer) => (
        <Space>
          <Button size="small" onClick={() => setConfigPeerId(peer.id)}>
            Konfigurasi
          </Button>
          <Button
            size="small"
            onClick={() =>
              updatePeer.mutate({ id: peer.id, data: { enabled: !peer.enabled } })
            }
          >
            {peer.enabled ? "Nonaktifkan" : "Aktifkan"}
          </Button>
          <Popconfirm
            title="Hapus tunnel site ini?"
            okText="Hapus"
            cancelText="Batal"
            onConfirm={() => deletePeer.mutate(peer.id)}
          >
            <Button size="small" danger>
              Hapus
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <PageHeader
        title="VPN"
        description="Akses site yang tidak punya IP publik"
      />
      <VpnServerCard />
      <Card
        title="Site terhubung"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setFormOpen(true)}
          >
            Tambah site
          </Button>
        }
      >
        <Table
          rowKey="id"
          loading={isLoading}
          dataSource={peers}
          columns={columns}
          pagination={false}
        />
      </Card>
      <VpnPeerFormModal open={formOpen} onClose={() => setFormOpen(false)} />
      <VpnConfigModal
        peerId={configPeerId}
        onClose={() => setConfigPeerId(null)}
      />
    </Space>
  );
}
```

- [ ] **Step 9: Daftarkan route dan menu**

Di `frontend/src/presentation/routes/index.tsx`, tambahkan impor bersama impor halaman lain:

```tsx
import VpnPage from "../pages/VpnPage";
```

dan tambahkan child route setelah `config-templates`:

```tsx
          {
            path: "vpn",
            element: <VpnPage />,
          },
```

Di `frontend/src/presentation/components/layout/Sidebar.tsx`, tambahkan `CloudServerOutlined` pada impor ikon, lalu tambahkan butir menu setelah OLTs:

```tsx
    {
      key: "/vpn",
      icon: <CloudServerOutlined />,
      label: "VPN",
      onClick: () => navigate("/vpn"),
    },
```

- [ ] **Step 10: Verifikasi seluruh gerbang frontend**

```bash
cd frontend
npm test -- --run
npm run lint
npm run format:check
npm run build
```
Expected: semuanya lulus.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/presentation/pages/VpnPage.tsx frontend/src/presentation/pages/vpn frontend/src/presentation/routes/index.tsx frontend/src/presentation/components/layout/Sidebar.tsx
git commit -m "feat(vpn): add the VPN page for managing site tunnels"
```

---

## Verifikasi akhir

Setelah seluruh task selesai, jalankan gerbang yang sama dengan CI:

```bash
cd backend
gofmt -s -l .
go vet ./...
go mod verify
go test ./... -race
go test ./... -coverprofile=coverage.out
go build ./... && GOOS=linux GOARCH=amd64 go build ./...

cd ../frontend
npm test -- --run
npm run lint
npm run format:check
npm run build

cd ..
docker compose config >/dev/null
```

Yang tidak terverifikasi otomatis dan harus dicoba pada satu site nyata: pembuatan interface, pemasangan rute, dan handshake sesungguhnya. Urutan pemeriksaannya: `docker compose exec api wg show` menampilkan peer, `docker compose exec api ip route` memuat subnet site, lalu `docker compose exec worker ping <ip OLT>` berhasil.
