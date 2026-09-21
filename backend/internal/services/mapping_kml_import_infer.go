package services

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

// nearestNodeThresholdMeters bounds how far a cable's own endpoint may sit
// from a candidate node before the two are treated as connected. Fibre runs
// between physically adjacent boxes, and GPS/hand-placement error in a field
// survey is typically single digits of metres; 30m leaves headroom for that
// error without reaching into "probably a different box".
const nearestNodeThresholdMeters = 30.0

// classifyPlacemarks turns every placemark parsed out of a file into a node,
// a cable, or an issue - pure and DB-free like buildKML itself, so the
// inference rules are testable without a database. Edge inference runs only
// after every node has been classified, so a cable listed before the nodes
// it connects (file order is never guaranteed) can still be matched against
// all of them - see classifyEdge.
func classifyPlacemarks(raw []rawPlacemark) ([]ImportedNode, []ImportedEdge, []ImportIssue) {
	var nodes []ImportedNode
	var issues []ImportIssue
	var pendingEdges []struct {
		row  int
		rp   rawPlacemark
		path []kmlWaypoint
	}

	for i, rp := range raw {
		row := i + 1
		switch {
		case rp.Placemark.Point != nil:
			lat, lng, ok := parseCoordinate(rp.Placemark.Point.Coordinates)
			if !ok {
				issues = append(issues, ImportIssue{Row: row, Name: rp.Placemark.Name, Folder: rp.Folder, Reason: "Koordinat titik tidak valid"})
				continue
			}
			nodes = append(nodes, classifyNode(row, rp, lat, lng))
		case rp.Placemark.LineString != nil:
			path, ok := parseLineString(rp.Placemark.LineString.Coordinates)
			if !ok {
				issues = append(issues, ImportIssue{Row: row, Name: rp.Placemark.Name, Folder: rp.Folder, Reason: "Garis tidak valid (kurang dari dua titik atau koordinat rusak)"})
				continue
			}
			pendingEdges = append(pendingEdges, struct {
				row  int
				rp   rawPlacemark
				path []kmlWaypoint
			}{row, rp, path})
		default:
			issues = append(issues, ImportIssue{Row: row, Name: rp.Placemark.Name, Folder: rp.Folder, Reason: "Bentuk tidak didukung (bukan titik atau garis)"})
		}
	}

	edges := make([]ImportedEdge, 0, len(pendingEdges))
	for _, pe := range pendingEdges {
		edges = append(edges, classifyEdge(pe.row, pe.rp, pe.path, nodes))
	}
	return nodes, edges, issues
}

// classifyNode resolves one Point placemark. ExtendedData, when its "type"
// is one of the four this system knows, is trusted completely - that is the
// contract this export writes for exactly this reason (mapping_kml_node.go).
// Anything else is a guess from the folder and the placemark's own name.
func classifyNode(row int, rp rawPlacemark, lat, lng float64) ImportedNode {
	ext := extendedDataMap(rp.Placemark.ExtendedData)
	var n ImportedNode
	if t := models.NodeType(ext["type"]); isValidNodeType(t) {
		n = nodeFromExtendedData(rp.Placemark, ext, t)
	} else {
		n = nodeFromGuess(rp)
	}
	n = boundNodeFields(n)
	n.Row, n.Latitude, n.Longitude = row, lat, lng

	if n.Type == models.NodeServer {
		n.Blocked = true
		n.BlockedReason = "Node server mengikuti data OLT dan dibuat lewat menu OLT, bukan lewat impor"
	}
	n.Include = !n.Blocked && n.Type != ""
	return n
}

func nodeFromExtendedData(pm kmlPlacemark, ext map[string]string, nodeType models.NodeType) ImportedNode {
	capacity, _ := strconv.Atoi(ext["capacity"]) // malformed -> 0, same as "not counted yet" elsewhere in this system
	return ImportedNode{
		NodeID: firstNonEmpty(ext["node_id"], pm.Name), Type: nodeType, Name: pm.Name,
		Capacity: capacity, Splitter: ext["splitter"], PPPoE: ext["pppoe"], SerialNumber: ext["serial_number"],
		Notes:  ext["notes"],
		Reason: "Dari data ekspor (ExtendedData)",
	}
}

