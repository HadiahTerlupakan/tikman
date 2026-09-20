package services

import (
	"archive/zip"
	"bytes"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
)

// kmzKMLEntry is the file name KML itself specifies for the document a KMZ
// archive wraps; Google Earth and every other reader looks for this name.
const kmzKMLEntry = "doc.kml"

// BuildKMZ zips buildKML's output into the .kmz Google Earth expects to open
// directly - a bare .kml would work too, but every contractor's copy of
// Earth is already set up to double-click a .kmz. The warning return is
// buildKML's own, passed through unchanged.
func BuildKMZ(nodes []models.MappingNode, edges []models.MappingEdge) ([]byte, string, error) {
	kml, warning, err := buildKML(nodes, edges)
	if err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create(kmzKMLEntry)
	if err != nil {
		return nil, "", fmt.Errorf("create %s: %w", kmzKMLEntry, err)
	}
	if _, err := entry.Write(kml); err != nil {
		return nil, "", fmt.Errorf("write %s: %w", kmzKMLEntry, err)
	}
	if err := zw.Close(); err != nil {
		return nil, "", fmt.Errorf("close kmz: %w", err)
	}
	return buf.Bytes(), warning, nil
}

// ExportKMZ answers the whole map as a KMZ in one shot rather than paged - a
// plant map is drawn a handful of boxes at a time in the field, so even a
// large one is a small download by web standards. The warning return is
// empty on a clean export; MappingHandler.ExportKMZ turns a non-empty one
// into a response header.
func (s *MappingService) ExportKMZ() ([]byte, string, error) {
	nodes, err := s.ListNodes()
	if err != nil {
		return nil, "", err
	}
	edges, err := s.ListEdges()
	if err != nil {
		return nil, "", err
	}
	return BuildKMZ(nodes, edges)
}
