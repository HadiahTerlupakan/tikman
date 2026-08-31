package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func createTestOLT(t *testing.T, db *gorm.DB, siteID uuid.UUID) *models.OLT {
	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            siteID,
		Name:              "Test OLT",
		IPAddress:         "192.168.1.1",
		SNMPCommunity:     "public",
		Username:          "admin",
		Password:          "encrypted",
		SSHPort:           22,
		TelnetPort:        23,
		SNMPPort:          161,
		PreferredProtocol: models.OLTProtocolSSH,
		Status:            models.OLTStatusOnline,
	}
	require.NoError(t, db.Create(olt).Error)
	return olt
}

func createTestONT(t *testing.T, ontService *ONTService, oltID uuid.UUID, serial string, portID, ontID int) *models.ONT {
	ont := &models.ONT{
		OLTID:        oltID,
		PortID:       portID,
		ONTID:        ontID,
		SerialNumber: serial,
		Name:         "Test ONT",
		Status:       models.ONTStatusOnline,
	}
	require.NoError(t, ontService.Create(ont))
	return ont
}

func TestEventService_NewEventService(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	assert.NotNil(t, service)
	assert.NotNil(t, service.db)
}

func TestEventService_LogEvent_Valid(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN001", 1, 1)

	event := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: time.Now(),
		Reason:    "Test online event",
	}

	err = service.LogEvent(event)
	require.NoError(t, err)

	var savedEvent models.ONTEvent
	result := db.Where("ont_id = ?", ont.ID).First(&savedEvent)
	require.NoError(t, result.Error)
	assert.Equal(t, models.EventTypeOnline, savedEvent.EventType)
	assert.Equal(t, "Test online event", savedEvent.Reason)
}

func TestEventService_LogEvent_InvalidType(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN002", 1, 1)

	event := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: "invalid",
		EventTime: time.Now(),
	}

	err = service.LogEvent(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event type")
}

func TestEventService_LogEvent_OfflineEvent(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN003", 1, 1)

	event := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOffline,
		EventTime: time.Now(),
		Reason:    "LOS",
	}

	err = service.LogEvent(event)
	require.NoError(t, err)

	var savedEvent models.ONTEvent
	result := db.Where("ont_id = ?", ont.ID).First(&savedEvent)
	require.NoError(t, result.Error)
	assert.Equal(t, models.EventTypeOffline, savedEvent.EventType)
}

func TestEventService_LogStatusChange_FirstEvent(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN004", 1, 1)

	err = service.LogStatusChanges([]StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: "Initial online"}})
	require.NoError(t, err)

	var event models.ONTEvent
	result := db.Where("ont_id = ?", ont.ID).Order("event_time DESC").First(&event)
	require.NoError(t, result.Error)
	assert.Equal(t, models.EventTypeOnline, event.EventType)
	assert.Equal(t, "Initial online", event.Reason)
}

func TestEventService_LogStatusChange_NoStatusChange(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN005", 1, 1)

	err = service.LogStatusChanges([]StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: "Online"}})
	require.NoError(t, err)

	err = service.LogStatusChanges([]StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: "Still online"}})
	require.NoError(t, err)

	var count int64
	db.Model(&models.ONTEvent{}).Where("ont_id = ?", ont.ID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestEventService_LogStatusChange_StatusChange(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN006", 1, 1)

	err = service.LogStatusChanges([]StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOnline, Reason: "Online"}})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	err = service.LogStatusChanges([]StatusChange{{ONTID: ont.ID, EventType: models.EventTypeOffline, Reason: "LOS"}})
	require.NoError(t, err)

	var count int64
	db.Model(&models.ONTEvent{}).Where("ont_id = ?", ont.ID).Count(&count)
	assert.Equal(t, int64(2), count)

	var firstEvent models.ONTEvent
	db.Where("ont_id = ?", ont.ID).Order("event_time ASC").First(&firstEvent)
	assert.NotNil(t, firstEvent.DurationSeconds)
	assert.GreaterOrEqual(t, *firstEvent.DurationSeconds, int64(0))
}

