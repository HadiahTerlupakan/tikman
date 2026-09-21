package services

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

func nodeByID(t *testing.T, nodes []ImportedNode, id string) ImportedNode {
	t.Helper()
	for _, n := range nodes {
		if n.NodeID == id {
			return n
		}
	}
	t.Fatalf("no imported node %q in %+v", id, nodes)
	return ImportedNode{}
}

// This is the test that matters most: what BuildKMZ writes for a real map
// must come back through parseKMZForImport/classifyPlacemarks as exactly the
// same nodes and cables, field for field - types, capacities, fibre types
// and the corners a cable was actually traced through, not a straight line
// between its ends (mapping_kml_edge_test.go's own TestEdgePlacemark... is
// the export half of the same guarantee).
func TestRoundTripExportThenImportRecoversEveryNodeAndEdgeField(t *testing.T) {
	oltID := uuid.New()
	nodes := []models.MappingNode{
		{
			NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC Satu",
			Latitude: -6.20, Longitude: 106.80, Capacity: 4, Notes: "dekat gerbang",
		},
		{
			NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Satu",
			Latitude: -6.25, Longitude: 106.85, Capacity: 8, Splitter: "1:8",
			PPPoE: "", SerialNumber: "SN-01",
		},
		{
			NodeID: "SERVER-01", Type: models.NodeServer, Name: "OLT Satu",
			Latitude: -6.15, Longitude: 106.75, OLTID: &oltID,
		},
	}
	edges := []models.MappingEdge{
		{
			EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01",
			FiberType: models.FiberDistribution, Distance: 543.21,
			Waypoints: datatypes.JSON(`[{"lat":-6.21,"lng":106.81},{"lat":-6.23,"lng":106.83}]`),
			Notes:     "sudah diperbaiki",
		},
		{
			EdgeID: "E-2", Source: "SERVER-01", Target: "ODC-01",
			FiberType: models.FiberFeeder, Distance: 1200,
		},
	}

	kmz, warning, err := BuildKMZ(nodes, edges)
	require.NoError(t, err)
	require.Empty(t, warning)

	raw, err := parseKMZForImport(kmz)
	require.NoError(t, err)
	gotNodes, gotEdges, issues := classifyPlacemarks(raw)

	assert.Empty(t, issues, "every placemark this export writes must be understood on the way back in")
	require.Len(t, gotNodes, 3)
	require.Len(t, gotEdges, 2)

	odc := nodeByID(t, gotNodes, "ODC-01")
	assert.Equal(t, models.NodeODC, odc.Type)
	assert.Equal(t, 4, odc.Capacity)
	assert.Equal(t, "ODC Satu", odc.Name)
	assert.InDelta(t, -6.20, odc.Latitude, 1e-6)
	assert.InDelta(t, 106.80, odc.Longitude, 1e-6)
	assert.Equal(t, "dekat gerbang", odc.Notes)
	assert.True(t, odc.Include)
	assert.False(t, odc.Blocked)

	odp := nodeByID(t, gotNodes, "ODP-01")
	assert.Equal(t, models.NodeODP, odp.Type)
	assert.Equal(t, 8, odp.Capacity)
	assert.Equal(t, "1:8", odp.Splitter)
	assert.Equal(t, "SN-01", odp.SerialNumber)

	// The server node round-trips as *recognised*, but is never something
	// import itself would create - see ImportedNode.Blocked.
	server := nodeByID(t, gotNodes, "SERVER-01")
	assert.Equal(t, models.NodeServer, server.Type)
	assert.True(t, server.Blocked)
	assert.False(t, server.Include)

	byID := map[string]ImportedEdge{}
	for _, e := range gotEdges {
		byID[e.EdgeID] = e
	}
	e1 := byID["E-1"]
	assert.Equal(t, "ODC-01", e1.Source)
	assert.Equal(t, "ODP-01", e1.Target)
	assert.Equal(t, models.FiberDistribution, e1.FiberType)
	assert.InDelta(t, 543.21, e1.Distance, 1e-9)
	require.Len(t, e1.Waypoints, 2, "the traced corners, not the endpoints edgePath prepended/appended on export")
	assert.InDelta(t, -6.21, e1.Waypoints[0].Lat, 1e-6)
	assert.InDelta(t, 106.81, e1.Waypoints[0].Lng, 1e-6)
	assert.InDelta(t, -6.23, e1.Waypoints[1].Lat, 1e-6)
	assert.InDelta(t, 106.83, e1.Waypoints[1].Lng, 1e-6)
	assert.Equal(t, "sudah diperbaiki", e1.Notes)
	assert.True(t, e1.Include)

	e2 := byID["E-2"]
	assert.Equal(t, "SERVER-01", e2.Source)
	assert.Equal(t, "ODC-01", e2.Target)
	assert.Equal(t, models.FiberFeeder, e2.FiberType)
	assert.InDelta(t, 1200, e2.Distance, 1e-9)
	assert.Empty(t, e2.Waypoints, "a cable exported with no corners must not gain phantom ones on the way back")
}

