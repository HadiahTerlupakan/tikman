package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every other test in this package asserts on the Go slices, where
// assert.Empty and len() treat a nil slice and an empty one as the same
// thing. The browser never sees those: it sees JSON, and encoding/json
// writes a nil slice as null rather than []. ImportKmzModal reads
// preview.nodes.length, preview.edges.length and preview.issues.length
// directly, so a null there is not a missing count - it is a TypeError that
// replaces the whole preview with an error screen.
//
// That made the ordinary case the broken one: a file whose placemarks all
// resolve produces no issues at all, so Issues stayed nil and the cleanest
// possible import was the one that could not be previewed. Only a test that
// crosses the JSON boundary can see this, which is why it lives in its own
// file rather than beside the classification tests.
func TestImportPreviewMarshalsEveryListAsAnArrayNeverNull(t *testing.T) {
	cases := []struct {
		name string
		raw  []rawPlacemark
	}{
		{
			// The reported failure: points and lines that all resolve, so
			// nothing lands in Issues.
			name: "a file that resolves completely leaves no issues",
			raw: []rawPlacemark{
				pointPlacemark("ODP Satu", "ODP", -6.21, 106.81, nil),
				pointPlacemark("ODP Dua", "ODP", -6.22, 106.82, nil),
			},
		},
		{
			// Nothing at all: Nodes and Issues are both nil here, so one
			// case pins both fields at once.
			name: "an empty file leaves every list empty",
			raw:  []rawPlacemark{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, edges, issues := classifyPlacemarks(tc.raw)

			body, err := json.Marshal(ImportPreview{
				Nodes:           nodes,
				Edges:           edges,
				Issues:          issues,
				TotalPlacemarks: len(tc.raw),
			})
			require.NoError(t, err)

			assert.NotContains(t, string(body), `"nodes":null`)
			assert.NotContains(t, string(body), `"edges":null`)
			assert.NotContains(t, string(body), `"issues":null`)
		})
	}
}