func TestEventService_GetEventsByONTID_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN007", 1, 1)

	events, total, err := service.GetEventsByONTID(ont.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Len(t, events, 0)
}

func TestEventService_GetEventsByONTID_WithData(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN008", 1, 1)

	now := time.Now()
	for i := 0; i < 5; i++ {
		event := &models.ONTEvent{
			ONTID:     ont.ID,
			EventType: models.EventTypeOnline,
			EventTime: now.Add(time.Duration(i) * time.Second),
			Reason:    "Test event",
		}
		err = service.LogEvent(event)
		require.NoError(t, err)
	}

	events, total, err := service.GetEventsByONTID(ont.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, events, 5)

	for i := 0; i < len(events)-1; i++ {
		assert.True(t, events[i].EventTime.After(events[i+1].EventTime))
	}
}

func TestEventService_GetEventsByONTID_Pagination(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN009", 1, 1)

	now := time.Now()
	for i := 0; i < 10; i++ {
		event := &models.ONTEvent{
			ONTID:     ont.ID,
			EventType: models.EventTypeOnline,
			EventTime: now.Add(time.Duration(i) * time.Second),
			Reason:    "Test event",
		}
		err = service.LogEvent(event)
		require.NoError(t, err)
	}

	events1, total, err := service.GetEventsByONTID(ont.ID, 3, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, events1, 3)

	events2, total, err := service.GetEventsByONTID(ont.ID, 3, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, events2, 3)

	assert.NotEqual(t, events1[0].ID, events2[0].ID)
}

func TestEventService_GetEventsInTimeRange_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN010", 1, 1)

	startTime := time.Now()
	endTime := startTime.Add(time.Hour)

	events, err := service.GetEventsInTimeRange(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestEventService_GetEventsInTimeRange_WithData(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN011", 1, 1)

	baseTime := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime.Add(-30 * time.Minute),
		Reason:    "Before range",
	}
	event2 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime,
		Reason:    "In range",
	}
	event3 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime.Add(30 * time.Minute),
		Reason:    "After range",
	}

	require.NoError(t, service.LogEvent(event1))
	require.NoError(t, service.LogEvent(event2))
	require.NoError(t, service.LogEvent(event3))

	startTime := baseTime.Add(-5 * time.Minute)
	endTime := baseTime.Add(5 * time.Minute)

	events, err := service.GetEventsInTimeRange(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "In range", events[0].Reason)
}

func TestEventService_GetLatestEvent_NotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN012", 1, 1)

	event, err := service.GetLatestEvent(ont.ID)
	require.Error(t, err)
	assert.Nil(t, event)
}

func TestEventService_GetLatestEvent_Found(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN013", 1, 1)

	now := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: now.Add(-10 * time.Second),
		Reason:    "First",
	}
	event2 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOffline,
		EventTime: now,
		Reason:    "Latest",
	}

	require.NoError(t, service.LogEvent(event1))
	require.NoError(t, service.LogEvent(event2))

	latest, err := service.GetLatestEvent(ont.ID)
	require.NoError(t, err)
	assert.Equal(t, "Latest", latest.Reason)
	assert.Equal(t, models.EventTypeOffline, latest.EventType)
}

func TestEventService_DeleteOldEvents(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN014", 1, 1)

	now := time.Now()
	oldEvent := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: now.Add(-24 * time.Hour),
		Reason:    "Old event",
	}
	newEvent := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: now,
		Reason:    "New event",
	}

	require.NoError(t, service.LogEvent(oldEvent))
	require.NoError(t, service.LogEvent(newEvent))

	deleted, err := service.DeleteOldEvents(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	events, total, err := service.GetEventsByONTID(ont.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "New event", events[0].Reason)
}

func TestEventService_CalculateAvailability_NoEvents(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN015", 1, 1)

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, ont.ID, stats.ONTID)
	assert.Equal(t, 100.0, stats.AvailabilityPercent)
	assert.Equal(t, 0, stats.TotalEvents)
}

