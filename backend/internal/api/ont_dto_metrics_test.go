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
		Distance:     1004,
		RxPower:      &rx,
		TxPower:      &tx,
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
