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

// A last-stopped older than the start already stored belongs to an earlier
// outage; dating this restart's drop by it would place the drop before the
// session's previous start.
func TestDiffBGPIgnoresALastStoppedFromAnEarlierOutage(t *testing.T) {
	stored := []models.RouterBGPSession{storedSession("peer-transit", models.BGPStateEstablished, at("2026-10-10 10:00:00"))}
	current := []BGPPeerState{{
		Name: "peer-transit", State: models.BGPStateEstablished,
		LastStoppedAt: at("2026-10-09 08:00:00"), LastStartedAt: at("2026-10-11 16:06:43"),
	}}

	events := DiffBGP(uuid.New(), stored, current, time.Now())

	require.Len(t, events, 2)
	assert.Equal(t, models.BGPStateDown, events[0].ToState)
	assert.Equal(t, *at("2026-10-11 16:06:43"), events[0].OccurredAt)
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
