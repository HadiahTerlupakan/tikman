package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestOwnPrefixesReadEachPeersOutputRuleAndTraffic(t *testing.T) {
	db := setupPostgresTestDB(t)
	router := &models.MikrotikRouter{ID: uuid.New(), Name: "border", Host: "192.0.2.1", SNMPCommunity: "x",
		SNMPPort: 161, APIUsername: "tikman", APIPassword: "x", APIPort: 8728,
		Status: models.RouterStatusUp, APIStatus: models.RouterAPIStatusOK}
	require.NoError(t, db.Create(router).Error)
	cdn, ix, transit, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for id, name := range map[uuid.UUID]string{cdn: "vlan-cdn", ix: "vlan-ix", transit: "vlan-transit", other: "vlan-other"} {
		require.NoError(t, db.Create(&models.RouterInterface{ID: id, RouterID: router.ID, Name: name, Type: "vlan",
			OperStatus: "up"}).Error)
	}
	for _, s := range []models.RouterBGPSession{
		{Name: "peer-cdn", RemoteAS: 64511, PrefixCount: 200_000, OutputFilter: "CDN_OUT", InterfaceID: &cdn},
		{Name: "peer-ix-rs1", RemoteAS: 64510, PrefixCount: 9000, OutputFilter: "IX_OUT", InterfaceID: &ix},
		{Name: "peer-ix-rs2", RemoteAS: 64510, PrefixCount: 9000, OutputFilter: "IX_OUT", InterfaceID: &ix},
		{Name: "peer-transit", RemoteAS: 64513, PrefixCount: 1_000_000, OutputFilter: "TRANSIT_OUT", InterfaceID: &transit},
		{Name: "peer-other", RemoteAS: 64514, PrefixCount: 100, OutputFilter: "OTHER_OUT", InterfaceID: &other},
	} {
		s.RouterID, s.State = router.ID, models.BGPStateEstablished
		require.NoError(t, db.Create(&s).Error)
	}
	require.NoError(t, db.Create(&models.RouterAnnouncedPrefix{RouterID: router.ID, Prefix: "203.0.113.0/24"}).Error)
	for i, r := range []models.RouterFilterRule{
		{Chain: "CDN_OUT", Rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }"},
		{Chain: "IX_OUT", Rule: "if (dst in 203.0.113.0/24 && dst-len == 24) { accept; }"},
		{Chain: "TRANSIT_OUT", Rule: "if (dst in 203.0.113.0/24) { set bgp-path-prepend 9; accept; }", Disabled: true},
		{Chain: "TRANSIT_OUT", Rule: "if (dst in 203.0.113.0/24) { reject; }"},
		{Chain: "TRANSIT_OUT", Rule: "if (dst in 203.0.113.0/24) { set bgp-path-prepend 6; accept; }"},
	} {
		r.RouterID, r.Position = router.ID, i
		require.NoError(t, db.Create(&r).Error)
	}
	at := time.Now().Add(-time.Hour)
	for _, row := range []models.RouterFlowOwnPrefix{
		{UpstreamInterfaceID: cdn, Direction: "download", Bytes: 700},
		{UpstreamInterfaceID: ix, Direction: "download", Bytes: 300},
		{UpstreamInterfaceID: ix, Direction: "upload", Bytes: 40},
	} {
		row.Time, row.RouterID, row.Prefix = at, router.ID, "203.0.113.0/24"
		require.NoError(t, db.Create(&row).Error)
	}

	out, err := NewFlowQueryService(db).OwnPrefixes(router.ID,
		FlowFilter{From: time.Now().Add(-2 * time.Hour), To: time.Now()}, time.Now())

	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.EqualValues(t, 1000, out[0].DownloadBytes)
	assert.EqualValues(t, 40, out[0].UploadBytes)
	peers := map[string]OwnPrefixPeer{}
	for _, p := range out[0].Peers {
		peers[p.Peer] = p
	}
	require.Len(t, peers, 4, "the IX's two route servers are one peer")
	assert.Equal(t, OwnPrefixPeer{Peer: "peer-cdn", Sessions: []string{"peer-cdn"}, OutputFilter: "CDN_OUT",
		Rule:      "if (dst in 203.0.113.0/24 && dst-len == 24) { set bgp-path-prepend 2; accept; }",
		Announced: true, Prepend: 2, DownloadBytes: 700}, peers["peer-cdn"])
	assert.Equal(t, []string{"peer-ix-rs1", "peer-ix-rs2"}, peers["peer-ix-rs1"].Sessions)
	assert.True(t, peers["peer-ix-rs1"].Announced)
	assert.Zero(t, peers["peer-ix-rs1"].Prepend)
	assert.EqualValues(t, 300, peers["peer-ix-rs1"].DownloadBytes)
	assert.False(t, peers["peer-transit"].Announced, "the first enabled rule naming it rejects it")
	assert.True(t, peers["peer-transit"].Transit)
	assert.Empty(t, peers["peer-other"].Rule, "no rule names the prefix")
}
