package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestGuessRole(t *testing.T) {
	pppoe := []mikrotik.PPPoEServer{{ServiceName: "pel", Interface: "vlan-10"}}
	bgp := []mikrotik.BGPConnection{{Name: "transit", RemoteAS: "64500", Disabled: "false"}}
	disabled := []mikrotik.BGPConnection{{Name: "old", RemoteAS: "64500", Disabled: "true"}}
	offPPPoE := []mikrotik.PPPoEServer{{ServiceName: "lab", Interface: "vlan-99", Disabled: "true"}}
	cases := []struct {
		name  string
		pppoe []mikrotik.PPPoEServer
		bgp   []mikrotik.BGPConnection
		want  models.RouterRole
	}{
		{"pppoe and bgp", pppoe, bgp, models.RouterRoleBRAS},
		{"bgp only", nil, bgp, models.RouterRoleBorder},
		{"disabled pppoe and bgp", offPPPoE, bgp, models.RouterRoleBorder},
		{"disabled pppoe only", offPPPoE, nil, models.RouterRoleOther},
		{"disabled bgp only", nil, disabled, models.RouterRoleOther},
		{"nothing", nil, nil, models.RouterRoleOther},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, guessRole(c.pppoe, c.bgp))
		})
	}
}

func TestUplinkFor(t *testing.T) {
	ifaces := []models.RouterInterface{
		{ID: uuid.New(), Name: "sfp-sfpplus1", Addresses: "192.0.2.2/30"},
		{ID: uuid.New(), Name: "sfp-sfpplus2"},
		{ID: uuid.New(), Name: "vlan-10", Addresses: "198.51.100.1/24"},
	}
	cases := []struct {
		name    string
		gateway string
		want    *uuid.UUID
	}{
		{"address in a subnet", "192.0.2.1", &ifaces[0].ID},
		{"interface name", "sfp-sfpplus2", &ifaces[1].ID},
		{"address scoped to an interface", "192.0.2.1%vlan-10", &ifaces[2].ID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, uplinkFor([]mikrotik.Route{{DstAddress: "0.0.0.0/0", Gateway: c.gateway}}, ifaces))
		})
	}
	assert.Nil(t, uplinkFor(nil, ifaces), "no default route, no uplink")
}

func brasInventory(api *fakeRouterAPI) {
	api.interfaces = []mikrotik.Interface{{Name: "sfp-sfpplus1", Type: "ether"}, {Name: "vlan-10", Type: "vlan"}}
	api.addresses = []mikrotik.Address{{Address: "192.0.2.2/30", Interface: "sfp-sfpplus1"}}
	api.pppoe = []mikrotik.PPPoEServer{{ServiceName: "pel", Interface: "vlan-10"}}
	api.defaults = []mikrotik.Route{{DstAddress: "0.0.0.0/0", Gateway: "192.0.2.1"}}
}

func (f *pollFixture) reloadRouter(t *testing.T) *models.MikrotikRouter {
	t.Helper()
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	return &router
}

func (f *pollFixture) jobKinds(t *testing.T) []models.RouterPollKind {
	t.Helper()
	var kinds []models.RouterPollKind
	require.NoError(t, f.db.Model(&models.RouterPollJob{}).Where("router_id = ?", f.router.ID).Pluck("kind", &kinds).Error)
	return kinds
}

func (f *pollFixture) storedInterface(t *testing.T, name string) models.RouterInterface {
	t.Helper()
	var iface models.RouterInterface
	require.NoError(t, f.db.First(&iface, "router_id = ? AND name = ?", f.router.ID, name).Error)
	return iface
}

