package services

import (
	"bufio"
	"math"
	"strconv"
	"strings"
)

// splitterMaxLen mirrors mapping_nodes.splitter's varchar(16).
const splitterMaxLen = 16

// maxLineStringPoints bounds one cable's corners. A real one comes from a
// technician tapping a map and is a handful to a few dozen points at most;
// this leaves generous headroom while keeping a single mapping_edges.waypoints
// value - loaded by ListEdges for every signed-in user on every map page -
// from being sized by whatever a file claims a cable's route was.
//
// Known residual, not fixed: this bounds one cable, not a file's total
// waypoints across every cable - measured, 145 LineStrings each legitimately
// at exactly this cap turned a 47,960 B KMZ into an 18,164,837 B preview
// response (379x) at 122.5 MiB peak live heap per concurrent request.
// Adversarial-only (needs the editor role) and preview-only (nothing is
// written), so left as a known gap rather than a fix this round.
const maxLineStringPoints = 5000

// haversineMeters mirrors cableMath.ts's metersBetween (Go cannot import
// that file) - the same formula and Earth radius, so a guessed distance
// reads the same as the map's own.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6_371_000.0
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }
	dLat, dLng := toRad(lat2-lat1), toRad(lng2-lng1)
	h := math.Pow(math.Sin(dLat/2), 2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Pow(math.Sin(dLng/2), 2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

// pathLengthMeters mirrors cableMath.ts's metersAlong: the sum of each
// consecutive segment's haversine distance. Used when ExtendedData is
// missing its own distance - the file's own traced line already says
// exactly how long the cable is, which is a computation, not a guess.
func pathLengthMeters(path []kmlWaypoint) float64 {
	var total float64
	for i := 1; i < len(path); i++ {
		total += haversineMeters(path[i-1].Lat, path[i-1].Lng, path[i].Lat, path[i].Lng)
	}
	return total
}

func extendedDataMap(ext *kmlExtendedData) map[string]string {
	m := make(map[string]string)
	if ext == nil {
		return m
	}
	for _, d := range ext.Data {
		m[d.Name] = d.Value
	}
	return m
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// sanitizeID bounds a guessed id to mapping_nodes.node_id/mapping_edges.edge_id's
// shared varchar(64), the same ceiling serverNodeIDForOLT already truncates
// to (olt_map_node.go) - reused here rather than redeclared.
func sanitizeID(s string) string {
	return truncateToRunes(strings.TrimSpace(s), serverNodeIDMaxLen)
}

// boundNodeFields truncates every string field to what mapping_nodes' own
// varchar columns allow. Applied once, in classifyNode, after either
// inference path has resolved a node, rather than duplicated inside
// nodeFromExtendedData and nodeFromGuess separately: an ExtendedData value
// out of a hand-typed or vendor file is exactly as unbounded as a
// placemark's raw <name>, and a length that only fails at commit time -
// after the preview already showed it with no problem - is a Postgres
// 22001 aborting the whole transaction into an opaque 500.
func boundNodeFields(n ImportedNode) ImportedNode {
	n.NodeID = sanitizeID(n.NodeID)
	n.Name = truncateToRunes(n.Name, mapNodeNameMaxLen)
	n.Splitter = truncateToRunes(n.Splitter, splitterMaxLen)
	n.PPPoE = truncateToRunes(n.PPPoE, serverNodeIDMaxLen)
	n.SerialNumber = truncateToRunes(n.SerialNumber, serverNodeIDMaxLen)
	return n
}

// boundEdgeFields is boundNodeFields' counterpart for mapping_edges: edge_id,
// source and target share the same varchar(64) as a mapping_nodes id.
// Applied after edgeFromGuess too, since its own source--target fallback
// can synthesise an id longer than either half alone.
func boundEdgeFields(e ImportedEdge) ImportedEdge {
	e.EdgeID = sanitizeID(e.EdgeID)
	e.Source = sanitizeID(e.Source)
	e.Target = sanitizeID(e.Target)
	return e
}

// isValidDistance rejects what strconv.ParseFloat accepts but a physical
// cable length cannot be: NaN and Inf both parse with no error - the same
// trap parseCoordinate is guarded against below - and a NaN/Inf Distance
// reaching an ImportedEdge field fails encoding/json's own Marshal after
// the response status is already written, a 200 with no usable body.
// Negative is rejected too: nothing on this map has a cable of negative
// length.
func isValidDistance(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0
}

// parseCoordinate reverses coordinate() in mapping_kml.go: KML's own
// "longitude,latitude,altitude" order, altitude ignored since nothing on
// this map ever has one. Validated through the same validateCoordinates
// every other coordinate write in this system uses (site_service.go):
// strconv.ParseFloat happily parses "NaN"/"Inf" as real floats with no
// error, and a swapped lat/lng pair - the commonest defect in a hand-made
// or third-party file, which is exactly what this feature exists to
// import - parses as two perfectly ordinary-looking numbers that are
// simply impossible as a latitude. Either would otherwise reach an
// ImportedNode/ImportedEdge field unnoticed.
func parseCoordinate(s string) (lat, lng float64, ok bool) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) < 2 {
		return 0, 0, false
	}
	lngVal, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	latVal, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if validateCoordinates(&latVal, &lngVal) != nil {
		return 0, 0, false
	}
	return latVal, lngVal, true
}

// parseLineString reads a LineString's space-separated coordinate tuples,
// refusing anything with fewer than the two a line must have to mean
// anything, or more than maxLineStringPoints.
func parseLineString(s string) ([]kmlWaypoint, bool) {
	if !lineStringFieldCountInRange(s) {
		return nil, false
	}
	fields := strings.Fields(s)
	points := make([]kmlWaypoint, 0, len(fields))
	for _, f := range fields {
		lat, lng, ok := parseCoordinate(f)
		if !ok {
			return nil, false
		}
		points = append(points, kmlWaypoint{Lat: lat, Lng: lng})
	}
	return points, true
}

// lineStringFieldCountInRange counts a LineString's space-separated
// coordinate tuples via bufio.ScanWords rather than strings.Fields, so
// maxLineStringPoints is a memory bound and not only a correctness one:
// strings.Fields must materialise a []string slot for every token it finds
// before its own length could ever be compared against the cap, and a
// single-character token repeated many times fits millions of tokens in a
// modest byte budget (measured: 5,000,000 tokens cost ~80 MB for the slice
// alone before this fix, purely from allocating one 16-byte header per
// token). ScanWords advances its own small internal buffer and is never
// asked for a token's text, so counting costs none of that - and the loop
// exits the instant the count is exceeded, so even the counting itself
// never runs past the cap.
//
// bufio.Scanner also stops - Scan returning false - when one token exceeds
// its internal buffer (bufio.MaxScanTokenSize, 64 KiB), which looks
// identical to a clean end of input unless Err() is checked afterwards.
// parseCoordinate only reads a token's first two comma-separated parts, so
// a coordinate tuple with an oversized third ("altitude") component is
// still a syntactically valid coordinate as far as this file's own parsing
// is concerned - just one word that trips the scanner's buffer. Without the
// Err() check, that truncated the count at whatever had been seen so far
// and reported it as final, so a LineString with a handful of ordinary
// points, one oversized token, and thousands more ordinary points after it
// passed as if it had only the first few - then parseLineString's own
// strings.Fields (no per-token size limit) went on to parse every one of
// those thousands with no cap left to catch them.
func lineStringFieldCountInRange(s string) bool {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanWords)
	count := 0
	for scanner.Scan() {
		count++
		if count > maxLineStringPoints {
			return false
		}
	}
	if scanner.Err() != nil {
		return false
	}
	return count >= 2
}
