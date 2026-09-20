package services

import (
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

// nodeTypeLabel mirrors NODE_LABELS in
// frontend/src/presentation/components/netmap/mappingLabels.ts - the same
// Indonesian type names shown in the map's own popup.
var nodeTypeLabel = map[models.NodeType]string{
	models.NodeServer: "Server / OLT",
	models.NodeODC:    "ODC",
	models.NodeODP:    "ODP",
	models.NodeONT:    "ONT",
}

// nodeFolder groups every node of one kind under a labelled folder. A kind
// with nothing placed yet is left out entirely rather than shown empty.
func nodeFolder(nt models.NodeType, nodes []models.MappingNode) *kmlFolder {
	var placemarks []kmlPlacemark
	for _, n := range nodes {
		if n.Type != nt {
			continue
		}
		placemarks = append(placemarks, nodePlacemark(n))
	}
	if len(placemarks) == 0 {
		return nil
	}
	return &kmlFolder{Name: nodeTypeLabel[nt], Placemarks: placemarks}
}

func nodePlacemark(n models.MappingNode) kmlPlacemark {
	return kmlPlacemark{
		Name:         n.Name,
		Description:  nodeDescription(n),
		StyleURL:     "#" + nodeStyleID(n.Type),
		Point:        &kmlPoint{Coordinates: coordinate(n.Latitude, n.Longitude)},
		ExtendedData: nodeExtendedData(n),
	}
}

// nodeDescription follows NodePopup.tsx's own rule (OptionalFields): a field
// nobody filled renders nothing, never an empty line.
func nodeDescription(n models.MappingNode) string {
	lines := []string{
		"Kode: " + n.NodeID,
		"Jenis: " + nodeTypeLabel[n.Type],
	}
	if n.Capacity > 0 {
		lines = append(lines, fmt.Sprintf("Kapasitas: %d", n.Capacity))
	}
	if n.Splitter != "" {
		lines = append(lines, "Splitter: "+n.Splitter)
	}
	if n.PPPoE != "" {
		lines = append(lines, "PPPoE: "+n.PPPoE)
	}
	if n.SerialNumber != "" {
		lines = append(lines, "Serial: "+n.SerialNumber)
	}
	if n.Notes != "" {
		lines = append(lines, "Catatan: "+n.Notes)
	}
	return strings.Join(lines, "\n")
}

// nodeExtendedData is the machine-readable record a future importer reads
// back, so unlike the description above, every field is present even when
// blank - a blank splitter is still a fact, not a line to hide.
func nodeExtendedData(n models.MappingNode) *kmlExtendedData {
	oltID := ""
	if n.OLTID != nil {
		oltID = n.OLTID.String()
	}
	return &kmlExtendedData{Data: []kmlData{
		{Name: "node_id", Value: n.NodeID},
		{Name: "type", Value: string(n.Type)},
		{Name: "capacity", Value: fmt.Sprintf("%d", n.Capacity)},
		{Name: "splitter", Value: n.Splitter},
		{Name: "pppoe", Value: n.PPPoE},
		{Name: "serial_number", Value: n.SerialNumber},
		{Name: "olt_id", Value: oltID},
	}}
}
