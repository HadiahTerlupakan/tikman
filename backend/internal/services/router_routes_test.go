package services

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// popRoutes is what /ip/route returns on borderInventory's router: two POP
// blocks, and every kind of route that is not one.
func popRoutes() []mikrotik.Route {
	return []mikrotik.Route{
		// A POP router's block, its interface named by immediate-gw.
		{DstAddress: "203.0.113.64/27", Gateway: "192.0.2.34", ImmediateGW: "192.0.2.34%vlan-pop-a", Static: "true", Active: "true"},
		// Only a gateway address, inside vlan-pop-a's second subnet; stored masked.
		{DstAddress: "203.0.113.130/26", Gateway: "192.0.2.58", Static: "true", Active: "true"},
		// The default route points at an upstream, not at a POP.
		{DstAddress: "0.0.0.0/0", Gateway: "198.51.100.0", Static: "true", Active: "true"},
		// A blackhole for the aggregate leaves through no interface.
		{DstAddress: "203.0.113.0/24", Static: "true", Active: "true"},
		// Flags the router-side filter should already have removed.
		{DstAddress: "198.51.100.128/25", Gateway: "198.51.100.0", Static: "true", Active: "true", BGP: "true"},
		{DstAddress: "192.0.2.32/29", Gateway: "vlan-pop-a", Static: "true", Active: "true", Connect: "true"},
		{DstAddress: "203.0.113.192/28", Gateway: "192.0.2.34", Static: "true", Active: "false"},
	}
}

// storedPrefixes maps each routed prefix of the fixture's router to the name
// of the interface it was stored under.
func storedPrefixes(t *testing.T, f *pollFixture) map[string]string {
	t.Helper()
	var prefixes []models.RouterRoutedPrefix
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&prefixes).Error)
	var ifaces []models.RouterInterface
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&ifaces).Error)
	names := map[uuid.UUID]string{}
	for _, i := range ifaces {
		names[i.ID] = i.Name
	}
	out := map[string]string{}
	for _, p := range prefixes {
		out[p.Prefix] = names[p.InterfaceID]
	}
	return out
}

func TestDiscoverStoresStaticRoutesUnderTheirGatewayInterface(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.routes = popRoutes()

	require.NoError(t, f.service.Discover(f.router))

	assert.Equal(t, map[string]string{
		"203.0.113.64/27":  "vlan-pop-a",
		"203.0.113.128/26": "vlan-pop-a",
	}, storedPrefixes(t, f))
}

func TestDiscoverDropsRoutesGoneFromTheRouter(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.routes = popRoutes()
	require.NoError(t, f.service.Discover(f.router))

	f.api.routes = popRoutes()[:1]
	require.NoError(t, f.service.Discover(f.router))

	assert.Equal(t, map[string]string{"203.0.113.64/27": "vlan-pop-a"}, storedPrefixes(t, f))
}

func TestDiscoverKeepsPrefixesButRefreshesInterfacesWhenRoutesCannotBeRead(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.routes = popRoutes()
	require.NoError(t, f.service.Discover(f.router))

	f.api.interfaces = append(f.api.interfaces, mikrotik.Interface{Name: "vlan-pop-b", Type: "vlan", Running: "true"})
	f.api.routesErr = errors.New("timeout")
	err := f.service.Discover(f.router)

	require.ErrorIs(t, err, ErrStaticRoutesUnread)
	var added int64
	require.NoError(t, f.db.Model(&models.RouterInterface{}).
		Where("router_id = ? AND name = ?", f.router.ID, "vlan-pop-b").Count(&added).Error)
	assert.EqualValues(t, 1, added)
	assert.Equal(t, map[string]string{
		"203.0.113.64/27":  "vlan-pop-a",
		"203.0.113.128/26": "vlan-pop-a",
	}, storedPrefixes(t, f))
}

func TestRouteGatewayWithAZoneNamesItsInterface(t *testing.T) {
	id := uuid.New()
	rows := buildRoutedPrefixes(uuid.New(), []mikrotik.Route{
		{DstAddress: "203.0.113.64/27", Gateway: "192.0.2.34%vlan-pop-a", Static: "true", Active: "true"},
	}, []models.RouterInterface{{ID: id, Name: "vlan-pop-a"}})

	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].InterfaceID)
}
