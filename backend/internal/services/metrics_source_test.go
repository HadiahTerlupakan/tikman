package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSourceForRangeReadsRawWithinItsRetention(t *testing.T) {
	now := time.Now()

	// Raw carries every ten-minute sample and, unlike the aggregates, the
	// minutes since the last rollup refresh.
	assert.Equal(t, sourceRaw, sourceForRange(now.Add(-2*time.Hour), now))
	assert.Equal(t, sourceRaw, sourceForRange(now.Add(-5*24*time.Hour), now))
}

func TestSourceForRangeStopsShortOfRawRetention(t *testing.T) {
	now := time.Now()

	// The policy drops raw after seven days, but it drops whole chunks at a
	// moment nothing here can predict, so the sixth day is already treated as
	// gone. A rollup that certainly holds the range beats raw that might not:
	// the cost of being wrong is an empty chart.
	assert.Equal(t, source5Min, sourceForRange(now.Add(-6*24*time.Hour), now))
}

func TestSourceForRangeFallsToTheRollupsPastRawRetention(t *testing.T) {
	now := time.Now()

	// Raw is dropped after seven days, so a range reaching further back has to
	// come from a rollup or it comes back empty.
	assert.Equal(t, source5Min, sourceForRange(now.Add(-10*24*time.Hour), now))
	assert.Equal(t, sourceHourly, sourceForRange(now.Add(-60*24*time.Hour), now))
}

func TestSourceForRangeTakesTheFinestSourceThatStillHoldsTheRange(t *testing.T) {
	now := time.Now()

	// Five-minute buckets last thirty days and hourly ones a year, so a
	// three-week range must not be coarsened to hourly.
	assert.Equal(t, source5Min, sourceForRange(now.Add(-21*24*time.Hour), now))
	assert.Equal(t, sourceHourly, sourceForRange(now.Add(-200*24*time.Hour), now))
}

func TestSourceForRangeHandlesARangeThatIsNotInThePast(t *testing.T) {
	now := time.Now()

	assert.Equal(t, sourceRaw, sourceForRange(now, now))
	assert.Equal(t, sourceRaw, sourceForRange(now.Add(time.Hour), now))
}
