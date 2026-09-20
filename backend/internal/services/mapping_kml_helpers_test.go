package services

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// mustBuildKML is buildKML with the error already asserted away, for the
// tests whose focus is the document shape rather than error handling.
func mustBuildKML(t *testing.T, nodes []models.MappingNode, edges []models.MappingEdge) []byte {
	t.Helper()
	kml, err := buildKML(nodes, edges)
	require.NoError(t, err)
	return kml
}

// parseKML decodes buildKML's output back into the same struct tree that
// produced it - a shallow "contains this substring" check on XML proves
// almost nothing, since a well-formed document and a corrupt one can share
// any given substring.
func parseKML(t *testing.T, kml []byte) kmlRoot {
	t.Helper()
	var root kmlRoot
	require.NoError(t, xml.Unmarshal(kml, &root))
	return root
}

func findFolder(t *testing.T, root kmlRoot, name string) kmlFolder {
	t.Helper()
	folder, ok := findFolderOk(root, name)
	require.True(t, ok, "no folder named %q in %+v", name, root.Document.Folders)
	return folder
}

func findFolderOk(root kmlRoot, name string) (kmlFolder, bool) {
	for _, f := range root.Document.Folders {
		if f.Name == name {
			return f, true
		}
	}
	return kmlFolder{}, false
}

func findPlacemark(t *testing.T, folder kmlFolder, name string) kmlPlacemark {
	t.Helper()
	for _, p := range folder.Placemarks {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no placemark named %q in folder %q", name, folder.Name)
	return kmlPlacemark{}
}

// extendedDataValue reads back one field the way an importer would have to:
// by its Data/@name, not by array position.
func extendedDataValue(t *testing.T, p kmlPlacemark, name string) string {
	t.Helper()
	require.NotNil(t, p.ExtendedData, "placemark %q has no ExtendedData at all", p.Name)
	for _, d := range p.ExtendedData.Data {
		if d.Name == name {
			return d.Value
		}
	}
	t.Fatalf("placemark %q has no ExtendedData field %q", p.Name, name)
	return ""
}
