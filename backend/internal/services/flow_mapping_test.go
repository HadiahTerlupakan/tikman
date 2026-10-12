package services

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

type mappingFixture struct {
	db      *gorm.DB
	routers *RouterService
	router  *models.MikrotikRouter
	transit models.RouterInterface
	ix      models.RouterInterface
	popA    models.RouterInterface
}

// newMappingFixture is a collecting router at 192.0.2.1 with two upstreams
// (each carrying a BGP session) and one POP VLAN with two blocks.
func newMappingFixture(t *testing.T) *mappingFixture {
	t.Helper()
	routers, db := newRouterServiceForTest(t)
	router, err := routers.Create(validRouterInput())
	require.NoError(t, err)
	require.NoError(t, db.Model(router).Updates(map[string]interface{}{"flow_enabled": true, "flow_sampling": 512}).Error)

	f := &mappingFixture{db: db, routers: routers, router: router}
	f.transit = f.iface(t, "vlan-transit", "198.51.100.1/31", 19)
	f.ix = f.iface(t, "vlan-ix", "203.0.113.141/26", 20)
	f.popA = f.iface(t, "vlan-pop-a", "192.0.2.33/29,192.0.2.57/29", 21)
	f.session(t, "peer-transit", f.transit.ID)
	f.session(t, "peer-ix", f.ix.ID)
	return f
}

func (f *mappingFixture) iface(t *testing.T, name, addresses string, ifIndex int) models.RouterInterface {
	t.Helper()
	iface := models.RouterInterface{RouterID: f.router.ID, Name: name, Type: "vlan",
		Addresses: addresses, IfIndex: &ifIndex, OperStatus: "up"}
	require.NoError(t, f.db.Create(&iface).Error)
	return iface
}

func (f *mappingFixture) session(t *testing.T, name string, interfaceID uuid.UUID) {
	t.Helper()
	require.NoError(t, f.db.Create(&models.RouterBGPSession{RouterID: f.router.ID, Name: name,
		RemoteAddress: "198.51.100.0", RemoteAS: 64500, State: models.BGPStateEstablished,
		InterfaceID: &interfaceID}).Error)
}

func (f *mappingFixture) load(t *testing.T) *netflow.RouterMap {
	t.Helper()
	maps, skipped, err := NewFlowMappingService(f.db).Load()
	require.NoError(t, err)
	assert.Empty(t, skipped)
	return maps[netip.MustParseAddr("192.0.2.1")]
}

func TestLoadMapsUpstreamsAndPOPBlocks(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Create(&models.RouterRoutedPrefix{RouterID: f.router.ID,
		Prefix: "203.0.113.64/27", InterfaceID: f.popA.ID}).Error)

	m := f.load(t)

	require.NotNil(t, m)
	assert.Equal(t, f.router.ID, m.RouterID)
	assert.Equal(t, uint64(512), m.Sampling)
	assert.Equal(t, map[uint32]uuid.UUID{19: f.transit.ID, 20: f.ix.ID}, m.Upstreams)
	cases := map[string]uuid.UUID{
		"192.0.2.38":   f.popA.ID, // interface address
		"192.0.2.60":   f.popA.ID, // its second block
		"203.0.113.70": f.popA.ID, // routed to a device behind it
		"198.51.100.0": uuid.Nil,  // an upstream's subnet is no POP
	}
	for ip, want := range cases {
		got, _ := m.POPs.Lookup(netip.MustParseAddr(ip))
		assert.Equal(t, want, got, ip)
	}
}

