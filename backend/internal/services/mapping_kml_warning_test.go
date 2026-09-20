package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// A cable dropped for a missing endpoint must not disappear silently: the
// person downloading the export has to be told how many were left out and
// why, in a sentence that reads plainly without needing to know the word
// "dangling". The reason is named in the file too (Document.Description),
// so the information survives even for someone who only opens the KMZ.
func TestBuildKMLWarnsWhenACableIsSkippedForAMissingEndpoint(t *testing.T) {
	edges := []models.MappingEdge{
		{EdgeID: "E-dangling", Source: "ghost", Target: "ODP-01"},
	}

	kml, warning, err := buildKML(fixtureNodes(), edges)

	require.NoError(t, err)
	assert.Contains(t, warning, "1 kabel dilewati")
	assert.Contains(t, warning, "sudah dihapus dari peta")
	root := parseKML(t, kml)
	assert.Contains(t, root.Document.Description, warning)
	assert.Contains(t, root.Document.Description, "E-dangling",
		"the skipped cable's id must be named in the file, not only in the warning sentence")
}

func TestBuildKMLCountsEveryCableSkipped(t *testing.T) {
	edges := []models.MappingEdge{
		{EdgeID: "E-1", Source: "ghost-a", Target: "ODP-01"},
		{EdgeID: "E-2", Source: "ODC-01", Target: "ghost-b"},
	}

	_, warning, err := buildKML(fixtureNodes(), edges)

	require.NoError(t, err)
	assert.Contains(t, warning, "2 kabel dilewati")
}

// The other half of the same rule: a one-sided test that only checks the
// warning appears would still pass a version that always warns regardless
// of input. A clean export must say nothing extra, in the sentence and in
// the file.
func TestBuildKMLHasNoWarningWhenEveryCableSurvives(t *testing.T) {
	edges := []models.MappingEdge{
		{EdgeID: "E-ok", Source: "ODC-01", Target: "ODP-01"},
	}

	kml, warning, err := buildKML(fixtureNodes(), edges)

	require.NoError(t, err)
	assert.Empty(t, warning)
	root := parseKML(t, kml)
	assert.Empty(t, root.Document.Description)
}

func TestBuildKMLHasNoWarningWhenThereAreNoCablesAtAll(t *testing.T) {
	_, warning, err := buildKML(fixtureNodes(), nil)

	require.NoError(t, err)
	assert.Empty(t, warning)
}