func TestEventService_CalculateAvailability_WithEvents(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN016", 1, 1)

	baseTime := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime.Add(-1 * time.Hour),
		Reason:    "Online",
	}
	event2 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOffline,
		EventTime: baseTime.Add(-30 * time.Minute),
		Reason:    "LOS",
	}
	event3 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime,
		Reason:    "Back online",
	}

	require.NoError(t, service.LogEvent(event1))
	require.NoError(t, service.LogEvent(event2))
	require.NoError(t, service.LogEvent(event3))

	startTime := baseTime.Add(-2 * time.Hour)
	endTime := baseTime.Add(time.Hour)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalEvents)
	assert.Equal(t, 2, stats.OnlineEvents)
	assert.Equal(t, 1, stats.OfflineEvents)
	assert.Greater(t, stats.AvailabilityPercent, 0.0)
	assert.Greater(t, stats.MTBF, 0.0)
	assert.Greater(t, stats.MTTR, 0.0)
}

func TestEventService_CalculateAvailability_MTBF_MTTR(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN017", 1, 1)

	baseTime := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime.Add(-120 * time.Second),
		Reason:    "Online",
	}
	event2 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOffline,
		EventTime: baseTime.Add(-60 * time.Second),
		Reason:    "Offline",
	}
	event3 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime,
		Reason:    "Back online",
	}

	require.NoError(t, service.LogEvent(event1))
	require.NoError(t, service.LogEvent(event2))
	require.NoError(t, service.LogEvent(event3))

	startTime := baseTime.Add(-240 * time.Second)
	endTime := baseTime.Add(60 * time.Second)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Greater(t, stats.MTBF, 0.0)
	assert.Greater(t, stats.MTTR, 0.0)
}

func TestEventService_CalculateAvailability_OnlineOnly(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN018", 1, 1)

	baseTime := time.Now()
	for i := 0; i < 3; i++ {
		event := &models.ONTEvent{
			ONTID:     ont.ID,
			EventType: models.EventTypeOnline,
			EventTime: baseTime.Add(time.Duration(i) * time.Minute),
			Reason:    "Online",
		}
		require.NoError(t, service.LogEvent(event))
	}

	startTime := baseTime.Add(-1 * time.Hour)
	endTime := baseTime.Add(1 * time.Hour)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.OnlineEvents)
	assert.Equal(t, 0, stats.OfflineEvents)
	assert.Greater(t, stats.AvailabilityPercent, 0.0)
	assert.Greater(t, stats.MTBF, 0.0)
	assert.Equal(t, 0.0, stats.MTTR)
}

func TestEventService_CalculateAvailability_OfflineOnly(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN019", 1, 1)

	baseTime := time.Now()
	for i := 0; i < 2; i++ {
		event := &models.ONTEvent{
			ONTID:     ont.ID,
			EventType: models.EventTypeOffline,
			EventTime: baseTime.Add(time.Duration(i) * time.Minute),
			Reason:    "Offline",
		}
		require.NoError(t, service.LogEvent(event))
	}

	startTime := baseTime.Add(-1 * time.Hour)
	endTime := baseTime.Add(1 * time.Hour)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.OfflineEvents)
	assert.Equal(t, 0, stats.OnlineEvents)
	assert.Equal(t, 0.0, stats.MTBF)
	assert.Greater(t, stats.MTTR, 0.0)
}

func TestEventService_GetEventsByONTID_WithOffset(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN020", 1, 1)

	now := time.Now()
	for i := 0; i < 5; i++ {
		event := &models.ONTEvent{
			ONTID:     ont.ID,
			EventType: models.EventTypeOnline,
			EventTime: now.Add(time.Duration(i) * time.Second),
			Reason:    "Test event",
		}
		err = service.LogEvent(event)
		require.NoError(t, err)
	}

	events, total, err := service.GetEventsByONTID(ont.ID, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, events, 2)
}

