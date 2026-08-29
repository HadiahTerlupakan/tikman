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
