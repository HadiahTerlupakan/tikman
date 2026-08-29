package services

import (
	"testing"
	"time"

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

func TestUpdatePeerRollsBackWhenDeviceRejects(t *testing.T) {
	service, device, db := newWireGuardService(t)
	_, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)
	site := createTestSite(t, db, "Site A")

	peer, err := service.CreatePeer(site.ID, "Site A", []string{"10.10.10.0/24"}, "")
	require.NoError(t, err)

	device.ApplyErr = errTunnelApplyForTest
	_, err = service.UpdatePeer(peer.ID, nil, []string{"10.20.20.0/24"}, nil)
	require.Error(t, err)

	var stored models.WireGuardPeer
	require.NoError(t, db.First(&stored, "id = ?", peer.ID).Error)
	allowedIPs, err := stored.AllowedIPsList()
	require.NoError(t, err)
	require.Equal(t, []string{"10.10.10.0/24"}, allowedIPs,
		"an edit the device refused must not stay stored, or every later reconcile fails on it")
}

func TestUpdateServerRollsBackWhenDeviceRejects(t *testing.T) {
	service, device, db := newWireGuardService(t)
	server, err := service.EnsureServer("vpn.contoh.id")
	require.NoError(t, err)

	device.ApplyErr = errTunnelApplyForTest
	_, err = service.UpdateServer("vpn.lain.id", 51821)
	require.Error(t, err)

	var stored models.WireGuardServer
	require.NoError(t, db.First(&stored, "id = ?", server.ID).Error)
	require.Equal(t, "vpn.contoh.id", stored.EndpointHost)
	require.Equal(t, 51820, stored.ListenPort)
}

var errTunnelApplyForTest = errTunnelApply{}

type errTunnelApply struct{}

func (errTunnelApply) Error() string { return "device refused" }