func TestEventService_LogStatusChange_WithError(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	invalidONTID := uuid.New()

	err := service.LogStatusChanges([]StatusChange{{ONTID: invalidONTID, EventType: models.EventTypeOnline, Reason: "Online"}})
	require.NoError(t, err)

	var count int64
	db.Model(&models.ONTEvent{}).Where("ont_id = ?", invalidONTID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestEventService_CalculateAvailability_PreFirstEventTime(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN021", 1, 1)

	baseTime := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime,
		Reason:    "Online",
	}

	require.NoError(t, service.LogEvent(event1))

	startTime := baseTime.Add(-1 * time.Hour)
	endTime := baseTime.Add(1 * time.Hour)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Greater(t, stats.OnlineSeconds, int64(0))
	assert.Greater(t, stats.AvailabilityPercent, 0.0)
}

func TestEventService_CalculateAvailability_TotalSecondsZero(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN022", 1, 1)

	baseTime := time.Now()
	startTime := baseTime
	endTime := baseTime

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.TotalSeconds)
}

func TestEventService_DeleteOldEvents_NoOldEvents(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN023", 1, 1)

	now := time.Now()
	newEvent := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: now,
		Reason:    "New event",
	}

	require.NoError(t, service.LogEvent(newEvent))

	deleted, err := service.DeleteOldEvents(time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	retrievedEvents, total, err := service.GetEventsByONTID(ont.ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, retrievedEvents, 1)
}

func TestEventService_GetEventsInTimeRange_ExactBoundaries(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN024", 1, 1)

	baseTime := time.Now()
	event1 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime,
		Reason:    "At start",
	}
	event2 := &models.ONTEvent{
		ONTID:     ont.ID,
		EventType: models.EventTypeOnline,
		EventTime: baseTime.Add(time.Hour),
		Reason:    "At end",
	}

	require.NoError(t, service.LogEvent(event1))
	require.NoError(t, service.LogEvent(event2))

	startTime := baseTime
	endTime := baseTime.Add(time.Hour)

	events, err := service.GetEventsInTimeRange(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestEventService_CalculateAvailability_MultipleTransitions(t *testing.T) {
	db := setupTestDB(t)
	service := NewEventService(db)

	siteService := NewSiteService(db)
	site, err := siteService.Create("Test Site", "Test Location", "Test Description")
	require.NoError(t, err)

	olt := createTestOLT(t, db, site.ID)
	ontService := NewONTService(db)
	ont := createTestONT(t, ontService, olt.ID, "SN025", 1, 1)

	baseTime := time.Now()
	eventsToLog := []*models.ONTEvent{
		{ONTID: ont.ID, EventType: models.EventTypeOnline, EventTime: baseTime.Add(-4 * time.Minute), Reason: "Online"},
		{ONTID: ont.ID, EventType: models.EventTypeOffline, EventTime: baseTime.Add(-3 * time.Minute), Reason: "Offline"},
		{ONTID: ont.ID, EventType: models.EventTypeOnline, EventTime: baseTime.Add(-2 * time.Minute), Reason: "Online"},
		{ONTID: ont.ID, EventType: models.EventTypeOffline, EventTime: baseTime.Add(-1 * time.Minute), Reason: "Offline"},
		{ONTID: ont.ID, EventType: models.EventTypeOnline, EventTime: baseTime, Reason: "Online"},
	}

	for _, ev := range eventsToLog {
		require.NoError(t, service.LogEvent(ev))
	}

	startTime := baseTime.Add(-5 * time.Minute)
	endTime := baseTime.Add(time.Minute)

	stats, err := service.CalculateAvailability(ont.ID, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, 5, stats.TotalEvents)
	assert.Equal(t, 3, stats.OnlineEvents)
	assert.Equal(t, 2, stats.OfflineEvents)
	assert.Greater(t, stats.MTBF, 0.0)
	assert.Greater(t, stats.MTTR, 0.0)
}
