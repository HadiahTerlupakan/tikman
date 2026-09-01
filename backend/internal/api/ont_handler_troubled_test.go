package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestTroubledQueryDefaultsToADayAndAPage(t *testing.T) {
	_, c := SetupTestContext("GET", "/api/v1/onts/troubled", nil)

	window, limit := troubledQuery(c)

	assert.Equal(t, 24*time.Hour, window)
	assert.Equal(t, 50, limit)
}

func TestTroubledQueryTakesWhatTheRequestAsksFor(t *testing.T) {
	_, c := SetupTestContext("GET", "/api/v1/onts/troubled?hours=168&limit=10", nil)

	window, limit := troubledQuery(c)

	assert.Equal(t, 168*time.Hour, window)
	assert.Equal(t, 10, limit)
}

func TestTroubledQueryClampsAWindowPastRetention(t *testing.T) {
	// Nothing older than the trap table's seven days exists to count, and the
	// aggregate reads every trap in the window, so a wider one costs more and
	// returns the same.
	_, c := SetupTestContext("GET", "/api/v1/onts/troubled?hours=8760&limit=99999", nil)

	window, limit := troubledQuery(c)

	assert.Equal(t, 7*24*time.Hour, window)
	assert.Equal(t, 200, limit)
}

func TestTroubledQueryFallsBackOnNonsense(t *testing.T) {
	_, c := SetupTestContext("GET", "/api/v1/onts/troubled?hours=abc&limit=-4", nil)

	window, limit := troubledQuery(c)

	assert.Equal(t, 24*time.Hour, window)
	assert.Equal(t, 50, limit)
}

func TestTroubledStatusRejectsAnUnknownValue(t *testing.T) {
	// Rejected rather than ignored: a filter that fails silently shows an empty
	// table, and the operator reads that as "nothing is wrong".
	_, err := parseTroubledStatus("rusak")
	require.Error(t, err)
}

func TestTroubledStatusAcceptsAKnownValue(t *testing.T) {
	status, err := parseTroubledStatus("los")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, models.ONTStatusLOS, *status)
}

func TestTroubledStatusIsOptional(t *testing.T) {
	status, err := parseTroubledStatus("")
	require.NoError(t, err)
	assert.Nil(t, status)
}
