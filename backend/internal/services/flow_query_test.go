package services

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestTopIPsRefusesARangeOlderThanItsRetention(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	week := 7 * 24 * time.Hour

	assert.ErrorIs(t, checkTopIPsRange(now.Add(-8*24*time.Hour), now), ErrTopIPsRange)
	assert.ErrorIs(t, checkTopIPsRange(now.Add(-8*24*time.Hour), now), ErrValidation)
	assert.NoError(t, checkTopIPsRange(now.Add(-week-2*time.Second), now),
		"the 7-day preset's start was computed a moment before the request")

	_, err := NewFlowQueryService(setupTestDB(t)).TopIPs(uuid.New(),
		FlowFilter{From: now.Add(-30 * 24 * time.Hour), To: now}, now)
	assert.ErrorIs(t, err, ErrTopIPsRange)
}

func TestStatusReportsTheNewestDataset(t *testing.T) {
	db := setupTestDB(t)
	flows := NewFlowQueryService(db)

	empty, err := flows.Status()
	require.NoError(t, err)
	assert.Nil(t, empty.ASNDatasetUpdatedAt)
	assert.Zero(t, empty.ASNCount)

	older := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: older}).Error)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64497, Name: "EXAMPLE-NET-B", Country: "ZZ", UpdatedAt: newer}).Error)

	status, err := flows.Status()
	require.NoError(t, err)
	require.NotNil(t, status.ASNDatasetUpdatedAt)
	assert.True(t, newer.Equal(*status.ASNDatasetUpdatedAt))
	assert.EqualValues(t, 2, status.ASNCount)
}

// Two upstreams' totals, each held at bigint's maximum by the query, would
// wrap negative as a plain int64 sum.
func TestAddBytesHoldsAtTheInt64Maximum(t *testing.T) {
	assert.EqualValues(t, 3, addBytes(1, 2))
	assert.EqualValues(t, int64(math.MaxInt64), addBytes(math.MaxInt64, math.MaxInt64))
	assert.EqualValues(t, int64(math.MaxInt64), addBytes(math.MaxInt64-1, 2))
	assert.EqualValues(t, int64(math.MaxInt64)-1, addBytes(math.MaxInt64, -1),
		"a negative row stored before counts were capped still subtracts")
}
