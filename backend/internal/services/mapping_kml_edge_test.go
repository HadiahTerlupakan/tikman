package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

func fixtureNodes() []models.MappingNode {
	return []models.MappingNode{
		{NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC Satu", Latitude: -6.20, Longitude: 106.80},
		{NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Satu", Latitude: -6.25, Longitude: 106.85},
	}
}

// The bug this system already shipped once: a cable's length (and here, its
// shape) measured over the corners alone, so a straight run with no corners
// recorded as a point rather than the source-to-target line it actually is.
func TestEdgePlacemarkLineStringOrdersSourceCornersTarget(t *testing.T) {
	waypoints := datatypes.JSON(`[{"lat":-6.21,"lng":106.81},{"lat":-6.23,"lng":106.83}]`)
	edge := models.MappingEdge{
		EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01",
		FiberType: models.FiberDistribution, Waypoints: waypoints,
	}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{edge}))
	pm := findPlacemark(t, findFolder(t, root, "Kabel"), "E-1")

	want := coordinate(-6.20, 106.80) + " " +
		coordinate(-6.21, 106.81) + " " +
		coordinate(-6.23, 106.83) + " " +
		coordinate(-6.25, 106.85)
	assert.NotNil(t, pm.LineString, "a cable placemark must carry a LineString, not a Point")
	assert.Nil(t, pm.Point)
	assert.Equal(t, want, pm.LineString.Coordinates)
}

func TestEdgePlacemarkWithNoWaypointsIsJustTheTwoEndpoints(t *testing.T) {
	edge := models.MappingEdge{EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01"}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{edge}))
	pm := findPlacemark(t, findFolder(t, root, "Kabel"), "E-1")

	want := coordinate(-6.20, 106.80) + " " + coordinate(-6.25, 106.85)
	assert.Equal(t, want, pm.LineString.Coordinates)
}

// Node deletion never cascades to edges (migration 53), so a cable naming a
// node that is gone is a normal state - cableMath.ts's edgePath skips
// drawing it rather than erroring, and this is that same choice in Go.
func TestEdgeIsSkippedWhenItsSourceNoLongerExists(t *testing.T) {
	edge := models.MappingEdge{EdgeID: "E-1", Source: "ghost", Target: "ODP-01"}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{edge}))

	_, found := findFolderOk(root, "Kabel")
	assert.False(t, found)
}

func TestEdgeIsSkippedWhenItsTargetNoLongerExists(t *testing.T) {
	edge := models.MappingEdge{EdgeID: "E-1", Source: "ODC-01", Target: "ghost"}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{edge}))

	_, found := findFolderOk(root, "Kabel")
	assert.False(t, found)
}

// A survivor and a dangling edge in the same export: the dangling one must
// not take the whole cable folder down with it.
func TestASurvivingEdgeIsKeptWhenAnotherEdgeDangles(t *testing.T) {
	edges := []models.MappingEdge{
		{EdgeID: "E-dangling", Source: "ghost", Target: "ODP-01"},
		{EdgeID: "E-ok", Source: "ODC-01", Target: "ODP-01"},
	}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), edges))
	folder := findFolder(t, root, "Kabel")

	assert.Len(t, folder.Placemarks, 1)
	findPlacemark(t, folder, "E-ok")
}

func TestEdgeExtendedDataRoundTripsEveryModelField(t *testing.T) {
	edge := models.MappingEdge{
		EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01",
		FiberType: models.FiberDistribution, Distance: 123.5,
	}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{edge}))
	pm := findPlacemark(t, findFolder(t, root, "Kabel"), "E-1")

	assert.Equal(t, "E-1", extendedDataValue(t, pm, "edge_id"))
	assert.Equal(t, "ODC-01", extendedDataValue(t, pm, "source"))
	assert.Equal(t, "ODP-01", extendedDataValue(t, pm, "target"))
	assert.Equal(t, "distribution", extendedDataValue(t, pm, "fiber_type"))
	assert.Equal(t, "123.5", extendedDataValue(t, pm, "distance"))
}

func TestEdgeDescriptionIncludesNotesOnlyWhenSet(t *testing.T) {
	bare := models.MappingEdge{EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01", FiberType: models.FiberDistribution}
	withNotes := models.MappingEdge{EdgeID: "E-2", Source: "ODC-01", Target: "ODP-01", FiberType: models.FiberDrop, Notes: "sudah diperbaiki"}

	root := parseKML(t, mustBuildKML(t, fixtureNodes(), []models.MappingEdge{bare, withNotes}))
	folder := findFolder(t, root, "Kabel")

	barePm := findPlacemark(t, folder, "E-1")
	notedPm := findPlacemark(t, folder, "E-2")
	assert.Contains(t, barePm.Description, "Jenis: Distribusi")
	assert.NotContains(t, barePm.Description, "Catatan")
	assert.Contains(t, notedPm.Description, "Jenis: Drop")
	assert.Contains(t, notedPm.Description, "Catatan: sudah diperbaiki")
}
