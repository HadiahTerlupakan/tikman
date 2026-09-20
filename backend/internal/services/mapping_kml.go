package services

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

// circleIcon is Google's own plain white circle, the standard base image for
// a coloured IconStyle: Earth tints an icon by multiplying it with the style
// colour, and an icon that already carried colour would be tinted twice.
const circleIcon = "http://maps.google.com/mapfiles/kml/shapes/placemark_circle.png"

// nodeColor mirrors NODE_COLORS in
// frontend/src/presentation/components/netmap/mappingLabels.ts. Go cannot
// import that file, so this is kept in sync by hand - the two must read as
// the same map.
var nodeColor = map[models.NodeType]string{
	models.NodeServer: "#8b5cf6",
	models.NodeODC:    "#3b82f6",
	models.NodeODP:    "#06b6d4",
	models.NodeONT:    "#22c55e",
}

// nodeOrder is deliberate rather than the alphabetical order ListNodes sorts
// by: server first reads as the map's root (eee2fb4), then feeder-side to
// drop-side.
var nodeOrder = []models.NodeType{models.NodeServer, models.NodeODC, models.NodeODP, models.NodeONT}

func nodeStyleID(nt models.NodeType) string { return "node-" + string(nt) }

func nodeStyles() []kmlStyle {
	styles := make([]kmlStyle, 0, len(nodeOrder))
	for _, nt := range nodeOrder {
		styles = append(styles, kmlStyle{
			ID:        nodeStyleID(nt),
			IconStyle: &kmlIconStyle{Color: kmlColor(nodeColor[nt]), Icon: kmlIcon{Href: circleIcon}},
		})
	}
	return styles
}

// cableLineStyle is the one shared look for every cable. Colour-coding by
// fibre type was not asked for; a single visible colour beats Earth's own
// per-session default, which can render as a line invisible against
// satellite imagery.
func cableLineStyle() kmlStyle {
	return kmlStyle{ID: "cable-line", LineStyle: &kmlLineStyle{Color: "ffffffff", Width: 2}}
}

// kmlColor turns a "#rrggbb" map colour into KML's own "aabbggrr" ordering
// (alpha, then blue, green, red), opaque since nothing on this map is ever
// partly transparent. Getting this backwards is the one mistake KML authors
// reliably make.
func kmlColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	return "ff" + hex[4:6] + hex[2:4] + hex[0:2]
}

// coordinate is KML's own "longitude,latitude,altitude" ordering - the
// reverse of how this system stores and reads back lat/lng everywhere else.
// Altitude is always 0: nothing on this map has one.
func coordinate(lat, lng float64) string {
	return fmt.Sprintf("%.6f,%.6f,0", lng, lat)
}

// buildKML assembles the whole document: one folder per node kind so Google
// Earth's sidebar stays navigable, then the cables. The second return value
// is empty for a clean export and a one-sentence Indonesian warning
// otherwise: a cable dropped for a missing endpoint must not disappear
// without telling the person downloading the file. Pure and DB-free, so the
// structural rules - coordinate order, colour, grouping, the dangling-edge
// rule - are all testable without a database.
func buildKML(nodes []models.MappingNode, edges []models.MappingEdge) ([]byte, string, error) {
	nodesByID := make(map[string]models.MappingNode, len(nodes))
	for _, n := range nodes {
		nodesByID[n.NodeID] = n
	}

	doc := kmlDocument{Name: "Peta Jaringan", Styles: nodeStyles()}
	for _, nt := range nodeOrder {
		if folder := nodeFolder(nt, nodes); folder != nil {
			doc.Folders = append(doc.Folders, *folder)
		}
	}
	folder, skipped := edgeFolder(edges, nodesByID)
	if folder != nil {
		doc.Styles = append(doc.Styles, cableLineStyle())
		doc.Folders = append(doc.Folders, *folder)
	}
	warning := skippedCablesMessage(len(skipped))
	if warning != "" {
		// The sentence is also what the HTTP response header carries
		// (mapping_handler_export.go); the file additionally names which
		// cables it is, since a downloaded header is gone the moment the
		// file is opened later.
		doc.Description = warning + ": " + strings.Join(skipped, ", ")
	}

	body, err := xml.MarshalIndent(kmlRoot{Xmlns: kmlNamespace, Document: doc}, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal kml: %w", err)
	}
	return append([]byte(xml.Header), body...), warning, nil
}

// skippedCablesMessage is the one sentence a technician sees when the export
// leaves cables out - empty when nothing was skipped, since a clean export
// must say nothing extra. "Salah satu ujungnya" (one of its ends) rather
// than naming source/target: either end can be the one that is gone, and
// this reads the same regardless, without requiring the reader to know the
// word "dangling".
func skippedCablesMessage(count int) string {
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("%d kabel dilewati karena salah satu ujungnya sudah dihapus dari peta", count)
}
