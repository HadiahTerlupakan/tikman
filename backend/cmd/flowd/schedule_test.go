package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Each tick is computed from the wall clock, so a clock that slews cannot
// drift the closes off the minute boundary the rows are stamped with.
func TestNextMinuteTickLandsJustAfterTheNextBoundary(t *testing.T) {
	cases := map[string]struct{ now, want time.Time }{
		"mid minute":            {minute0.Add(25 * time.Second), minute0.Add(time.Minute + minuteTickOffset)},
		"just before boundary":  {minute0.Add(time.Minute - time.Millisecond), minute0.Add(time.Minute + minuteTickOffset)},
		"on the tick just made": {minute0.Add(minuteTickOffset), minute0.Add(time.Minute + minuteTickOffset)},
		"woken a bit late":      {minute0.Add(3 * time.Second), minute0.Add(time.Minute + minuteTickOffset)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, nextMinuteTick(tc.now))
		})
	}
}