func TestDiscoverWritesGuessUnlessAdminSet(t *testing.T) {
	f := newPollFixture(t)
	brasInventory(f.api)

	require.NoError(t, f.service.Discover(f.router))

	router := f.reloadRouter(t)
	assert.Equal(t, models.RouterRoleBRAS, router.Role)
	assert.False(t, router.RoleSetByAdmin)
	require.NotNil(t, router.RoleGuess)
	assert.Equal(t, models.RouterRoleBRAS, *router.RoleGuess)
	assert.ElementsMatch(t, RouterPollKindsFor(models.RouterRoleBRAS), f.jobKinds(t))

	admin := newPollFixture(t)
	brasInventory(admin.api)
	require.NoError(t, SetRouterRole(admin.db, admin.router.ID, models.RouterRoleBorder, true))

	require.NoError(t, admin.service.Discover(admin.reloadRouter(t)))

	router = admin.reloadRouter(t)
	assert.Equal(t, models.RouterRoleBorder, router.Role, "the operator's choice stands")
	require.NotNil(t, router.RoleGuess)
	assert.Equal(t, models.RouterRoleBRAS, *router.RoleGuess)
}

// An admin who picks a role while discovery is reading the router must not
// have it overwritten by the guess discovery made from the router it loaded.
func TestDiscoverKeepsRoleAnAdminSetMidRun(t *testing.T) {
	f := newPollFixture(t)
	brasInventory(f.api)
	f.service.readIfNames = func(context.Context, string, int, string) (map[string]int, error) {
		require.NoError(t, SetRouterRole(f.db, f.router.ID, models.RouterRoleOther, true))
		return nil, nil
	}

	require.NoError(t, f.service.Discover(f.router))

	router := f.reloadRouter(t)
	assert.Equal(t, models.RouterRoleOther, router.Role)
	assert.True(t, router.RoleSetByAdmin)
	assert.ElementsMatch(t, RouterPollKindsFor(models.RouterRoleOther), f.jobKinds(t))
}

func TestDiscoverKeepsRoleWhenPPPoEReadFails(t *testing.T) {
	f := newPollFixture(t)
	brasInventory(f.api)
	f.api.pppoeErr = errors.New("no such command")

	require.NoError(t, f.service.Discover(f.router))

	router := f.reloadRouter(t)
	assert.Equal(t, models.RouterRoleBorder, router.Role)
	assert.Nil(t, router.RoleGuess, "a failed read is no evidence for any role")
	assert.ElementsMatch(t, RouterPollKindsFor(models.RouterRoleBorder), f.jobKinds(t))
}

func TestDiscoverStoresBRASUplink(t *testing.T) {
	f := newPollFixture(t)
	brasInventory(f.api)

	require.NoError(t, f.service.Discover(f.router))

	router := f.reloadRouter(t)
	require.NotNil(t, router.UplinkInterfaceID)
	assert.Equal(t, f.storedInterface(t, "sfp-sfpplus1").ID, *router.UplinkInterfaceID)

	f.api.defaults, f.api.defaultsErr = nil, errors.New("timeout")
	require.NoError(t, f.service.Discover(f.reloadRouter(t)))

	router = f.reloadRouter(t)
	require.NotNil(t, router.UplinkInterfaceID, "a failed read keeps the uplink already known")
}

func TestDiscoverMapsBRASIfIndexes(t *testing.T) {
	f := newPollFixture(t)
	brasInventory(f.api)
	f.ifNames = map[string]int{"sfp-sfpplus1": 3}

	require.NoError(t, f.service.Discover(f.router))

	stored := f.storedInterface(t, "sfp-sfpplus1")
	require.NotNil(t, stored.IfIndex)
	assert.Equal(t, 3, *stored.IfIndex)
	assert.Nil(t, f.storedInterface(t, "vlan-10").IfIndex)

	f.ifNames, f.ifNameErr = nil, errors.New("request timeout")
	require.NoError(t, f.service.Discover(f.reloadRouter(t)), "a failed walk does not fail discovery")

	stored = f.storedInterface(t, "sfp-sfpplus1")
	require.NotNil(t, stored.IfIndex)
	assert.Equal(t, 3, *stored.IfIndex)
}

func TestDiscoverBorderSkipsBRASReads(t *testing.T) {
	f := newPollFixture(t)
	borderInventory(f.api)
	f.api.connections = []mikrotik.BGPConnection{{Name: "transit", RemoteAS: "64500", Disabled: "false"}}

	require.NoError(t, f.service.Discover(f.router))

	assert.Empty(t, f.api.calls)
	router := f.reloadRouter(t)
	assert.Equal(t, models.RouterRoleBorder, router.Role)
	assert.Nil(t, router.UplinkInterfaceID)
}
