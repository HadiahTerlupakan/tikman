package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The raw table is dropped after 7 days, so a 30-day graph that read it could
// only ever draw a week: both the 7d and 30d buttons returned the identical
// 221 points. Each window has to come from a store that still holds it.
func TestTrafficTierFor(t *testing.T) {
	for _, tc := range []struct {
		name   string
		window time.Duration
		table  string
		step   time.Duration
	}{
		{"three hours", 3 * time.Hour, rawTrafficTable, 5 * time.Minute},
		{"one day", 24 * time.Hour, rawTrafficTable, 5 * time.Minute},
		{"three days", 3 * 24 * time.Hour, fiveMinuteRollup, 30 * time.Minute},
		{"seven days", 7 * 24 * time.Hour, fiveMinuteRollup, 30 * time.Minute},
		{"thirty days", 30 * 24 * time.Hour, hourlyRollupTable, 6 * time.Hour},
		{"a year", 365 * 24 * time.Hour, hourlyRollupTable, 6 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tier := trafficTierFor(tc.window)

			assert.Equal(t, tc.table, tier.table)
			assert.Equal(t, tc.step, tier.step)
		})
	}
}

// A 30-day window at the raw table's 5-minute resolution is 8640 points, which
// no chart can draw apart. Every tier has to stay in the low hundreds.
func TestTrafficTierKeepsPointCountPlottable(t *testing.T) {
	for _, window := range []time.Duration{
		3 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour,
	} {
		tier := trafficTierFor(window)
		points := int(window / tier.step)

		assert.LessOrEqual(t, points, 400, "window %v yields %d points", window, points)
	}
}

// A rollup records the peak inside its own bucket. Recomputing one from the
// averages would flatten a spike shorter than that bucket.
func TestTrafficPeakPrefersTheRecordedPeak(t *testing.T) {
	recorded := 42.0

	assert.Equal(t, 42.0, trafficPeak(&recorded, 3.0))
	assert.Equal(t, 3.0, trafficPeak(nil, 3.0))
	// A peak below the average is not a peak; the average stands.
	lower := 1.0
	assert.Equal(t, 3.0, trafficPeak(&lower, 3.0))
}
