package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The map's own four colours (frontend/.../mappingLabels.ts NODE_COLORS),
// reordered by hand once here so every other test can assert against a
// literal rather than re-deriving it.
func TestKmlColorReordersRRGGBBToAABBGGRR(t *testing.T) {
	cases := []struct{ hex, want string }{
		{"#8b5cf6", "fff65c8b"}, // server
		{"#3b82f6", "fff6823b"}, // odc
		{"#06b6d4", "ffd4b606"}, // odp
		{"#22c55e", "ff5ec522"}, // ont
	}
	for _, c := range cases {
		assert.Equal(t, c.want, kmlColor(c.hex), c.hex)
	}
}

// Getting this backwards is the one mistake KML authors reliably make:
// coordinates read "longitude,latitude", not the field order this system
// stores them in everywhere else.
func TestCoordinateOrdersLongitudeBeforeLatitude(t *testing.T) {
	assert.Equal(t, "106.810000,-6.210000,0", coordinate(-6.21, 106.81))
}

func TestBuildKMLDeclaresTheOpenGISNamespace(t *testing.T) {
	root := parseKML(t, mustBuildKML(t, nil, nil))
	assert.Equal(t, kmlNamespace, root.Xmlns)
}

func TestBuildKMLGroupsNodesIntoOneFolderPerType(t *testing.T) {
	nodes := []models.MappingNode{
		{NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Satu", Latitude: -6.2, Longitude: 106.8},
		{NodeID: "ODP-02", Type: models.NodeODP, Name: "ODP Dua", Latitude: -6.21, Longitude: 106.81},
		{NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC Satu", Latitude: -6.22, Longitude: 106.82},
	}

	root := parseKML(t, mustBuildKML(t, nodes, nil))

	assert.Len(t, root.Document.Folders, 2, "one folder per type present - no empty Server/ONT folders")
	assert.Len(t, findFolder(t, root, "ODP").Placemarks, 2)
	assert.Len(t, findFolder(t, root, "ODC").Placemarks, 1)
}

func TestBuildKMLOmitsTheCableFolderWhenNoEdgeSurvives(t *testing.T) {
	edges := []models.MappingEdge{{EdgeID: "E-1", Source: "ghost-a", Target: "ghost-b"}}

	root := parseKML(t, mustBuildKML(t, nil, edges))

	_, found := findFolderOk(root, "Kabel")
	assert.False(t, found, "both ends are missing, so the cable - and the folder - must not appear")
}

func TestBuildKMLDefinesAStylePerNodeType(t *testing.T) {
	root := parseKML(t, mustBuildKML(t, nil, nil))

	ids := make(map[string]bool)
	for _, s := range root.Document.Styles {
		ids[s.ID] = true
	}
	for _, want := range []string{"node-server", "node-odc", "node-odp", "node-ont"} {
		assert.True(t, ids[want], "missing style %q", want)
	}
}
