package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sqlRecorder keeps every statement GORM executes, so a test can assert how
// many times a page of ONTs reads the event history rather than only what it
// wrote. The count is the point of the batch path: the per-ONT read is the
// ceiling that a chassis of a hundred thousand subscribers runs into.
type sqlRecorder struct {
	logger.Interface
	statements []string
}

func (r *sqlRecorder) LogMode(logger.LogLevel) logger.Interface { return r }

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.statements = append(r.statements, sql)
}

func (r *sqlRecorder) selectsAgainst(table string) int {
	count := 0
	for _, sql := range r.statements {
		trimmed := strings.TrimSpace(sql)
		if strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") && strings.Contains(trimmed, table) {
			count++
		}
	}
	return count
}

// batchFixture is one OLT with the requested number of ONTs.
type batchFixture struct {
	db   *gorm.DB
	onts []*models.ONT
}

func newBatchFixture(t *testing.T, db *gorm.DB, ontCount int) batchFixture {
	t.Helper()

	site, err := NewSiteService(db).Create("Batch Site", "Location", "Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)

	onts := make([]*models.ONT, 0, ontCount)
	for i := 0; i < ontCount; i++ {
		onts = append(onts, createTestONT(t, ontService, olt.ID, uuid.NewString()[:12], 1, i+1))
	}
	return batchFixture{db: db, onts: onts}
}

func (f batchFixture) changes(eventType string, reason string) []StatusChange {
	changes := make([]StatusChange, 0, len(f.onts))
	for _, ont := range f.onts {
		changes = append(changes, StatusChange{ONTID: ont.ID, EventType: eventType, Reason: reason})
	}
	return changes
}

func (f batchFixture) eventsFor(t *testing.T, ontID uuid.UUID) []models.ONTEvent {
	t.Helper()
	var events []models.ONTEvent
	require.NoError(t, f.db.Where("ont_id = ?", ontID).Order("event_time ASC, id ASC").Find(&events).Error)
	return events
}

func TestLogStatusChangesOpensABaselineForAnONTWithNoHistory(t *testing.T) {
	db := setupTestDB(t)
	fixture := newBatchFixture(t, db, 3)

	require.NoError(t, NewEventService(db).LogStatusChanges(fixture.changes(models.EventTypeOnline, "poll")))

	for _, ont := range fixture.onts {
		events := fixture.eventsFor(t, ont.ID)
		require.Len(t, events, 1)
		assert.Equal(t, models.EventTypeOnline, events[0].EventType)
		assert.Equal(t, "poll", events[0].Reason)
		assert.Nil(t, events[0].DurationSeconds)
	}
}

func TestLogStatusChangesWritesNothingWhenTheStatusIsUnchanged(t *testing.T) {
	db := setupTestDB(t)
	fixture := newBatchFixture(t, db, 2)
	service := NewEventService(db)

	require.NoError(t, service.LogStatusChanges(fixture.changes(models.EventTypeOnline, "first")))
	require.NoError(t, service.LogStatusChanges(fixture.changes(models.EventTypeOnline, "second")))

	for _, ont := range fixture.onts {
		// The second pass found the same state, so it must not open a second
		// event: availability measures intervals between transitions, and a row
		// per cycle would make every interval zero.
		assert.Len(t, fixture.eventsFor(t, ont.ID), 1)
	}
}

func TestLogStatusChangesClosesThePreviousEventOnATransition(t *testing.T) {
	db := setupTestDB(t)
	fixture := newBatchFixture(t, db, 1)
	service := NewEventService(db)
	ont := fixture.onts[0]

	require.NoError(t, service.LogStatusChanges([]StatusChange{
		{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: "up"},
	}))

	// Backdate the opening event so the closed duration is a number the test can
	// state, rather than the zero a same-second transition would produce.
	require.NoError(t, db.Model(&models.ONTEvent{}).Where("ont_id = ?", ont.ID).
		Update("event_time", time.Now().Add(-90*time.Second)).Error)

	require.NoError(t, service.LogStatusChanges([]StatusChange{
		{ONTID: ont.ID, EventType: models.EventTypeOffline, Reason: "los"},
	}))

	events := fixture.eventsFor(t, ont.ID)
	require.Len(t, events, 2)

	require.NotNil(t, events[0].DurationSeconds)
	assert.InDelta(t, 90, *events[0].DurationSeconds, 2)
	assert.Equal(t, models.EventTypeOffline, events[1].EventType)
	assert.Equal(t, "los", events[1].Reason)
	assert.Nil(t, events[1].DurationSeconds)
}

func TestLogStatusChangesReadsTheHistoryOnceForTheWholePage(t *testing.T) {
	recorder := &sqlRecorder{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: recorder})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	const ontCount = 50
	fixture := newBatchFixture(t, db, ontCount)
	service := NewEventService(db)
	require.NoError(t, service.LogStatusChanges(fixture.changes(models.EventTypeOnline, "first")))

	recorder.statements = nil
	require.NoError(t, service.LogStatusChanges(fixture.changes(models.EventTypeOnline, "second")))

	// One read for fifty ONTs. The per-ONT version issued fifty, which is the
	// cost that grows with the subscriber count on every single cycle.
	assert.Equal(t, 1, recorder.selectsAgainst("ont_events"))
}

func TestLogStatusChangesRejectsAnInvalidEventTypeBeforeWritingAnything(t *testing.T) {
	db := setupTestDB(t)
	fixture := newBatchFixture(t, db, 2)

	err := NewEventService(db).LogStatusChanges([]StatusChange{
		{ONTID: fixture.onts[0].ID, EventType: models.EventTypeOnline},
		{ONTID: fixture.onts[1].ID, EventType: "transitional"},
	})
	require.Error(t, err)

	// The valid entry must not have been written either: a page is validated
	// before it is applied, so a bad entry cannot leave half a page recorded.
	assert.Empty(t, fixture.eventsFor(t, fixture.onts[0].ID))
}
