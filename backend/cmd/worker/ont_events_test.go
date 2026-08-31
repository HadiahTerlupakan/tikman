package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// Nothing in the running worker used to write ont_events: the only writer was the
// /admin/seed-events endpoint, so an ONT's Events tab and its availability figure
// were empty unless someone had seeded demo data.
//
// The subtle part is that logging only on a status *change* does not fix it. An
// ONT registered with the status its OLT already reports never changes, so it
// would never get the opening event that availability measures from - and that is
// every ONT on a freshly added OLT. This pins the idempotent behaviour the worker
// now relies on.
// A chassis-less OLT such as the HSGQ XE08ID has no card slots and reports slot
// 0. The worker's update guard used to be "discoveredSlot > 0", so those ONTs
// kept a NULL slot forever, and GetRealtimeMetrics rejects a NULL slot - which is
// what left the Traffic Statistics tab empty for every HSGQ ONT. Slot 0 has to be
// stored as the fact it is, distinct from "not discovered".
func TestSlotZeroIsStoredNotTreatedAsMissing(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)

	ont := models.ONT{
		ID:           uuid.New(),
		OLTID:        uuid.New(),
		PortID:       1,
		ONTID:        1,
		SerialNumber: "EC237BD71FA8",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&ont).Error)

	var before models.ONT
	require.NoError(t, db.First(&before, "id = ?", ont.ID).Error)
	require.Nil(t, before.Slot, "a freshly registered ONT starts with no slot")

	// This mirrors what the worker writes once the OLT has matched the ONT.
	require.NoError(t, db.Table("onts").Where("id = ?", ont.ID).
		Updates(map[string]interface{}{"slot": 0}).Error)

	var after models.ONT
	require.NoError(t, db.First(&after, "id = ?", ont.ID).Error)
	require.NotNil(t, after.Slot, "slot 0 must persist; NULL makes realtime metrics refuse the ONT")
	assert.Equal(t, 0, *after.Slot)
}

func TestLogStatusChangeOpensBaselineForStableONT(t *testing.T) {
	db := setupWorkerOLTStatusTestDB(t)
	eventService := services.NewEventService(db)

	ont := models.ONT{
		ID:           uuid.New(),
		OLTID:        uuid.New(),
		PortID:       1,
		ONTID:        1,
		SerialNumber: "EC237BD71FA8",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, db.Create(&ont).Error)

	countEvents := func() int64 {
		var n int64
		require.NoError(t, db.Model(&models.ONTEvent{}).Where("ont_id = ?", ont.ID).Count(&n).Error)
		return n
	}

	// First poll of an ONT that has never changed state must still record it,
	// otherwise there is no interval to compute availability over.
	require.NoError(t, eventService.LogStatusChanges([]services.StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: string(models.ONTStatusOnline)}}))
	assert.Equal(t, int64(1), countEvents(), "a stable ONT must still get a baseline event")

	// Repeated polls with an unchanged state must not pile up duplicates - the
	// worker now calls this every cycle.
	for i := 0; i < 3; i++ {
		require.NoError(t, eventService.LogStatusChanges([]services.StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: string(models.ONTStatusOnline)}}))
	}
	assert.Equal(t, int64(1), countEvents(), "unchanged polls must not duplicate events")

	// A real transition appends an event and closes out the previous one.
	require.NoError(t, eventService.LogStatusChanges([]services.StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOffline, Reason: string(models.ONTStatusOffline)}}))
	assert.Equal(t, int64(2), countEvents())

	var first models.ONTEvent
	require.NoError(t, db.Where("ont_id = ?", ont.ID).Order("event_time ASC").First(&first).Error)
	assert.Equal(t, models.EventTypeOnline, first.EventType)
	require.NotNil(t, first.DurationSeconds, "the closed event needs a duration for availability maths")
}
