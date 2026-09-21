package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// linePlacemark builds a LineString placemark; extData/pointPlacemark (the
// node-side fixtures edge tests also need, for the nodes a cable connects)
// live in mapping_kml_import_infer_test.go.
func linePlacemark(name, folder string, coords string, ext *kmlExtendedData) rawPlacemark {
	return rawPlacemark{
		Folder: folder,
		Placemark: kmlPlacemark{
			Name: name, ExtendedData: ext,
			LineString: &kmlLineString{Coordinates: coords},
		},
	}
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

// A foreign file's ExtendedData carrying only source/target (the two keys
// required to enter this path at all - see classifyEdge) must not silently
// zero its distance while still claiming full export provenance. The file's
// own traced line already says exactly how long the cable is; that is a
// computation, not a guess.
func TestClassifyPlacemarksEdgeWithNoExtendedDataDistanceComputesItFromThePath(t *testing.T) {
	ext := extData("source", "ODC-01", "target", "ODP-01")
	coords := coordinate(-6.20, 106.80) + " " + coordinate(-6.21, 106.81)
	raw := []rawPlacemark{linePlacemark("E-1", "Kabel", coords, ext)}

	_, edges, _ := classifyPlacemarks(raw)

	require.Len(t, edges, 1)
	want := haversineMeters(-6.20, 106.80, -6.21, 106.81)
	assert.InDelta(t, want, edges[0].Distance, 1e-6)
	assert.Contains(t, edges[0].Reason, "dihitung dari garis", "must disclose that distance was computed, not supplied, unlike source/target")
}

// Same shape, for fiber_type: nothing stops a vendor's ExtendedData from
// naming source/target that also happen to resolve to nodes in this same
// file, in which case guessFiberType has exactly what it needs.
func TestClassifyPlacemarksEdgeWithNoExtendedDataFiberTypeGuessesFromResolvedTypes(t *testing.T) {
	ext := extData("source", "ODC-01", "target", "ODP-01")
	coords := coordinate(-6.20, 106.80) + " " + coordinate(-6.21, 106.81)
	raw := []rawPlacemark{
		linePlacemark("E-1", "Kabel", coords, ext),
		pointPlacemark("ODC-01", "ODC", -6.20, 106.80, nil),
		pointPlacemark("ODP-01", "ODP", -6.21, 106.81, nil),
	}

	_, edges, _ := classifyPlacemarks(raw)

	require.Len(t, edges, 1)
	assert.Equal(t, models.FiberDistribution, edges[0].FiberType)
	assert.Contains(t, edges[0].Reason, "distribusi")
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

func TestClassifyPlacemarksBoundsAnOverlongEdgeIDSourceAndTargetFromExtendedData(t *testing.T) {
	long := strings.Repeat("e", 100)
	ext := extData("edge_id", long, "source", long, "target", long)
	raw := []rawPlacemark{linePlacemark("E-1", "Kabel", "106.8,-6.2,0 106.9,-6.3,0", ext)}

	_, edges, _ := classifyPlacemarks(raw)

	require.Len(t, edges, 1)
	assert.LessOrEqual(t, len([]rune(edges[0].EdgeID)), 64)
	assert.LessOrEqual(t, len([]rune(edges[0].Source)), 64)
	assert.LessOrEqual(t, len([]rune(edges[0].Target)), 64)
}

// The exact same trap parseCoordinate was fixed for, one function over:
// strconv.ParseFloat parses "NaN"/"Inf"/"-Inf" with no error, and a NaN/Inf
// Distance reaching an ImportedEdge field fails encoding/json's own Marshal
// after the response status is already written - a 200 with no usable
// body. Negative is rejected too: a cable has no such thing as a negative
// length. All four fall through to pathLengthMeters, the same as a missing
// distance already does.
func TestClassifyPlacemarksEdgeRejectsNonFiniteOrNegativeDistance(t *testing.T) {
	for _, bad := range []string{"NaN", "Inf", "-Inf", "-5"} {
		t.Run(bad, func(t *testing.T) {
			ext := extData("source", "ODC-01", "target", "ODP-01", "distance", bad)
			coords := coordinate(-6.20, 106.80) + " " + coordinate(-6.21, 106.81)
			raw := []rawPlacemark{linePlacemark("E-1", "Kabel", coords, ext)}

			_, edges, _ := classifyPlacemarks(raw)

			require.Len(t, edges, 1)
			want := haversineMeters(-6.20, 106.80, -6.21, 106.81)
			assert.InDelta(t, want, edges[0].Distance, 1e-6)
			assert.Contains(t, edges[0].Reason, "dihitung dari garis")
		})
	}
}