// buildLargePlant makes nodes+edges connected in a simple chain (each edge's
// source is the previous node, target the next) - realistic enough to
// exercise edgeFromExtendedData's real Source/Target fields, not a
// contrived shape chosen to be cheap to classify.
func buildLargePlant(nodeCount int) ([]models.MappingNode, []models.MappingEdge) {
	nodes := make([]models.MappingNode, nodeCount)
	for i := range nodes {
		nodes[i] = models.MappingNode{
			NodeID: fmt.Sprintf("ODP-%04d", i), Type: models.NodeODP,
			Name: fmt.Sprintf("ODP %d", i), Latitude: -6.2 + float64(i)*0.0001, Longitude: 106.8 + float64(i)*0.0001,
		}
	}
	edges := make([]models.MappingEdge, nodeCount-1)
	for i := range edges {
		edges[i] = models.MappingEdge{
			EdgeID: fmt.Sprintf("E-%04d", i), Source: nodes[i].NodeID, Target: nodes[i+1].NodeID,
			FiberType: models.FiberDistribution,
		}
	}
	return nodes, edges
}

// A full plant at roughly this system's own historical "2,000 placemarks"
// reference point has close to as many cables as boxes (a real network is
// not a bag of disconnected nodes), so 2,000 nodes plus 1,999 cables
// connecting them is 3,999 placemarks in one file - past maxKMLPlacemarks
// when that counted nodes and edges together with no headroom for the
// second half of a real network. ExportKMZ has no cap of its own, so this
// plant exports happily and then cannot be read back by this system's own
// importer - and round-tripping a full export is a real, confirmed usage
// (import source A), not a hypothetical edge case.
func TestRoundTripHandlesAFullPlantAtTheHistoricalPlacemarkReferenceSize(t *testing.T) {
	nodes, edges := buildLargePlant(2000)
	kmz, warning, err := BuildKMZ(nodes, edges)
	require.NoError(t, err)
	require.Empty(t, warning, "ExportKMZ itself carries no placemark-count cap")

	raw, err := parseKMZForImport(kmz)

	require.NoError(t, err, "this system's own maximal realistic export must read back through its own importer")
	gotNodes, gotEdges, issues := classifyPlacemarks(raw)
	assert.Empty(t, issues)
	assert.Len(t, gotNodes, 2000)
	assert.Len(t, gotEdges, 1999)
}

// The other half of the round trip: importing a file that is a straight
// re-export of the current map must be recognised as already there, not
// offered as new rows to create - every id collides with what PreviewImport
// itself is looking at.
func TestRoundTripReimportingTheSameMapFlagsEveryRowAsAConflict(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 4)
	plantNode(t, s, "ODP-01", models.NodeODP, 8)
	require.NoError(t, drawCable(s, "E-1", "ODC-01", "ODP-01", models.FiberDistribution))

	nodes, err := s.ListNodes()
	require.NoError(t, err)
	edges, err := s.ListEdges()
	require.NoError(t, err)
	kmz, _, err := BuildKMZ(nodes, edges)
	require.NoError(t, err)

	preview, err := s.PreviewImport(kmz)
	require.NoError(t, err)

	require.Len(t, preview.Nodes, 2)
	require.Len(t, preview.Edges, 1)
	for _, n := range preview.Nodes {
		assert.True(t, n.Conflict, "node %s should collide with what is already on the map", n.NodeID)
		assert.False(t, n.Include)
	}
	for _, e := range preview.Edges {
		assert.True(t, e.Conflict, "edge %s should collide with what is already on the map", e.EdgeID)
		assert.False(t, e.Include)
	}
}
