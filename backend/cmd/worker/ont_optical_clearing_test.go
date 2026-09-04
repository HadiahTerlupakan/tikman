package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ontWithReading(t *testing.T, db *gorm.DB, rx, tx float64) models.ONT {
	t.Helper()
	ont := models.ONT{
		ID:           uuid.New(),
		OLTID:        uuid.New(),
		SerialNumber: "ZTEG12345678",
		Status:       models.ONTStatusOnline,
		RxPower:      &rx,
		TxPower:      &tx,
	}
	require.NoError(t, db.Create(&ont).Error)
	return ont
}

func reload(t *testing.T, db *gorm.DB, id uuid.UUID) models.ONT {
	t.Helper()
	var row models.ONT
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	return row
}

// An ONT that has gone dark still has a row on the OLT, and the OLT answers for
// it with its no-signal sentinel — which decodes to nil. Writing only when the
// reading is non-nil left the column with no way back to empty, so a dead ONT
// kept displaying the last reading it had while it was alive, beside a status
// saying it was not.
func TestPollingAnONTWithNoSignalClearsItsOpticalReading(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	ont := ontWithReading(t, db, -22.22, 2.5)

	updateOntFields(db, ont, &connectivity.ONTMetrics{}, 1, zap.NewNop())

	after := reload(t, db, ont.ID)
	assert.Nil(t, after.RxPower, "a reading with no signal must not leave the old one behind")
	assert.Nil(t, after.TxPower)
}

// The dangerous half of the same change: a cycle that read nothing at all —
// the OLT unreachable, the walk timing out — must not be mistaken for "no
// signal". It taught us nothing, and clearing on it would wipe every good
// reading on the first blip.
func TestACycleThatReadNothingLeavesTheReadingAlone(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	ont := ontWithReading(t, db, -22.22, 2.5)

	updateOntFields(db, ont, nil, 1, zap.NewNop())

	after := reload(t, db, ont.ID)
	require.NotNil(t, after.RxPower)
	assert.InDelta(t, -22.22, *after.RxPower, 0.001)
	require.NotNil(t, after.TxPower)
	assert.InDelta(t, 2.5, *after.TxPower, 0.001)
}

// And the ordinary case still has to work: a real reading replaces the old one.
func TestPollingWithASignalWritesTheNewReading(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	ont := ontWithReading(t, db, -22.22, 2.5)

	rx, tx := -18.40, 3.1
	updateOntFields(db, ont, &connectivity.ONTMetrics{RxPower: &rx, TxPower: &tx}, 1, zap.NewNop())

	after := reload(t, db, ont.ID)
	require.NotNil(t, after.RxPower)
	assert.InDelta(t, -18.40, *after.RxPower, 0.001)
	require.NotNil(t, after.TxPower)
	assert.InDelta(t, 3.1, *after.TxPower, 0.001)
}
