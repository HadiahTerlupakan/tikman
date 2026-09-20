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
	kmz, err := BuildKMZ(nil, nil)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err, "must be a well-formed zip archive")
	require.Len(t, zr.File, 1)
	assert.Equal(t, "doc.kml", zr.File[0].Name)
}

func TestBuildKMZsDocKMLIsWellFormedXML(t *testing.T) {
	kmz, err := BuildKMZ(
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

	kmz, err := s.ExportKMZ()
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

	kmz, err := s.ExportKMZ()

	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(kmz), int64(len(kmz)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
}
