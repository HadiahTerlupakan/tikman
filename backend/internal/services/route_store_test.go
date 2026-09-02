package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func routeFixture(t *testing.T) (*DistributionService, *models.ODP) {
	t.Helper()
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, _ := distributionFixture(t, db)
	odc, err := service.CreateODC(ODCInput{SiteID: site.ID, Code: "ODC"})
	require.NoError(t, err)
	odp, err := service.CreateODP(ODPInput{Code: "ODP", PortCount: 8, ODCID: &odc.ID})
	require.NoError(t, err)
	return service, odp
}

func TestSetODPRouteStoresThePathAndItsLength(t *testing.T) {
	service, odp := routeFixture(t)

	require.NoError(t, service.SetODPRoute(odp.ID, []models.RoutePoint{
		{Lat: -6.4000, Lng: 107.0000},
		{Lat: -6.4100, Lng: 107.0000},
	}))

	stored, err := service.ODPByID(odp.ID)
	require.NoError(t, err)
	points, err := stored.RoutePath()
	require.NoError(t, err)
	assert.Len(t, points, 2)
	// Kept beside the path so a list can add lengths without parsing each one.
	assert.InDelta(t, 1112, stored.RouteMeters, 20)
}

func TestSetODPRouteClearingReturnsItToTheStraightLine(t *testing.T) {
	service, odp := routeFixture(t)
	require.NoError(t, service.SetODPRoute(odp.ID, []models.RoutePoint{
		{Lat: -6.40, Lng: 107.00}, {Lat: -6.41, Lng: 107.00},
	}))

	// Choosing "automatic" clears the path rather than storing the two ends, so
	// the line follows the box if it is ever moved instead of freezing where it
	// used to stand.
	require.NoError(t, service.SetODPRoute(odp.ID, nil))

	stored, err := service.ODPByID(odp.ID)
	require.NoError(t, err)
	points, err := stored.RoutePath()
	require.NoError(t, err)
	assert.Empty(t, points)
	assert.Zero(t, stored.RouteMeters)
}

func TestSetODPRouteRefusesAPathThatIsNotAPath(t *testing.T) {
	service, odp := routeFixture(t)

	// One point is a place, not a route. Storing it would report a cable of
	// zero length as though someone had measured it.
	err := service.SetODPRoute(odp.ID, []models.RoutePoint{{Lat: -6.4, Lng: 107}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "two points")
}

func TestSetODCFeedRouteStoresThePathAndItsLength(t *testing.T) {
	db := setupTestDB(t)
	service := NewDistributionService(db)
	site, olt := distributionFixture(t, db)
	odc, err := service.CreateODCWithFeeds(
		ODCInput{SiteID: site.ID, Code: "ODC"},
		[]ODCFeedInput{{OLTID: olt.ID, Slot: 1, PortID: 1, SplitterOutputs: 8}},
	)
	require.NoError(t, err)
	feeds, err := service.ODCFeedsFor(odc.ID)
	require.NoError(t, err)
	require.Len(t, feeds, 1)

	require.NoError(t, service.SetODCFeedRoute(feeds[0].ID, []models.RoutePoint{
		{Lat: -6.4000, Lng: 107.0000},
		{Lat: -6.4100, Lng: 107.0000},
	}))

	after, err := service.ODCFeedsFor(odc.ID)
	require.NoError(t, err)
	assert.InDelta(t, 1112, after[0].RouteMeters, 20)
}
