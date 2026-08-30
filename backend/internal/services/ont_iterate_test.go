package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestEachONTOfOLTVisitsEveryRowBeyondOnePage(t *testing.T) {
	// The worker used to take one page of 1000 rows and treat it as the network.
	// At any real size that silently stops monitoring most of it, and nothing in
	// the system says so.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	const total = 2500
	rows := make([]models.ONT, 0, total)
	for i := 0; i < total; i++ {
		rows = append(rows, models.ONT{
			ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: i,
			SerialNumber: fmt.Sprintf("SN%08d", i), Status: models.ONTStatusOnline,
		})
	}
	require.NoError(t, db.CreateInBatches(rows, 500).Error)

	seen := 0
	batches := 0
	require.NoError(t, ontService.EachONTOfOLT(oltID, 1000, func(batch []models.ONT) error {
		seen += len(batch)
		batches++
		return nil
	}))

	require.Equal(t, total, seen)
	require.Equal(t, 3, batches, "paging did not advance in whole pages")
}

func TestEachONTOfOLTHandsBackEveryDistinctONT(t *testing.T) {
	// A cursor that failed to advance would re-serve the same page forever, and
	// a cursor that skipped would drop subscribers. Counting rows alone catches
	// neither.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")

	const total = 250
	for i := 0; i < total; i++ {
		require.NoError(t, db.Create(&models.ONT{
			ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: i,
			SerialNumber: fmt.Sprintf("SN%08d", i), Status: models.ONTStatusOnline,
		}).Error)
	}

	seen := map[uuid.UUID]bool{}
	require.NoError(t, ontService.EachONTOfOLT(oltID, 40, func(batch []models.ONT) error {
		for _, ont := range batch {
			require.False(t, seen[ont.ID], "the same ONT was handed back twice")
			seen[ont.ID] = true
		}
		return nil
	}))

	require.Len(t, seen, total)
}

func TestEachONTOfOLTIgnoresOtherOLTs(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	cariu := oltForPositions(t, db, "Cariu", "172.30.30.3")
	bekasi := oltForPositions(t, db, "Bekasi", "172.30.30.2")

	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: cariu, PortID: 1, ONTID: 1,
		SerialNumber: "SNCARIU01", Status: models.ONTStatusOnline,
	}).Error)
	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: bekasi, PortID: 1, ONTID: 1,
		SerialNumber: "SNBEKASI1", Status: models.ONTStatusOnline,
	}).Error)

	seen := 0
	require.NoError(t, ontService.EachONTOfOLT(cariu, 100, func(batch []models.ONT) error {
		for _, ont := range batch {
			require.Equal(t, "SNCARIU01", ont.SerialNumber)
			seen++
		}
		return nil
	}))

	require.Equal(t, 1, seen)
}

func TestEachONTOfOLTStopsWhenTheCallbackFails(t *testing.T) {
	// The callback writes metrics. Carrying on after a write failure would
	// report a complete cycle over a partial one.
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Cariu", "172.30.30.3")
	require.NoError(t, db.Create(&models.ONT{
		ID: uuid.New(), OLTID: oltID, PortID: 1, ONTID: 1,
		SerialNumber: "SN1", Status: models.ONTStatusOnline,
	}).Error)

	boom := errors.New("write failed")
	require.ErrorIs(t, ontService.EachONTOfOLT(oltID, 100, func([]models.ONT) error {
		return boom
	}), boom)
}

func TestEachONTOfOLTOnAnOLTWithNoONTs(t *testing.T) {
	db := setupTestDB(t)
	ontService := NewONTService(db)
	oltID := oltForPositions(t, db, "Empty", "172.30.30.9")

	called := false
	require.NoError(t, ontService.EachONTOfOLT(oltID, 100, func([]models.ONT) error {
		called = true
		return nil
	}))

	require.False(t, called, "an empty OLT should hand back no batches at all")
}