func nodeFromGuess(rp rawPlacemark) ImportedNode {
	pm := rp.Placemark
	n := ImportedNode{Name: pm.Name, NodeID: sanitizeID(pm.Name)}
	if t, why := guessNodeType(rp.Folder, pm.Name); t != "" {
		n.Type, n.Reason = t, why
	} else {
		n.Reason = "Tidak ada info jenis pada folder maupun nama titik; pilih manual"
	}
	return n
}

func guessNodeType(folder, name string) (models.NodeType, string) {
	if t, ok := matchNodeTypeKeyword(folder); ok {
		return t, fmt.Sprintf("Dari nama folder %q", folder)
	}
	if t, ok := matchNodeTypeKeyword(name); ok {
		return t, fmt.Sprintf("Dari nama titik %q", name)
	}
	return "", ""
}

// matchNodeTypeKeyword checks for this ISP's own naming convention (ODP-01,
// ODC-CENTRAL) first and the wider industry's ONU synonym second - a
// vendor's export is the one source where "ONU" rather than "ONT" is likely.
func matchNodeTypeKeyword(s string) (models.NodeType, bool) {
	upper := strings.ToUpper(s)
	switch {
	case strings.Contains(upper, "OLT") || strings.Contains(upper, "SERVER"):
		return models.NodeServer, true
	case strings.Contains(upper, "ODC"):
		return models.NodeODC, true
	case strings.Contains(upper, "ODP"):
		return models.NodeODP, true
	case strings.Contains(upper, "ONT") || strings.Contains(upper, "ONU"):
		return models.NodeONT, true
	}
	return "", false
}

func isValidNodeType(t models.NodeType) bool {
	switch t {
	case models.NodeServer, models.NodeODC, models.NodeODP, models.NodeONT:
		return true
	}
	return false
}

// classifyEdge resolves one LineString placemark. ExtendedData carrying a
// source is trusted completely, the same rule classifyNode applies to type.
// Otherwise this is the one shape a bare KML line truly cannot avoid
// guessing at scope note: matching only against nodes found in this same
// file, not the whole existing map - a field survey typically draws its
// boxes and its cables together in one file, and reaching into the whole
// production plant's coordinates is a larger, separate feature.
func classifyEdge(row int, rp rawPlacemark, path []kmlWaypoint, localNodes []ImportedNode) ImportedEdge {
	ext := extendedDataMap(rp.Placemark.ExtendedData)
	var e ImportedEdge
	// source and target are both NOT NULL columns (models.MappingEdge), so
	// any edge that ever passed through CreateEdge - and so could ever have
	// been exported - is guaranteed to carry both. Requiring both here is
	// what tells "this ExtendedData is our own export" apart from a vendor
	// file that happens to carry some unrelated ExtendedData of its own.
	if ext["source"] != "" && ext["target"] != "" {
		e = edgeFromExtendedData(rp.Placemark, ext, path, localNodes)
	} else {
		e = edgeFromGuess(rp.Placemark, path, localNodes)
	}
	e = boundEdgeFields(e)
	e.Row = row
	e.Unresolved = e.EdgeID == "" || e.Source == "" || e.Target == ""
	e.Include = !e.Unresolved
	return e
}

// edgeFromExtendedData trusts source and target completely - classifyEdge
// only reaches here once both are present - but distance and fiber_type are
// each allowed to be missing on their own, since nothing requires a foreign
// file's ExtendedData to carry this system's whole schema rather than part
// of it. Each falls through to what can still be derived rather than a
// silent zero or blank claimed as if it came from the file: a missing
// distance is computed from the line's own traced points (a computation,
// not a guess), and a missing fiber_type is guessed the same way
// edgeFromGuess would, now that source and target are known. The reason
// says exactly which parts were supplied and which were not, so this can
// never read as more confident than it is.
func edgeFromExtendedData(pm kmlPlacemark, ext map[string]string, path []kmlWaypoint, localNodes []ImportedNode) ImportedEdge {
	e := ImportedEdge{
		EdgeID: firstNonEmpty(ext["edge_id"], pm.Name),
		Source: ext["source"], Target: ext["target"],
		Waypoints: innerCorners(path),
		Notes:     ext["notes"],
		Reason:    "Sumber dan tujuan dari data ekspor (ExtendedData).",
	}
	if distance, err := strconv.ParseFloat(ext["distance"], 64); err == nil && isValidDistance(distance) {
		e.Distance = distance
	} else {
		e.Distance = pathLengthMeters(path)
		e.Reason += " Panjang dihitung dari garis."
	}
	if ext["fiber_type"] != "" {
		e.FiberType = models.FiberType(ext["fiber_type"])
	} else if ft, why := guessFiberType(e.Source, e.Target, localNodes); ft != "" {
		e.FiberType = ft
		e.Reason += " " + why
	}
	return e
}

