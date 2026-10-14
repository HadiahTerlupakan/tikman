package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedPrefixRoutes(t *testing.T, db *gorm.DB, routerID uuid.UUID, prefix string, routes []StoredRoute, at time.Time) {
	t.Helper()
	encoded, err := json.Marshal(routes)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.RouterPrefixRoute{RouterID: routerID, Prefix: prefix, Routes: string(encoded), CheckedAt: at}).Error)
}

func TestRouteAdviceSumsAddressesIntoTheirPrefixesAndClassifies(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID := uuid.New()
	require.NoError(t, db.Create(&models.MikrotikRouter{ID: routerID, Name: "border", Host: "192.0.2.1",
		SNMPCommunity: "x", SNMPPort: 161, APIUsername: "tikman", APIPassword: "x", APIPort: 8728,
		Status: models.RouterStatusUp, APIStatus: models.RouterAPIStatusOK}).Error)
	w0 := time.Now().UTC().Truncate(5 * time.Minute).Add(-10 * time.Minute)
	for _, r := range []models.RouterFlowTopIP{
		{Time: w0, IP: "198.51.100.7", Direction: "download", ASN: 64496, Bytes: 600},
		{Time: w0, IP: "198.51.101.9", Direction: "download", ASN: 64496, Bytes: 300},
		{Time: w0, IP: "198.51.100.7", Direction: "upload", ASN: 64496, Bytes: 40},
		{Time: w0, IP: "203.0.113.5", Direction: "download", ASN: 64497, Bytes: 2000},
		{Time: w0, IP: "192.0.2.77", Direction: "download", ASN: 64498, Bytes: 5000}, // no route read yet
	} {
		r.RouterID = routerID
		require.NoError(t, db.Create(&r).Error)
	}
	seedPrefixRoutes(t, db, routerID, "198.51.100.0/23", []StoredRoute{
		{Session: "peer-ix-rs1-1", ASPath: "64510,64496", LocalPref: 700, Active: true},
		{Session: "peer-transit-1", ASPath: "64513,64496", LocalPref: 100},
	}, w0)
	seedPrefixRoutes(t, db, routerID, "203.0.113.0/24", []StoredRoute{
		{Session: "peer-transit-1", ASPath: "64513,64512,64497", LocalPref: 100, Active: true},
	}, w0)
	for id, name := range map[uuid.UUID]string{ixLink: "vlan-ix", transitLink: "vlan-transit"} {
		require.NoError(t, db.Create(&models.RouterInterface{ID: id, RouterID: routerID, Name: name, Type: "vlan",
			OperStatus: "up"}).Error)
	}
	for _, s := range []models.RouterBGPSession{
		{RouterID: routerID, Name: "peer-ix-rs1", RemoteAS: 64510, PrefixCount: 9000, InputFilter: "IX_IN", InterfaceID: &ixLink},
		{RouterID: routerID, Name: "peer-transit", RemoteAS: 64513, PrefixCount: 1_090_000, InputFilter: "TRANSIT_IN", InterfaceID: &transitLink},
	} {
		require.NoError(t, db.Create(&s).Error)
	}
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: w0, RouterID: routerID, UpstreamInterfaceID: transitLink,
		Direction: "download", ASN: 64496, Bytes: 700})
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: w0, RouterID: routerID, UpstreamInterfaceID: ixLink,
		Direction: "download", ASN: 64496, Bytes: 100})
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: w0}).Error)

	advice, err := flows.RouteAdvice(routerID, FlowFilter{From: w0, To: w0.Add(15 * time.Minute)}, time.Now())

	require.NoError(t, err)
	require.Len(t, advice, 2, "the address with no route read yet is left out")
	assert.Equal(t, "203.0.113.0/24", advice[0].Prefix, "busiest first")
	assert.Equal(t, RouteTransitOnly, advice[0].Status)
	assert.EqualValues(t, 64497, advice[0].ASN)

	assert.Equal(t, "198.51.100.0/23", advice[1].Prefix)
	assert.EqualValues(t, 900, advice[1].DownloadBytes, "both addresses in the /23")
	assert.EqualValues(t, 40, advice[1].UploadBytes)
	assert.Equal(t, "EXAMPLE-NET-A", advice[1].Name)
	assert.Equal(t, RouteInboundViaTransit, advice[1].Status, "download comes via the transit though the IX offers the prefix")
	assert.Equal(t, "peer-ix-rs1", advice[1].Active.Peer)
}

func TestRouteAdviceWithNoRoutesReadIsEmpty(t *testing.T) {
	db := setupPostgresTestDB(t)
	w0 := time.Now().UTC().Add(-10 * time.Minute)

	advice, err := NewFlowQueryService(db).RouteAdvice(uuid.New(), FlowFilter{From: w0, To: time.Now()}, time.Now())

	require.NoError(t, err)
	assert.Empty(t, advice)
}
