package services

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestRouteMetersOverAKnownDistance(t *testing.T) {
	// One degree of latitude is about 111.19 km anywhere on the globe. Anything
	// wildly off this means the formula, not the rounding, is wrong.
	meters := RouteMeters([]models.RoutePoint{{Lat: 0, Lng: 0}, {Lat: 1, Lng: 0}})

	assert.InDelta(t, 111195, meters, 500)
}

func TestRouteMetersOverASmallSpanNearCariu(t *testing.T) {
	// Two points about 1.1 km apart in West Java, the scale a drop cable
	// actually runs at.
	meters := RouteMeters([]models.RoutePoint{
		{Lat: -6.4000, Lng: 107.0000},
		{Lat: -6.4100, Lng: 107.0000},
	})

	assert.InDelta(t, 1112, meters, 20)
}

func TestRouteMetersAddsEveryLeg(t *testing.T) {
	straight := RouteMeters([]models.RoutePoint{
		{Lat: 0, Lng: 0}, {Lat: 0.02, Lng: 0},
	})
	bent := RouteMeters([]models.RoutePoint{
		{Lat: 0, Lng: 0}, {Lat: 0.01, Lng: 0.01}, {Lat: 0.02, Lng: 0},
	})

	// A cable that goes round a corner is longer than one that does not, which
	// is the entire reason for tracing a route rather than measuring the gap.
	assert.Greater(t, bent, straight)
}

func TestRouteMetersOfNothing(t *testing.T) {
	// A route not drawn yet has no length. Zero, not an error and not a guess.
	assert.Equal(t, 0.0, RouteMeters(nil))
	assert.Equal(t, 0.0, RouteMeters([]models.RoutePoint{}))
	assert.Equal(t, 0.0, RouteMeters([]models.RoutePoint{{Lat: -6.4, Lng: 107}}))
}

func TestRouteMetersIsSymmetric(t *testing.T) {
	forward := RouteMeters([]models.RoutePoint{{Lat: -6.4, Lng: 107}, {Lat: -6.41, Lng: 107.01}})
	backward := RouteMeters([]models.RoutePoint{{Lat: -6.41, Lng: 107.01}, {Lat: -6.4, Lng: 107}})

	assert.Less(t, math.Abs(forward-backward), 0.001)
}
