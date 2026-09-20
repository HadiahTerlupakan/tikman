package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func extData(pairs ...string) *kmlExtendedData {
	var data []kmlData
	for i := 0; i+1 < len(pairs); i += 2 {
		data = append(data, kmlData{Name: pairs[i], Value: pairs[i+1]})
	}
	return &kmlExtendedData{Data: data}
}

func pointPlacemark(name, folder string, lat, lng float64, ext *kmlExtendedData) rawPlacemark {
	return rawPlacemark{
		Folder: folder,
		Placemark: kmlPlacemark{
			Name: name, ExtendedData: ext,
			Point: &kmlPoint{Coordinates: coordinate(lat, lng)},
		},
	}
}

func linePlacemark(name, folder string, coords string, ext *kmlExtendedData) rawPlacemark {
	return rawPlacemark{
		Folder: folder,
		Placemark: kmlPlacemark{
			Name: name, ExtendedData: ext,
			LineString: &kmlLineString{Coordinates: coords},
		},
	}
}

// This is exactly what our own export writes for a node - see
// nodeExtendedData in mapping_kml_node.go - so a re-imported export must be
// read back exactly, not guessed at.
func TestClassifyPlacemarksUsesExtendedDataWhenPresent(t *testing.T) {
	ext := extData("node_id", "ODP-01", "type", "odp", "capacity", "8",
		"splitter", "1:8", "pppoe", "", "serial_number", "SN-1", "olt_id", "")
	raw := []rawPlacemark{pointPlacemark("ODP Satu", "ODP", -6.21, 106.81, ext)}

	nodes, edges, issues := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.Empty(t, edges)
	assert.Empty(t, issues)
	n := nodes[0]
	assert.Equal(t, "ODP-01", n.NodeID)
	assert.Equal(t, models.NodeODP, n.Type)
	assert.Equal(t, 8, n.Capacity)
	assert.Equal(t, "1:8", n.Splitter)
	assert.Equal(t, "SN-1", n.SerialNumber)
	assert.False(t, n.Blocked)
	assert.True(t, n.Include)
	assert.Contains(t, n.Reason, "ekspor")
}

