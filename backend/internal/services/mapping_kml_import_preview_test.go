package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func kmzFromKML(t *testing.T, kml string) []byte {
	t.Helper()
	return buildTestKMZ(t, kmzKMLEntry, []byte(kml))
}

// The whole point of a preview: it answers what was found, and the database
// is exactly as it was before the request.
func TestPreviewImportReportsCountsAndLeavesTheDatabaseUntouched(t *testing.T) {
	s := mappingSetup(t)

	preview, err := s.PreviewImport(kmzFromKML(t, minimalKML))

	require.NoError(t, err)
	assert.Equal(t, 2, preview.TotalPlacemarks)
	require.Len(t, preview.Nodes, 1)
	require.Len(t, preview.Edges, 1)

	nodes, err := s.ListNodes()
	require.NoError(t, err)
	edges, err := s.ListEdges()
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, edges)
}

// Two placemarks in the same file sharing an id is a collision the database
// alone cannot see - nothing has been written yet for it to check against.
func TestPreviewImportFlagsADuplicateNodeIDWithinTheSameFile(t *testing.T) {
	s := mappingSetup(t)
	kml := `<kml><Document><Folder><name>ODP</name>
<Placemark><name>ODP-01</name><Point><coordinates>106.8,-6.2,0</coordinates></Point></Placemark>
<Placemark><name>ODP-01</name><Point><coordinates>106.9,-6.3,0</coordinates></Point></Placemark>
</Folder></Document></kml>`

	preview, err := s.PreviewImport(kmzFromKML(t, kml))

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 2)
	assert.False(t, preview.Nodes[0].Conflict, "the first row claims the id")
	assert.True(t, preview.Nodes[1].Conflict, "the second row repeats it")
	assert.False(t, preview.Nodes[1].Include)
}

func TestPreviewImportFlagsANodeIDThatAlreadyExistsOnTheMap(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODP-01", models.NodeODP, 8)
	kml := `<kml><Document><Folder><name>ODP</name>
<Placemark><name>ODP-01</name><Point><coordinates>106.8,-6.2,0</coordinates></Point></Placemark>
</Folder></Document></kml>`

	preview, err := s.PreviewImport(kmzFromKML(t, kml))

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	assert.True(t, preview.Nodes[0].Conflict)
	assert.Contains(t, preview.Nodes[0].ConflictReason, "ODP-01")
	assert.False(t, preview.Nodes[0].Include)
}

// A parse failure (not a database concern at all) must still answer
// cleanly - PreviewImport is the handler's whole contract for a bad upload.
func TestPreviewImportPropagatesAParseFailure(t *testing.T) {
	s := mappingSetup(t)

	_, err := s.PreviewImport([]byte("bukan kmz"))

	assert.Error(t, err)
}

// A placemark neither a node nor a cable must still be counted, so the
// preview's totals reconcile with what the file actually held.
func TestPreviewImportSurfacesIssuesFromUnsupportedPlacemarks(t *testing.T) {
	s := mappingSetup(t)
	kml := `<kml><Document><Folder><name>Lain</name>
<Placemark><name>Area Layanan</name></Placemark>
</Folder></Document></kml>`

	preview, err := s.PreviewImport(kmzFromKML(t, kml))

	require.NoError(t, err)
	assert.Equal(t, 1, preview.TotalPlacemarks)
	assert.Empty(t, preview.Nodes)
	require.Len(t, preview.Issues, 1)
	assert.Equal(t, "Area Layanan", preview.Issues[0].Name)
}
