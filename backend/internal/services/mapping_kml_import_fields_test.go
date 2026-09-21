package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
