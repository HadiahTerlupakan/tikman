package services

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// A shallow "does the byte slice contain the string doc.kml" proves nothing
// about whether this is actually a zip - only that archive/zip.NewReader
// accepts it and hands back that one entry does.
func TestBuildKMZProducesAZipContainingExactlyDocKML(t *testing.T) {
	kmz, _, err := BuildKMZ(nil, nil)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err, "must be a well-formed zip archive")
	require.Len(t, zr.File, 1)
	assert.Equal(t, "doc.kml", zr.File[0].Name)
}

func TestBuildKMZsDocKMLIsWellFormedXML(t *testing.T) {
	kmz, _, err := BuildKMZ(
		[]models.MappingNode{minimalOdpNode()},
		[]models.MappingEdge{{EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01"}},
	)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err)
	f, err := zr.File[0].Open()
	require.NoError(t, err)
	defer f.Close()

	var root kmlRoot
	require.NoError(t, xml.NewDecoder(f).Decode(&root), "doc.kml must parse as XML")
	assert.Equal(t, "Peta Jaringan", root.Document.Name)
}

func TestExportKMZReadsTheWholeMapFromTheDatabase(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(minimalOdpNode())
	require.NoError(t, err)

	kmz, _, err := s.ExportKMZ()
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err)
	f, err := zr.File[0].Open()
	require.NoError(t, err)
	defer f.Close()
	var root kmlRoot
	require.NoError(t, xml.NewDecoder(f).Decode(&root))
	findPlacemark(t, findFolder(t, root, "ODP"), "ODP Depan Masjid")
}

func TestExportKMZOnAnEmptyMapIsStillAValidKMZ(t *testing.T) {
	s := mappingSetup(t)

	kmz, _, err := s.ExportKMZ()

	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
}

// The database round trip for the same rule mapping_kml_warning_test.go
// checks against buildKML directly: a cable orphaned by deleting its node
// must be reported, not just quietly missing from the file.
func TestExportKMZPropagatesTheWarningFromTheDatabase(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(models.MappingNode{
		NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC", Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)
	_, err = s.CreateNode(odpNode("ODP-01", "Satu"))
	require.NoError(t, err)
	_, err = s.CreateEdge(models.MappingEdge{
		EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01", FiberType: models.FiberDistribution,
	})
	require.NoError(t, err)
	require.NoError(t, s.DeleteNode("ODC-01"))

	_, warning, err := s.ExportKMZ()

	require.NoError(t, err)
	assert.Contains(t, warning, "1 kabel dilewati")
}

// The paired negative case: nothing was orphaned, so ExportKMZ must not
// report anything - guards against a version that always warns.
func TestExportKMZReportsNoWarningOnACleanMap(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(minimalOdpNode())
	require.NoError(t, err)

	_, warning, err := s.ExportKMZ()

	require.NoError(t, err)
	assert.Empty(t, warning)
}
