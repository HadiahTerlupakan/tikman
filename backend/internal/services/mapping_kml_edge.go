package services

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

// fiberTypeLabel mirrors FIBER_LABELS in
// frontend/src/presentation/components/netmap/mappingLabels.ts.
var fiberTypeLabel = map[models.FiberType]string{
	models.FiberFeeder:        "Feeder",
	models.FiberDistribution:  "Distribusi",
	models.FiberDrop:          "Drop",
	models.FiberODPToODP:      "ODP ke ODP",
	models.FiberODPToODPRatio: "ODP ke ODP (splitter)",
	models.FiberODCToODC:      "ODC ke ODC",
	models.FiberODCToODCRatio: "ODC ke ODC (splitter)",
}

type kmlWaypoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// edgeFolder is the Kabel folder: every cable whose two ends still resolve
// to a node on the map. It is left out entirely when nothing survives, the
// same rule nodeFolder applies per node kind. The second return is the
// edge_id of every cable that did not survive - the caller needs that list
// to tell the person downloading the file that something was left out,
// rather than producing a smaller file with no explanation.
func edgeFolder(edges []models.MappingEdge, nodesByID map[string]models.MappingNode) (*kmlFolder, []string) {
	var placemarks []kmlPlacemark
	var skipped []string
	for _, e := range edges {
		path, ok := edgePath(e, nodesByID)
		if !ok {
			skipped = append(skipped, e.EdgeID)
			continue
		}
		placemarks = append(placemarks, edgePlacemark(e, path))
	}
	if len(placemarks) == 0 {
		return nil, skipped
	}
	return &kmlFolder{Name: "Kabel", Placemarks: placemarks}, skipped
}

// edgePath is cableMath.ts's edgePath ported to Go: the full drawn path of
// one cable is its named ends plus the corners traced between them, never a
// straight line between endpoints - this system has already shipped that bug
// once. ok is false only when an endpoint no longer resolves to a node,
// which migrations/53_network_mapping.sql leaves as a normal, reachable
// state; that means "skip this one edge", never an error that aborts the
// rest of the export.
func edgePath(e models.MappingEdge, nodesByID map[string]models.MappingNode) ([]kmlWaypoint, bool) {
	source, ok := nodesByID[e.Source]
	if !ok {
		return nil, false
	}
	target, ok := nodesByID[e.Target]
	if !ok {
		return nil, false
	}
	corners, err := parseWaypoints(e.Waypoints)
	if err != nil {
		return nil, false
	}
	path := make([]kmlWaypoint, 0, len(corners)+2)
	path = append(path, kmlWaypoint{Lat: source.Latitude, Lng: source.Longitude})
	path = append(path, corners...)
	path = append(path, kmlWaypoint{Lat: target.Latitude, Lng: target.Longitude})
	return path, true
}

// parseWaypoints reads the same shape useCableDraw.ts writes: a JSON array
// of {lat,lng} corners, or nothing at all for a cable with none.
func parseWaypoints(raw []byte) ([]kmlWaypoint, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var points []kmlWaypoint
	if err := json.Unmarshal(raw, &points); err != nil {
		return nil, fmt.Errorf("parse waypoints: %w", err)
	}
	return points, nil
}

func edgePlacemark(e models.MappingEdge, path []kmlWaypoint) kmlPlacemark {
	coords := make([]string, len(path))
	for i, p := range path {
		coords[i] = coordinate(p.Lat, p.Lng)
	}
	return kmlPlacemark{
		Name:         e.EdgeID,
		Description:  edgeDescription(e),
		StyleURL:     "#cable-line",
		LineString:   &kmlLineString{Coordinates: strings.Join(coords, " ")},
		ExtendedData: edgeExtendedData(e),
	}
}

func edgeDescription(e models.MappingEdge) string {
	lines := []string{
		"Jenis: " + fiberTypeLabel[e.FiberType],
		"Panjang: " + formatMeters(e.Distance),
	}
	if e.Notes != "" {
		lines = append(lines, "Catatan: "+e.Notes)
	}
	return strings.Join(lines, "\n")
}

func edgeExtendedData(e models.MappingEdge) *kmlExtendedData {
	return &kmlExtendedData{Data: []kmlData{
		{Name: "edge_id", Value: e.EdgeID},
		{Name: "source", Value: e.Source},
		{Name: "target", Value: e.Target},
		{Name: "fiber_type", Value: string(e.FiberType)},
		{Name: "distance", Value: strconv.FormatFloat(e.Distance, 'f', -1, 64)},
	}}
}

// formatMeters mirrors cableMath.ts's formatMeters exactly (including the
// Indonesian comma decimal separator), so a cable's KML description reads
// the same length the popup already shows for it.
func formatMeters(m float64) string {
	if m >= 1000 {
		km := strconv.FormatFloat(m/1000, 'f', 2, 64)
		return strings.Replace(km, ".", ",", 1) + " km"
	}
	return fmt.Sprintf("%d m", int(math.Round(m)))
}
