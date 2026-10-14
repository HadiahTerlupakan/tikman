package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

var (
	ixLink      = uuid.MustParse("00000000-0000-0000-0000-0000000000e1")
	cdnLink     = uuid.MustParse("00000000-0000-0000-0000-0000000000e2")
	smallIXLink = uuid.MustParse("00000000-0000-0000-0000-0000000000e3")
	transitLink = uuid.MustParse("00000000-0000-0000-0000-0000000000e4")
)

// The border router's shape: four route-server sessions at one IX, a CDN
// upstream, a smaller IX, and a full-table transit.
func borderPeers() routePeers {
	return newRoutePeers([]models.RouterBGPSession{
		{Name: "peer-ix-rs1", RemoteAS: 64510, PrefixCount: 9000, InputFilter: "IX_IN", InterfaceID: &ixLink},
		{Name: "peer-ix-rs2", RemoteAS: 64510, PrefixCount: 8900, InputFilter: "IX_IN", InterfaceID: &ixLink},
		{Name: "peer-cdn", RemoteAS: 64511, PrefixCount: 205000, InputFilter: "CDN_IN", InterfaceID: &cdnLink},
		{Name: "peer-small-ix", RemoteAS: 64512, PrefixCount: 43000, InputFilter: "SMALLIX_IN", InterfaceID: &smallIXLink},
		{Name: "peer-transit", RemoteAS: 64513, PrefixCount: 1_090_000, InputFilter: "TRANSIT_IN", InterfaceID: &transitLink},
	})
}

func advise(routes []StoredRoute, inbound []FlowUpstreamShare) RouteAdvice {
	advice := RouteAdvice{Prefix: "198.51.100.0/24"}
	adviseRoute(&advice, routes, borderPeers(), inbound)
	return advice
}

func TestRouteOnlyOfferedByTheTransitIsFlagged(t *testing.T) {
	advice := advise([]StoredRoute{{Session: "peer-transit-1", ASPath: "64513,64512,64496", LocalPref: 100, Active: true}}, nil)

	assert.Equal(t, RouteTransitOnly, advice.Status)
	assert.Empty(t, advice.Draft, "no configuration can add a path the router is not offered")
	require.NotNil(t, advice.Active)
	assert.True(t, advice.Active.Transit)
}

// Moving traffic onto the transit is never suggested, however short its path.
func TestAShorterPathOnlyViaTransitIsNotSuggested(t *testing.T) {
	advice := advise([]StoredRoute{
		{Session: "peer-cdn-1", ASPath: "64511,64520,64521,64496", LocalPref: 600, Active: true},
		{Session: "peer-transit-1", ASPath: "64513,64512,64496", LocalPref: 100},
	}, nil)

	assert.Equal(t, RouteOptimal, advice.Status)
	assert.Empty(t, advice.Draft)
}

func TestAShorterPathAtAnIXGetsADraftForThatPeersChain(t *testing.T) {
	advice := advise([]StoredRoute{
		{Session: "peer-cdn-1", ASPath: "64511,64520,64521,64496", LocalPref: 600, Active: true},
		{Session: "peer-small-ix-1", ASPath: "64512,64496", LocalPref: 300},
		{Session: "peer-transit-1", ASPath: "64513,64512,64496", LocalPref: 100},
	}, nil)

	assert.Equal(t, RouteShorterOutbound, advice.Status)
	assert.Equal(t, "peer-cdn", advice.Active.Peer)
	require.Len(t, advice.Alternatives, 2)
	assert.Equal(t, "peer-small-ix", advice.Alternatives[0].Peer, "shortest first")
	assert.Contains(t, advice.Draft, "chain=SMALLIX_IN")
	assert.Contains(t, advice.Draft, `rule="if (dst == 198.51.100.0/24) { set bgp-local-pref 610; accept; }"`)
	assert.Contains(t, advice.Draft, `place-before=[find where chain="SMALLIX_IN" and rule~"set bgp-local-pref"`)
	assert.Contains(t, advice.Draft, `/routing/filter/rule remove [find where comment="tikman: 198.51.100.0/24 via peer-small-ix"]`)
}

// Local-pref only steers what leaves; download is the remote network's choice.
func TestDownloadArrivingViaTransitDespiteAnIXPathIsFlagged(t *testing.T) {
	advice := advise([]StoredRoute{
		{Session: "peer-ix-rs1-1", ASPath: "64510,64496", LocalPref: 700, Active: true},
		{Session: "peer-transit-1", ASPath: "64513,64496", LocalPref: 100},
	}, []FlowUpstreamShare{
		{UpstreamInterfaceID: transitLink, Bytes: 800},
		{UpstreamInterfaceID: ixLink, Bytes: 200},
	})

	assert.Equal(t, RouteInboundViaTransit, advice.Status)
	assert.Equal(t, []RouteInbound{{Peer: "peer-transit", Transit: true, Bytes: 800}, {Peer: "peer-ix-rs1", Bytes: 200}},
		advice.Inbound)
	assert.Empty(t, advice.Draft, "inbound cannot be fixed with our local-pref")
}

func TestAnIXsRouteServersCountAsOnePeer(t *testing.T) {
	advice := advise([]StoredRoute{
		{Session: "peer-ix-rs1-1", ASPath: "64510,64496", LocalPref: 700, Active: true},
		{Session: "peer-ix-rs2-1", ASPath: "64510,64496", LocalPref: 700},
		{Session: "peer-cdn-1", ASPath: "64511,64496", LocalPref: 600},
	}, []FlowUpstreamShare{{UpstreamInterfaceID: ixLink, Bytes: 900}})

	assert.Equal(t, RouteOptimal, advice.Status)
	require.Len(t, advice.Alternatives, 1)
	assert.Equal(t, "peer-cdn", advice.Alternatives[0].Peer)
}

func TestOriginASNIsTheLastHop(t *testing.T) {
	assert.EqualValues(t, 15169, originASN("152032,24198,138089,15169"))
	assert.EqualValues(t, 13335, originASN("13335"))
}
