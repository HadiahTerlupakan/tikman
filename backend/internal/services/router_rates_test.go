package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInterfaceRate(t *testing.T) {
	start := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

	bps, ok := InterfaceRate(1_000, 750_001_000, start, start.Add(time.Minute))
	assert.True(t, ok)
	assert.InDelta(t, 100_000_000, bps, 0.001)

	// A counter that went backwards means the router rebooted or the counter
	// was reset. The difference is meaningless, and drawing it would put a
	// spike of exabits on the graph.
	_, ok = InterfaceRate(750_001_000, 5_000, start, start.Add(time.Minute))
	assert.False(t, ok)

	_, ok = InterfaceRate(1_000, 2_000, start, start)
	assert.False(t, ok, "no elapsed time gives no rate")

	// Past 2^63 the counter is still read correctly.
	high := uint64(1) << 63
	bps, ok = InterfaceRate(high, high+125, start, start.Add(time.Second))
	assert.True(t, ok)
	assert.InDelta(t, 1000, bps, 0.001)
}
