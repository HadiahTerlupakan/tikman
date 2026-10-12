package services

import (
	"testing"
	"time"

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

func nowForTest() time.Time { return time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC) }
