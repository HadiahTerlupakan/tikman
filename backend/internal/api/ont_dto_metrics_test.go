package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func storedONT() *models.ONT {
	rx, tx := -25.85, 2.50
	return &models.ONT{
		ID:           uuid.New(),
		SerialNumber: "RTEGC609E381",
		// A real ONT always has one, and the optical figures are only served
		// while it is online. Left unset, these two overlay tests would be
		// asserting against a state no row can be in.
		Status:   models.ONTStatusOnline,
		Distance: 1004,
		RxPower:  &rx,
		TxPower:  &tx,
	}
}

// The OLT drops varbinds under load, so a poll can store a row with no
// distance and no optical power. Overlaying it blanked values that had been
// read a minute earlier, and the list appeared to lose data at random.
func TestToONTResponseWithMetricsKeepsStoredValuesWhenAReadingIsMissing(t *testing.T) {
	resp := ToONTResponseWithMetrics(storedONT(), &services.ONTMetricsRow{
		Distance: 0,
		RxPower:  nil,
		TxPower:  nil,
	})

	require.NotNil(t, resp.Distance)
	assert.Equal(t, 1004, *resp.Distance)
	require.NotNil(t, resp.RxPower)
	assert.InDelta(t, -25.85, *resp.RxPower, 0.001)
	require.NotNil(t, resp.TxPower)
	assert.InDelta(t, 2.50, *resp.TxPower, 0.001)
}

// A reading that did arrive is newer than the stored one and must win.
func TestToONTResponseWithMetricsPrefersAFreshReading(t *testing.T) {
	rx, tx := -30.10, 1.80
	resp := ToONTResponseWithMetrics(storedONT(), &services.ONTMetricsRow{
		Distance: 2107,
		RxPower:  &rx,
		TxPower:  &tx,
	})

	require.NotNil(t, resp.Distance)
	assert.Equal(t, 2107, *resp.Distance)
	assert.InDelta(t, -30.10, *resp.RxPower, 0.001)
	assert.InDelta(t, 1.80, *resp.TxPower, 0.001)
}

// An ONT that is not online has no optical reading to report: the figure on
// the row is whatever it last measured while it was alive, and serving it
// beside a status saying the ONT is down reads as a current measurement.
// The status poll runs every minute and the metrics poll every ten, so
// without this the row tells that lie for up to nine of them.
func TestToONTResponseHidesOpticsWhileTheONTIsNotOnline(t *testing.T) {
	for _, status := range []models.ONTStatus{
		models.ONTStatusOffline,
		models.ONTStatusLOS,
		models.ONTStatusDyingGas,
	} {
		ont := storedONT()
		ont.Status = status

		resp := ToONTResponse(ont)

		assert.Nil(t, resp.RxPower, string(status))
		assert.Nil(t, resp.TxPower, string(status))
	}
}

func TestToONTResponseServesOpticsWhileTheONTIsOnline(t *testing.T) {
	ont := storedONT()
	ont.Status = models.ONTStatusOnline

	resp := ToONTResponse(ont)

	require.NotNil(t, resp.RxPower)
	assert.InDelta(t, -25.85, *resp.RxPower, 0.001)
	require.NotNil(t, resp.TxPower)
	assert.InDelta(t, 2.50, *resp.TxPower, 0.001)
}

// The overlay is the path that can put a reading back after the row was
// cleared: a live poll can return optical power for an ONT the OLT still
// answers for while calling it offline, which is exactly what production
// showed. The rule has to hold at the end, not only at the start.
func TestAFreshReadingDoesNotResurrectOpticsForANotOnlineONT(t *testing.T) {
	rx, tx := -30.10, 1.80
	ont := storedONT()
	ont.Status = models.ONTStatusOffline

	resp := ToONTResponseWithMetrics(ont, &services.ONTMetricsRow{RxPower: &rx, TxPower: &tx})

	assert.Nil(t, resp.RxPower)
	assert.Nil(t, resp.TxPower)
}
