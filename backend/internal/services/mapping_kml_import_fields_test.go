package services

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strconv.ParseFloat parses "NaN" successfully - a hand-written or vendor
// KML carrying it is not a parse failure, only a bad value, and a NaN
// latitude must not silently become an ImportedNode field: it fails
// encoding/json's own Marshal after the response status is already
// written, so the request answers 200 with no usable body at all.
func TestParseCoordinateRejectsNaN(t *testing.T) {
	_, _, ok := parseCoordinate("106.8,NaN,0")
	assert.False(t, ok)
}

func TestParseCoordinateRejectsInfinity(t *testing.T) {
	_, _, ok := parseCoordinate("Inf,-6.2,0")
	assert.False(t, ok)
}

// The commonest defect in a hand-made or third-party file: lng,lat written
// where KML wants lat,lng lands a real-looking but impossible latitude.
func TestParseCoordinateRejectsOutOfRangeLatitude(t *testing.T) {
	_, _, ok := parseCoordinate("106.8,200,0")
	assert.False(t, ok)
}

// A cable's corners come from a technician tapping a map - a real route
// never has more than a few dozen. Nothing bounded how many a LineString
// could carry: at maxKMLPlacemarks, one cable whose coordinate list runs to
// the KML byte cap marshals into a single mapping_edges.waypoints value of
// a comparable size, which then loads on every map page for every
// signed-in user - ListEdges has no per-row limit.
func TestParseLineStringRefusesTooManyPoints(t *testing.T) {
	var b strings.Builder
	for i := 0; i <= maxLineStringPoints; i++ {
		b.WriteString("0,0,0 ")
	}

	_, ok := parseLineString(b.String())

	assert.False(t, ok)
}

func TestParseLineStringAcceptsExactlyTheCap(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxLineStringPoints; i++ {
		b.WriteString("0,0,0 ")
	}

	points, ok := parseLineString(b.String())

	assert.True(t, ok)
	assert.Len(t, points, maxLineStringPoints)
}

// The cap on maxLineStringPoints is correct but was checked against
// strings.Fields' own output - and strings.Fields must materialise a
// []string slot for every token it finds *before* its own length could
// ever be compared against the cap. A single-character token repeated many
// times fits millions of tokens in a modest byte budget: measured, 5
// million tokens (10 bytes here, scaling to 216 KiB at the size that
// produced 8 million in the field) cost the old ordering roughly 80 MB for
// the slice alone (16 bytes/header x 5,000,000), on top of the input
// itself. Counting first, without asking for any token's text, must cost a
// small, bounded fraction of that - not scale with the token count at all.
// bufio.Scanner (which lineStringFieldCountInRange uses) has a default max
// token size of 64 KiB (bufio.MaxScanTokenSize) - a whitespace-delimited
// word longer than that makes Scan return false with ErrTooLong, which
// looks identical to a clean end of input if the caller never checks Err().
// parseCoordinate only reads a token's first two comma-separated parts, so
// a coordinate tuple with an oversized third ("altitude") component is
// still a syntactically valid coordinate - just one word that trips the
// scanner's own buffer. The count function stopped early and reported
// whatever count it had reached as final, so a file with two ordinary
// points, one oversized token, and thousands more ordinary points after it
// was accepted as if it had only two points - then strings.Fields (no
// per-token size limit) went on to parse every one of those thousands into
// the result, with no cap left to catch them.
func TestParseLineStringRefusesWhenAnOversizedTokenEndsTheWordScanEarly(t *testing.T) {
	oversized := "106.8,-6.2," + strings.Repeat("0", 70_000)
	var b strings.Builder
	b.WriteString("106.8,-6.2,0 106.8,-6.2,0 ")
	b.WriteString(oversized)
	b.WriteString(" ")
	for i := 0; i < maxLineStringPoints+100; i++ {
		b.WriteString("106.8,-6.2,0 ")
	}

	_, ok := parseLineString(b.String())

	assert.False(t, ok, "an oversized token must not let the rest of the LineString bypass the point cap")
}

func TestParseLineStringCountsBeforeMaterialisingFields(t *testing.T) {
	const tokenCount = 5_000_000
	var b strings.Builder
	b.Grow(tokenCount * 2)
	for i := 0; i < tokenCount; i++ {
		b.WriteString("0 ")
	}
	input := b.String()

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	_, ok := parseLineString(input)

	runtime.ReadMemStats(&after)
	require.False(t, ok, "5,000,000 tokens must still be refused - this is a memory-ordering test, not a cap-value test")
	grew := after.TotalAlloc - before.TotalAlloc
	const tenMiB = 10 << 20
	assert.Less(t, grew, uint64(tenMiB),
		"parseLineString allocated %d bytes rejecting %d tokens - strings.Fields likely ran before the count was checked", grew, tokenCount)
}
