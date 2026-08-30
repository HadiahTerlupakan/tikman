package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

func slotOf(n int) *int { return &n }

func readingWith(metrics map[connectivity.ONTLocation]connectivity.ONTMetrics, statuses map[connectivity.ONTLocation]int) *oltReading {
	r := &oltReading{metrics: metrics, statuses: statuses}
	r.index()
	return r
}

func rxAt(dbm float64) connectivity.ONTMetrics {
	return connectivity.ONTMetrics{RxPower: &dbm}
}

func TestMatchMetricsTakesTheReadingFromTheONTsOwnCard(t *testing.T) {
	// A chassis carries several cards, and port 12 ONU 22 exists on each of
	// them. Handing an ONT the reading from a neighbouring card would report one
	// subscriber's optical power against another's.
	reading := readingWith(map[connectivity.ONTLocation]connectivity.ONTMetrics{
		{Slot: 8, Port: 12, ONTID: 22}: rxAt(-18),
		{Slot: 9, Port: 12, ONTID: 22}: rxAt(-27),
	}, nil)

	ont := models.ONT{Slot: slotOf(9), PortID: 12, ONTID: 22}

	found, slot := matchMetricsForONT(ont, reading)

	require.NotNil(t, found)
	require.Equal(t, -27.0, *found.RxPower)
	require.Equal(t, 9, slot)
}

func TestMatchMetricsReturnsNothingWhenThatCardHasNoReading(t *testing.T) {
	// Falling back to another card's reading would be worse than no reading: the
	// ONT would look healthy on someone else's light.
	reading := readingWith(map[connectivity.ONTLocation]connectivity.ONTMetrics{
		{Slot: 8, Port: 12, ONTID: 22}: rxAt(-18),
	}, nil)

	found, slot := matchMetricsForONT(models.ONT{Slot: slotOf(9), PortID: 12, ONTID: 22}, reading)

	require.Nil(t, found)
	require.Zero(t, slot)
}

func TestMatchMetricsAdoptsTheCardWhenTheONTHasNone(t *testing.T) {
	// An ONT registered before its OLT reported card numbers has no slot to
	// match on. It takes the reading at its port and ONU number, and the card
	// that reading came from.
	reading := readingWith(map[connectivity.ONTLocation]connectivity.ONTMetrics{
		{Slot: 7, Port: 3, ONTID: 5}: rxAt(-21),
	}, nil)

	found, slot := matchMetricsForONT(models.ONT{PortID: 3, ONTID: 5}, reading)

	require.NotNil(t, found)
	require.Equal(t, -21.0, *found.RxPower)
	require.Equal(t, 7, slot, "the ONT did not adopt the card the reading came from")
}

func TestMatchMetricsOnAnEmptyReading(t *testing.T) {
	reading := readingWith(nil, nil)

	found, slot := matchMetricsForONT(models.ONT{Slot: slotOf(1), PortID: 1, ONTID: 1}, reading)

	require.Nil(t, found)
	require.Zero(t, slot)
}

func TestReportsPositionFindsAnONUOnAnyCard(t *testing.T) {
	// The presence check asks whether the OLT named this port and ONU at all.
	// It used to answer by scanning the whole table for every ONT, which costs
	// the square of the subscriber count.
	reading := readingWith(nil, map[connectivity.ONTLocation]int{
		{Slot: 9, Port: 6, ONTID: 21}: 3,
	})

	require.True(t, reading.reportsPosition(6, 21))
	require.False(t, reading.reportsPosition(6, 22))
	require.False(t, reading.reportsPosition(7, 21))
}

func TestReportsPositionOnAFailedWalkFindsNothing(t *testing.T) {
	// A failed status walk leaves no statuses. The caller must only act on that
	// when statusWalkOK says the walk actually happened — otherwise every
	// subscriber looks unreported at once.
	reading := readingWith(nil, nil)

	require.False(t, reading.statusWalkOK)
	require.False(t, reading.reportsPosition(1, 1))
}
