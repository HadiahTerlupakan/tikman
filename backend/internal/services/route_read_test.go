package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

var routeReadNow = time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)

func (f *pollFixture) seedTopIP(t *testing.T, ip string, bytes int64) {
	t.Helper()
	require.NoError(t, f.db.Create(&models.RouterFlowTopIP{Time: routeReadNow.Add(-10 * time.Minute),
		RouterID: f.router.ID, Direction: "download", IP: ip, ASN: 64496, Bytes: bytes}).Error)
}

func storedRoutes(t *testing.T, f *pollFixture) map[string][]StoredRoute {
	t.Helper()
	var rows []models.RouterPrefixRoute
	require.NoError(t, f.db.Where("router_id = ?", f.router.ID).Find(&rows).Error)
	out := map[string][]StoredRoute{}
	for _, r := range rows {
		var routes []StoredRoute
		require.NoError(t, json.Unmarshal([]byte(r.Routes), &routes))
		out[r.Prefix] = routes
	}
	return out
}

func TestReadRoutesFindsTheMostSpecificPrefixHoldingAnAddress(t *testing.T) {
	f := newPollFixture(t)
	f.seedTopIP(t, "198.51.100.7", 900)
	f.seedTopIP(t, "198.51.101.9", 800)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/23": {
		{Session: "peer-ix-1", ASPath: "64500,64496", LocalPref: "700", Active: "true", Gateway: "192.0.2.1"},
		{Session: "peer-transit-1", ASPath: "64501,64502,64496", LocalPref: "100", Gateway: "192.0.2.9"},
	}}

	require.NoError(t, f.service.ReadRoutes(f.router, routeReadNow))

	assert.Equal(t, []string{"198.51.100.0/24", "198.51.100.0/23"}, f.api.asked,
		"the second address is inside the /23 found for the first")
	assert.Equal(t, map[string][]StoredRoute{"198.51.100.0/23": {
		{Session: "peer-ix-1", ASPath: "64500,64496", LocalPref: 700, Active: true, Gateway: "192.0.2.1"},
		{Session: "peer-transit-1", ASPath: "64501,64502,64496", LocalPref: 100, Gateway: "192.0.2.9"},
	}}, storedRoutes(t, f))
}

func TestReadRoutesRereadsAKnownPrefixWithoutSearching(t *testing.T) {
	f := newPollFixture(t)
	f.seedTopIP(t, "198.51.100.7", 900)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/23": {{Session: "peer-ix-1", LocalPref: "700"}}}
	require.NoError(t, f.service.ReadRoutes(f.router, routeReadNow))
	f.api.asked = nil
	f.api.bgpRoutes["198.51.100.0/23"] = []mikrotik.BGPRoute{{Session: "peer-transit-1", LocalPref: "100", Active: "true"}}

	require.NoError(t, f.service.ReadRoutes(f.router, routeReadNow.Add(time.Minute)))

	assert.Equal(t, []string{"198.51.100.0/23"}, f.api.asked)
	assert.Equal(t, "peer-transit-1", storedRoutes(t, f)["198.51.100.0/23"][0].Session, "the new answer replaces the old")
}

// Each query scans the router's whole table: addresses no prefix holds must
// not keep it busy past the budget.
func TestReadRoutesStopsAtItsQueryBudget(t *testing.T) {
	f := newPollFixture(t)
	for i := 0; i < 10; i++ {
		f.seedTopIP(t, fmt.Sprintf("203.0.%d.1", i), int64(1000-i))
	}

	require.NoError(t, f.service.ReadRoutes(f.router, routeReadNow))

	assert.Len(t, f.api.asked, routeReadBudget)
	assert.Empty(t, storedRoutes(t, f))
}

func TestReadRoutesFailureLeavesTheAPIStatusToTheBGPPoll(t *testing.T) {
	f := newPollFixture(t)
	f.seedTopIP(t, "198.51.100.7", 900)
	f.api.err = errors.New("read timeout")

	err := f.service.ReadRoutes(f.router, routeReadNow)

	assert.ErrorContains(t, err, "read timeout")
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.NotEqual(t, models.RouterAPIStatusError, router.APIStatus)
}

func TestReadRoutesWithNoTrafficAsksNothing(t *testing.T) {
	f := newPollFixture(t)

	require.NoError(t, f.service.ReadRoutes(f.router, routeReadNow))

	assert.Empty(t, f.api.asked)
	assert.Zero(t, f.api.closed, "no connection is opened")
}