// A server node mirrors a real OLT (migrations/55_olt_map_node.sql) and must
// never be fabricated by import, even when the file plainly says "server" -
// see ImportedNode.Blocked's own doc comment for why.
func TestClassifyPlacemarksBlocksAServerNodeFromExtendedData(t *testing.T) {
	ext := extData("node_id", "SERVER-01", "type", "server")
	raw := []rawPlacemark{pointPlacemark("OLT Satu", "Server / OLT", -6.2, 106.8, ext)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.True(t, nodes[0].Blocked)
	assert.False(t, nodes[0].Include)
	assert.NotEmpty(t, nodes[0].BlockedReason)
}

func TestClassifyPlacemarksGuessesTypeFromFolderName(t *testing.T) {
	raw := []rawPlacemark{pointPlacemark("Kotak Jalan Melati", "ODP", -6.21, 106.81, nil)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.Equal(t, models.NodeODP, nodes[0].Type)
	assert.Contains(t, nodes[0].Reason, "folder")
	assert.True(t, nodes[0].Include)
	// No ExtendedData at all: the placemark's own name is the only candidate
	// for a node_id, so it is reused verbatim rather than left blank.
	assert.Equal(t, "Kotak Jalan Melati", nodes[0].NodeID)
}

// A field survey's own folders are not always foldered by kind (a vendor
// tool may group by site instead) - the placemark's own name is the fallback
// this ISP's naming convention (ODP-01, ODC-CENTRAL) makes usable.
func TestClassifyPlacemarksGuessesTypeFromPlacemarkName(t *testing.T) {
	raw := []rawPlacemark{pointPlacemark("ODP-07", "Titik Survei", -6.21, 106.81, nil)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.Equal(t, models.NodeODP, nodes[0].Type)
	assert.Contains(t, nodes[0].Reason, "nama titik")
}

// Never guess silently: when neither the folder nor the name says anything,
// the row must say so and stay excluded until a person picks a type.
func TestClassifyPlacemarksLeavesTypeBlankWhenNothingMatches(t *testing.T) {
	raw := []rawPlacemark{pointPlacemark("Lokasi A", "Titik", -6.21, 106.81, nil)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.Equal(t, models.NodeType(""), nodes[0].Type)
	assert.False(t, nodes[0].Include)
	assert.Contains(t, nodes[0].Reason, "manual")
}

func TestClassifyPlacemarksFlagsUnsupportedShapeAsIssue(t *testing.T) {
	raw := []rawPlacemark{{Folder: "Lain", Placemark: kmlPlacemark{Name: "Poligon"}}}

	nodes, edges, issues := classifyPlacemarks(raw)

	assert.Empty(t, nodes)
	assert.Empty(t, edges)
	require.Len(t, issues, 1)
	assert.Equal(t, "Poligon", issues[0].Name)
}

func TestClassifyPlacemarksFlagsInvalidPointCoordinatesAsIssue(t *testing.T) {
	raw := []rawPlacemark{{Folder: "ODP", Placemark: kmlPlacemark{
		Name: "Rusak", Point: &kmlPoint{Coordinates: "tidak-valid"},
	}}}

	nodes, _, issues := classifyPlacemarks(raw)

	assert.Empty(t, nodes)
	require.Len(t, issues, 1)
	assert.Contains(t, issues[0].Reason, "oordinat")
}

func TestClassifyPlacemarksFlagsDegenerateLineStringAsIssue(t *testing.T) {
	raw := []rawPlacemark{linePlacemark("E-1", "Kabel", "106.8,-6.2,0", nil)}

	_, edges, issues := classifyPlacemarks(raw)

	assert.Empty(t, edges)
	require.Len(t, issues, 1)
}

// Mirrors edgeExtendedData in mapping_kml_edge.go exactly, including that
// Waypoints must come back as only the corners - source and target are
// re-derived from Source/Target, not carried twice.
func TestClassifyPlacemarksEdgeUsesExtendedDataWhenPresent(t *testing.T) {
	ext := extData("edge_id", "E-1", "source", "ODC-01", "target", "ODP-01",
		"fiber_type", "distribution", "distance", "543.21")
	coords := coordinate(-6.20, 106.80) + " " + coordinate(-6.21, 106.81) + " " + coordinate(-6.25, 106.85)
	raw := []rawPlacemark{linePlacemark("E-1", "Kabel", coords, ext)}

	_, edges, issues := classifyPlacemarks(raw)

	require.Empty(t, issues)
	require.Len(t, edges, 1)
	e := edges[0]
	assert.Equal(t, "E-1", e.EdgeID)
	assert.Equal(t, "ODC-01", e.Source)
	assert.Equal(t, "ODP-01", e.Target)
	assert.Equal(t, models.FiberDistribution, e.FiberType)
	assert.InDelta(t, 543.21, e.Distance, 1e-9)
	require.Len(t, e.Waypoints, 1)
	assert.InDelta(t, -6.21, e.Waypoints[0].Lat, 1e-6)
	assert.InDelta(t, 106.81, e.Waypoints[0].Lng, 1e-6)
	assert.True(t, e.Include)
}

// The realistic field-survey shape: a technician places boxes, then draws a
// line between them with no attributes at all. The line's own endpoints are
// the only clue, so the nearest already-placed node in the same file is what
// has to answer "which two boxes does this connect". Order in the file must
// not matter, so the edge placemark is listed first here.
func TestClassifyPlacemarksEdgeGuessesEndpointsFromNearestNodeInSameFile(t *testing.T) {
	coords := coordinate(-6.200010, 106.800010) + " " + coordinate(-6.250010, 106.850010)
	raw := []rawPlacemark{
		linePlacemark("", "Kabel", coords, nil),
		pointPlacemark("ODC-01", "ODC", -6.20, 106.80, nil),
		pointPlacemark("ODP-01", "ODP", -6.25, 106.85, nil),
	}

	_, edges, issues := classifyPlacemarks(raw)

	require.Empty(t, issues)
	require.Len(t, edges, 1)
	e := edges[0]
	assert.Equal(t, "ODC-01", e.Source)
	assert.Equal(t, "ODP-01", e.Target)
	assert.Equal(t, models.FiberDistribution, e.FiberType, "ODC to ODP has only one sensible default")
	assert.Equal(t, "ODC-01--ODP-01", e.EdgeID, "falls back to the frontend's own source--target convention")
	assert.True(t, e.Include)
	assert.Contains(t, e.Reason, "cocok")
}

// Nothing on the map is close enough to guess from: the row must say so and
// stay excluded, never invent an endpoint out of thin air.
func TestClassifyPlacemarksEdgeLeavesEndpointsUnresolvedWhenNoNodeIsClose(t *testing.T) {
	coords := coordinate(1.0, 1.0) + " " + coordinate(2.0, 2.0)
	raw := []rawPlacemark{
		linePlacemark("Garis Sendirian", "Kabel", coords, nil),
		pointPlacemark("ODC-01", "ODC", -6.20, 106.80, nil),
	}

	_, edges, _ := classifyPlacemarks(raw)

	require.Len(t, edges, 1)
	assert.Equal(t, "", edges[0].Source)
	assert.Equal(t, "", edges[0].Target)
	assert.True(t, edges[0].Unresolved)
	assert.False(t, edges[0].Include)
}

// The other cascade pairing guessFiberType has to assume a default for -
// ODC-to-ODC is covered by the round-trip test's own feeder/distribution
// path already, but nothing else exercises this one.
func TestClassifyPlacemarksGuessesODPToODPCascadeWithoutRatioByDefault(t *testing.T) {
	coords := coordinate(-6.200010, 106.800010) + " " + coordinate(-6.250010, 106.850010)
	raw := []rawPlacemark{
		linePlacemark("", "Kabel", coords, nil),
		pointPlacemark("ODP-A", "ODP", -6.20, 106.80, nil),
		pointPlacemark("ODP-B", "ODP", -6.25, 106.85, nil),
	}

	_, edges, _ := classifyPlacemarks(raw)

	require.Len(t, edges, 1)
	assert.Equal(t, models.FiberODPToODP, edges[0].FiberType)
	assert.Contains(t, edges[0].Reason, "tanpa rasio splitter")
}

// A very long placemark name must not overflow mapping_nodes.node_id's
// varchar(64) when it is reused as the id.
func TestClassifyPlacemarksTruncatesAnOverlongGuessedNodeID(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "x"
	}
	raw := []rawPlacemark{pointPlacemark(long, "ODP", -6.2, 106.8, nil)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.LessOrEqual(t, len([]rune(nodes[0].NodeID)), 64)
}
