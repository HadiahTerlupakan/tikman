package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

// stubStatusQuerier answers with one phase state per call, so a test can say
// what the OLT reports on the first read and on the ones after it.
type stubStatusQuerier struct {
	replies []int
	calls   int
}

func (s *stubStatusQuerier) QueryStatusFor(_, _ string, _ int, locations []connectivity.ONTLocation) (map[connectivity.ONTLocation]int, error) {
	reply := connectivity.PhaseStateUnknown
	if s.calls < len(s.replies) {
		reply = s.replies[s.calls]
	}
	s.calls++

	statuses := map[connectivity.ONTLocation]int{}
	if reply != connectivity.PhaseStateUnknown {
		statuses[locations[0]] = reply
	}
	return statuses, nil
}

func provisionedONT(t *testing.T) (models.ONT, models.OLT) {
	t.Helper()
	original := onuSettleInterval
	onuSettleInterval = time.Millisecond
	t.Cleanup(func() { onuSettleInterval = original })

	slot := 3
	return models.ONT{
			ID: uuid.New(), OLTID: uuid.New(), Slot: &slot, PortID: 1, ONTID: 19,
			SerialNumber: "HWTCB403E8A0", Status: models.ONTStatusUnknown,
		}, models.OLT{
			Model: models.OLTModelZTEC300, IPAddress: "10.0.0.1",
			SNMPCommunity: "public", SNMPPort: 161,
		}
}

// A registered ONU has to range before the OLT reports a state, so the first
// read comes back empty. Giving up there left the row at "unknown" until the
// discovery poll came round, which is what an operator was waiting on.
func TestStoreSettledONUStatusWaitsForTheONUToRange(t *testing.T) {
	db := setupTestDB(t)
	ont, olt := provisionedONT(t)
	require.NoError(t, db.Create(&ont).Error)

	querier := &stubStatusQuerier{replies: []int{connectivity.PhaseStateUnknown, connectivity.PhaseStateOnline}}
	resolved, err := storeSettledONUStatus(db, querier, olt, ont, time.Second)
	require.NoError(t, err)
	assert.True(t, resolved)

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.Equal(t, models.ONTStatusOnline, stored.Status)
	assert.Equal(t, 2, querier.calls)
	// Online is the one state that means the ONU was reachable.
	assert.NotNil(t, stored.LastOnline)
}

func TestStoreSettledONUStatusStoresLOS(t *testing.T) {
	db := setupTestDB(t)
	ont, olt := provisionedONT(t)
	require.NoError(t, db.Create(&ont).Error)

	querier := &stubStatusQuerier{replies: []int{connectivity.PhaseStateLOS}}
	resolved, err := storeSettledONUStatus(db, querier, olt, ont, time.Second)
	require.NoError(t, err)
	assert.True(t, resolved)

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.Equal(t, models.ONTStatusLOS, stored.Status)
	assert.Equal(t, 1, querier.calls)
}

// An ONU the OLT never reports on is left for the backstop and the poll, not
// written as something the device did not say. The caller needs to be told, so
// it can carry on reading after the request has answered.
func TestStoreSettledONUStatusReportsAnUnresolvedONU(t *testing.T) {
	db := setupTestDB(t)
	ont, olt := provisionedONT(t)
	require.NoError(t, db.Create(&ont).Error)

	querier := &stubStatusQuerier{}
	resolved, err := storeSettledONUStatus(db, querier, olt, ont, 20*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, resolved)

	var stored models.ONT
	require.NoError(t, db.First(&stored, "id = ?", ont.ID).Error)
	assert.Equal(t, models.ONTStatusUnknown, stored.Status)
	assert.Greater(t, querier.calls, 1, "the window has to cover more than one read")
}

// A busy OLT answered a single GET in seven seconds while the ONU was ranging.
// A fixed count of attempts said nothing about how long that took, so the wait
// is bounded by the clock instead.
func TestStoreSettledONUStatusStopsAtTheWindow(t *testing.T) {
	db := setupTestDB(t)
	ont, olt := provisionedONT(t)
	require.NoError(t, db.Create(&ont).Error)

	start := time.Now()
	_, err := storeSettledONUStatus(db, &stubStatusQuerier{}, olt, ont, 40*time.Millisecond)
	require.NoError(t, err)

	assert.Less(t, time.Since(start), time.Second)
}

func TestStoreSettledONUStatusRefusesONTWithoutSlot(t *testing.T) {
	db := setupTestDB(t)
	ont, olt := provisionedONT(t)
	ont.Slot = nil

	_, err := storeSettledONUStatus(db, &stubStatusQuerier{}, olt, ont, time.Second)
	require.ErrorContains(t, err, "no slot")
}
