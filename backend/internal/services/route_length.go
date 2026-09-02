package services

import (
	"math"

	"github.com/tikman/olt-provisioning/internal/models"
)

// earthRadiusMeters is the mean radius. A cable run is a few hundred metres, so
// the difference between the mean radius and a local one is far below the error
// in where someone clicked the pole.
const earthRadiusMeters = 6371008.8

// RouteMeters is how long a cable is, following the path traced for it.
//
// The sum of the legs, not the distance between the ends: a cable that goes
// round a corner is longer than the gap it spans, and that difference is the
// whole reason for tracing a route rather than measuring a straight line.
//
// A route with fewer than two points has no length — nobody has drawn it yet.
func RouteMeters(points []models.RoutePoint) float64 {
	total := 0.0
	for i := 1; i < len(points); i++ {
		total += legMeters(points[i-1], points[i])
	}
	return total
}

// legMeters is the great-circle distance between two vertices, by the haversine
// formula, which stays accurate at the short spans plant is built at.
func legMeters(from, to models.RoutePoint) float64 {
	lat1 := from.Lat * math.Pi / 180
	lat2 := to.Lat * math.Pi / 180
	deltaLat := (to.Lat - from.Lat) * math.Pi / 180
	deltaLng := (to.Lng - from.Lng) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	return 2 * earthRadiusMeters * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
