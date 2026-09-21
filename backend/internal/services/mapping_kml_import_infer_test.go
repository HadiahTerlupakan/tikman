package services

import (
	"strings"
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

// A distribution box outnumbers every other kind of box in this plant, so a
// surveyed point carrying no type hint is overwhelmingly an ODP. Leaving it
// blank made the technician pick the same value dozens of times by hand,
// which is the work this preview exists to remove.
//
// The assumption is still never silent, which was the original rule's real
// point: Reason says the type was assumed rather than read, so the operator
// can see at a glance which rows are inferred and change the few that are
// ODCs before committing.
func TestClassifyPlacemarksAssumesODPWhenNothingSaysOtherwise(t *testing.T) {
	raw := []rawPlacemark{pointPlacemark("Lokasi A", "Titik", -6.21, 106.81, nil)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.Equal(t, models.NodeODP, nodes[0].Type)
	assert.True(t, nodes[0].Include)
	assert.Contains(t, nodes[0].Reason, "dianggap ODP")
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

// strconv.ParseFloat parses "NaN" successfully - a hand-written or vendor
// KML carrying it is not a parse failure, only a bad value, and a NaN
// latitude must not silently become an ImportedNode field: it fails
// encoding/json's own Marshal after the response status is already
// written, so the request answers 200 with no usable body at all.
// parseCoordinate's own unit tests live in mapping_kml_import_fields_test.go;
// this one exercises the same rule end to end, through classifyPlacemarks.
func TestClassifyPlacemarksFlagsANaNCoordinateAsAnIssueNotANode(t *testing.T) {
	raw := []rawPlacemark{{Folder: "ODP", Placemark: kmlPlacemark{
		Name: "Rusak", Point: &kmlPoint{Coordinates: "106.8,NaN,0"},
	}}}

	nodes, _, issues := classifyPlacemarks(raw)

	assert.Empty(t, nodes)
	require.Len(t, issues, 1)
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

// sanitizeID's truncation only ever ran on a guessed node_id. Every other
// string field - on both the guess path and the ExtendedData path, which is
// exactly as unbounded as a placemark's raw <name> or a hand-typed
// ExtendedData value - reached commit unbounded, where Postgres's own
// varchar limits would abort the whole transaction with a 22001 the person
// never sees coming, since the preview showed no problem with it.
func TestClassifyPlacemarksBoundsAnOverlongNameOnEitherPath(t *testing.T) {
	long := strings.Repeat("x", 200)
	ext := extData("node_id", "ODP-01", "type", "odp")

	fromExtendedData, _, _ := classifyPlacemarks([]rawPlacemark{pointPlacemark(long, "ODP", -6.2, 106.8, ext)})
	fromGuess, _, _ := classifyPlacemarks([]rawPlacemark{pointPlacemark(long, "ODP", -6.2, 106.8, nil)})

	require.Len(t, fromExtendedData, 1)
	require.Len(t, fromGuess, 1)
	assert.LessOrEqual(t, len([]rune(fromExtendedData[0].Name)), 120)
	assert.LessOrEqual(t, len([]rune(fromGuess[0].Name)), 120)
}

func TestClassifyPlacemarksBoundsAnOverlongNodeIDFromExtendedData(t *testing.T) {
	ext := extData("node_id", strings.Repeat("x", 100), "type", "odp")
	raw := []rawPlacemark{pointPlacemark("ODP Satu", "ODP", -6.2, 106.8, ext)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.LessOrEqual(t, len([]rune(nodes[0].NodeID)), 64)
}

func TestClassifyPlacemarksBoundsAnOverlongSplitterPPPoEAndSerialFromExtendedData(t *testing.T) {
	long := strings.Repeat("x", 100)
	ext := extData("node_id", "ODP-01", "type", "odp", "splitter", long, "pppoe", long, "serial_number", long)
	raw := []rawPlacemark{pointPlacemark("ODP Satu", "ODP", -6.2, 106.8, ext)}

	nodes, _, _ := classifyPlacemarks(raw)

	require.Len(t, nodes, 1)
	assert.LessOrEqual(t, len([]rune(nodes[0].Splitter)), 16)
	assert.LessOrEqual(t, len([]rune(nodes[0].PPPoE)), 64)
	assert.LessOrEqual(t, len([]rune(nodes[0].SerialNumber)), 64)
}