func TestLoadSkipsRoutersNotCollectingAndNamesHostnames(t *testing.T) {
	f := newMappingFixture(t)
	named := validRouterInput()
	named.Name, named.Host = "border-2", "border-2.example.net"
	byName, err := f.routers.Create(named)
	require.NoError(t, err)
	require.NoError(t, f.db.Model(byName).Update("flow_enabled", true).Error)
	quiet := validRouterInput()
	quiet.Name, quiet.Host = "border-3", "192.0.2.2"
	_, err = f.routers.Create(quiet)
	require.NoError(t, err)

	maps, skipped, err := NewFlowMappingService(f.db).Load()

	require.NoError(t, err)
	assert.Equal(t, []string{"border-2"}, skipped, "IPFIX carries an address, never a name")
	assert.Len(t, maps, 1, "a router without flow collection is not listened to")

	require.NoError(t, f.db.Model(f.router).Update("enabled", false).Error)
	maps, _, err = NewFlowMappingService(f.db).Load()
	require.NoError(t, err)
	assert.Empty(t, maps, "a disabled router is not listened to")
}

// Review Focus 2: RouterOS renumbers ifIndex after a reboot; the interfaces
// poll stores the new number and the next load follows it.
func TestLoadFollowsARenumberedIfIndex(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(&f.transit).Update("if_index", 42).Error)

	m := f.load(t)

	assert.Equal(t, f.transit.ID, m.Upstreams[42])
	assert.NotContains(t, m.Upstreams, uint32(19))
}

// Review Focus 2: a disabled interface is not polled, so its stored ifIndex
// may since have been given to another; and an index two upstreams both
// claim is dropped rather than guessed.
func TestLoadNeverGuessesAnUpstreamFromAStaleIfIndex(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(&f.ix).Update("if_index", 19).Error)
	assert.NotContains(t, f.load(t).Upstreams, uint32(19), "two upstreams claim 19")

	require.NoError(t, f.db.Model(&f.ix).Updates(map[string]interface{}{"if_index": 20, "disabled": true}).Error)
	m := f.load(t)
	assert.NotContains(t, m.Upstreams, uint32(20))
	assert.Equal(t, f.transit.ID, m.Upstreams[19])
}

// An Admin changing the ratio while flowd runs: the next load carries it.
func TestLoadReflectsTheCurrentSamplingRatio(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(f.router).Update("flow_sampling", 1000).Error)

	assert.Equal(t, uint64(1000), f.load(t).Sampling)
}

// A static route toward an IX peer is a prefix reached through an upstream,
// not a POP, and an upstream's own addresses are no POP either.
func TestLoadIgnoresRoutedPrefixesOnAnUpstream(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Create(&models.RouterRoutedPrefix{RouterID: f.router.ID,
		Prefix: "203.0.113.200/32", InterfaceID: f.ix.ID}).Error)

	m := f.load(t)

	got, _ := m.POPs.Lookup(netip.MustParseAddr("203.0.113.200"))
	assert.Equal(t, uuid.Nil, got)
	got, _ = m.POPs.Lookup(netip.MustParseAddr("203.0.113.141"))
	assert.Equal(t, uuid.Nil, got)
}

// Classify multiplies by Sampling; a zero would erase every count.
func TestLoadNeverReturnsAZeroSampling(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(f.router).Update("flow_sampling", 0).Error)

	assert.Equal(t, uint64(1), f.load(t).Sampling)
}

// A stale upstream keeps its old ifIndex; when a reboot gives that number to
// a POP interface, the flows are the POP's, not the upstream's.
func TestLoadDropsAnIndexAPOPInterfaceAlsoClaims(t *testing.T) {
	f := newMappingFixture(t)
	require.NoError(t, f.db.Model(&f.popA).Update("if_index", 19).Error)

	m := f.load(t)

	assert.NotContains(t, m.Upstreams, uint32(19))
	assert.Equal(t, f.ix.ID, m.Upstreams[20])
}

// Two routers claiming one source address cannot be told apart.
func TestLoadExcludesAnAddressTwoRoutersShare(t *testing.T) {
	f := newMappingFixture(t)
	twin := validRouterInput()
	twin.Name = "border-2"
	other, err := f.routers.Create(twin)
	require.NoError(t, err)
	require.NoError(t, f.db.Model(other).Update("flow_enabled", true).Error)

	maps, skipped, err := NewFlowMappingService(f.db).Load()

	require.NoError(t, err)
	assert.Empty(t, maps)
	assert.ElementsMatch(t, []string{f.router.Name, "border-2"}, skipped)
}