func edgeFromGuess(pm kmlPlacemark, path []kmlWaypoint, localNodes []ImportedNode) ImportedEdge {
	e := ImportedEdge{EdgeID: sanitizeID(pm.Name), Waypoints: innerCorners(path)}

	source, sourceWhy := nearestNode(path[0], localNodes)
	target, targetWhy := nearestNode(path[len(path)-1], localNodes)
	e.Source, e.Target = source, target
	e.Reason = strings.TrimSpace(sourceWhy + " " + targetWhy)

	if e.EdgeID == "" && source != "" && target != "" {
		e.EdgeID = source + "--" + target
	}
	if ft, why := guessFiberType(source, target, localNodes); ft != "" {
		e.FiberType = ft
		e.Reason = strings.TrimSpace(e.Reason + " " + why)
	}
	return e
}

// innerCorners recovers exactly the corners useCableDraw.ts originally
// recorded, undoing edgePath's own source-then-corners-then-target
// concatenation (mapping_kml_edge.go) - the two endpoints are re-derived
// from Source/Target instead of being carried a second time.
func innerCorners(path []kmlWaypoint) []kmlWaypoint {
	if len(path) <= 2 {
		return nil
	}
	return path[1 : len(path)-1]
}

// nearestNode answers the closest candidate to one line endpoint, or "" with
// an explanation once nothing is within nearestNodeThresholdMeters - never a
// far-away guess presented as if it were confident.
func nearestNode(p kmlWaypoint, nodes []ImportedNode) (string, string) {
	var best *ImportedNode
	bestDist := math.Inf(1)
	for i := range nodes {
		d := haversineMeters(p.Lat, p.Lng, nodes[i].Latitude, nodes[i].Longitude)
		if d < bestDist {
			best, bestDist = &nodes[i], d
		}
	}
	if best == nil || bestDist > nearestNodeThresholdMeters {
		return "", "Ujung kabel tidak dekat node manapun pada berkas ini; isi manual."
	}
	return best.NodeID, fmt.Sprintf("Ujung dicocokkan ke %s (~%.0f m).", best.NodeID, bestDist)
}

// guessFiberType only fires once both ends are types with exactly one
// sensible default; the cascade kinds have two possible answers (plain or
// splitter ratio) and are left for a person to pick.
func guessFiberType(sourceID, targetID string, nodes []ImportedNode) (models.FiberType, string) {
	st, ok1 := nodeTypeByID(sourceID, nodes)
	tt, ok2 := nodeTypeByID(targetID, nodes)
	if !ok1 || !ok2 {
		return "", ""
	}
	switch {
	case st == models.NodeServer && tt == models.NodeODC:
		return models.FiberFeeder, "Jenis feeder diasumsikan dari OLT ke ODC."
	case st == models.NodeODC && tt == models.NodeODP:
		return models.FiberDistribution, "Jenis distribusi diasumsikan dari ODC ke ODP."
	case st == models.NodeODP && tt == models.NodeONT:
		return models.FiberDrop, "Jenis drop diasumsikan dari ODP ke ONT."
	case st == models.NodeODC && tt == models.NodeODC:
		return models.FiberODCToODC, "Jenis ODC ke ODC diasumsikan tanpa rasio splitter; ubah bila perlu."
	case st == models.NodeODP && tt == models.NodeODP:
		return models.FiberODPToODP, "Jenis ODP ke ODP diasumsikan tanpa rasio splitter; ubah bila perlu."
	}
	return "", ""
}

func nodeTypeByID(id string, nodes []ImportedNode) (models.NodeType, bool) {
	for _, n := range nodes {
		if n.NodeID == id {
			return n.Type, true
		}
	}
	return "", false
}
