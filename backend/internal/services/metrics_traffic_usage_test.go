package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func counters(firstRx, lastRx, firstTx, lastTx uint64) ONTMetricsRow {
	return ONTMetricsRow{
		FirstRxBytes: &firstRx, LastRxBytes: &lastRx,
		FirstTxBytes: &firstTx, LastTxBytes: &lastTx,
	}
}

// Usage is what the counter moved across the whole window, including the gap
// between one bucket's last reading and the next bucket's first.
func TestTrafficUsageOverSpansBuckets(t *testing.T) {
	usage := trafficUsageOver([]ONTMetricsRow{
		counters(100, 200, 1000, 1500),
		counters(250, 400, 1800, 2000),
	}, time.Hour)

	// 100 within the first bucket, 50 across the gap, 150 within the second.
	assert.Equal(t, uint64(300), usage.UploadBytes)
	assert.Equal(t, uint64(1000), usage.DownloadBytes)
}

// An ONU that restarts resets its counters. Subtracting across that would
// report a negative figure, or an absurd one once it wraps to unsigned.
func TestTrafficUsageOverIgnoresACounterReset(t *testing.T) {
	usage := trafficUsageOver([]ONTMetricsRow{
		counters(1000, 5000, 1000, 5000),
		counters(0, 300, 0, 300), // rebooted
	}, time.Hour)

	// The 4000 before the reset and the 300 after it, with no bogus gap term.
	assert.Equal(t, uint64(4300), usage.UploadBytes)
	assert.Equal(t, uint64(4300), usage.DownloadBytes)
}

// A window with no counter readings reports nothing rather than zero traffic.
func TestTrafficUsageOverWithoutCounters(t *testing.T) {
	usage := trafficUsageOver([]ONTMetricsRow{{}, {}}, time.Hour)

	assert.Equal(t, TrafficUsage{}, usage)
}

// The day this system started reading the right counter OID, the stored value
// jumped from a few kilobytes to hundreds of gigabytes in one step. That is a
// discontinuity, not a subscriber's traffic, and billing it would be absurd.
func TestTrafficUsageOverIgnoresAnImpossibleJump(t *testing.T) {
	usage := trafficUsageOver([]ONTMetricsRow{
		counters(6775, 6800, 6775, 6800),
		counters(405_000_000_000, 405_000_100_000, 405_000_000_000, 405_000_100_000),
	}, 5*time.Minute)

	// The 25 bytes before the jump and the 100000 after it, but not the 405 GB
	// step between them, which no GPON link could carry in five minutes.
	assert.Equal(t, uint64(100_025), usage.UploadBytes)
	assert.Equal(t, uint64(100_025), usage.DownloadBytes)
}

// The ceiling has to admit a link running flat out, or a busy subscriber's real
// traffic would be discarded as a glitch.
func TestTrafficUsageOverKeepsAFullyLoadedLink(t *testing.T) {
	// One minute at roughly 1 Gbps.
	const oneMinuteAtGigabit = 7_500_000_000

	usage := trafficUsageOver([]ONTMetricsRow{
		counters(0, oneMinuteAtGigabit, 0, oneMinuteAtGigabit),
	}, time.Minute)

	assert.Equal(t, uint64(oneMinuteAtGigabit), usage.DownloadBytes)
}
