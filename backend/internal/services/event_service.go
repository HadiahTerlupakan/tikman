package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// EventService handles ONT event logging and availability calculations
type EventService struct {
	db *gorm.DB
}

// NewEventService creates a new EventService instance
func NewEventService(db *gorm.DB) *EventService {
	return &EventService{db: db}
}

// LogEvent records a new ONT status change event
func (s *EventService) LogEvent(event *models.ONTEvent) error {
	if event.EventType != models.EventTypeOnline && event.EventType != models.EventTypeOffline {
		return fmt.Errorf("invalid event type: %s", event.EventType)
	}

	return s.db.Create(event).Error
}

// GetEventsByONTID retrieves all events for a specific ONT with pagination
func (s *EventService) GetEventsByONTID(ontID uuid.UUID, limit, offset int) ([]models.ONTEvent, int64, error) {
	var events []models.ONTEvent
	var total int64

	// Get total count
	if err := s.db.Model(&models.ONTEvent{}).Where("ont_id = ?", ontID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated events
	if err := s.db.Where("ont_id = ?", ontID).
		Order("event_time DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

// GetEventsInTimeRange retrieves events within a specific time range
func (s *EventService) GetEventsInTimeRange(ontID uuid.UUID, startTime, endTime time.Time) ([]models.ONTEvent, error) {
	var events []models.ONTEvent

	if err := s.db.Where("ont_id = ? AND event_time BETWEEN ? AND ?", ontID, startTime, endTime).
		Order("event_time DESC").
		Find(&events).Error; err != nil {
		return nil, err
	}

	return events, nil
}

// AvailabilityStats represents availability metrics for an ONT
type AvailabilityStats struct {
	ONTID               uuid.UUID `json:"ont_id"`
	StartTime           time.Time `json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	TotalSeconds        int64     `json:"total_seconds"`
	OnlineSeconds       int64     `json:"online_seconds"`
	OfflineSeconds      int64     `json:"offline_seconds"`
	AvailabilityPercent float64   `json:"availability_percent"`
	TotalEvents         int       `json:"total_events"`
	OnlineEvents        int       `json:"online_events"`
	OfflineEvents       int       `json:"offline_events"`
	MTBF                float64   `json:"mtbf"` // Mean Time Between Failures (average online duration)
	MTTR                float64   `json:"mttr"` // Mean Time To Repair (average offline duration)
}

// CalculateAvailability computes availability metrics for a given time range
func (s *EventService) CalculateAvailability(ontID uuid.UUID, startTime, endTime time.Time) (*AvailabilityStats, error) {
	stats := &AvailabilityStats{
		ONTID:        ontID,
		StartTime:    startTime,
		EndTime:      endTime,
		TotalSeconds: int64(endTime.Sub(startTime).Seconds()),
	}

	events, err := s.GetEventsInTimeRange(ontID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	stats.TotalEvents = len(events)

	if len(events) == 0 {
		return s.availabilityFromCurrentStatus(stats)
	}

	online, offline := accumulateEventDurations(stats, events, endTime)

	// The window opens before the first event, and that stretch was spent in
	// whatever state the first event ended.
	first := events[len(events)-1]
	if first.EventTime.After(startTime) {
		before := int64(first.EventTime.Sub(startTime).Seconds())
		if first.EventType == models.EventTypeOnline {
			stats.OnlineSeconds += before
		} else {
			stats.OfflineSeconds += before
		}
	}

	if stats.TotalSeconds > 0 {
		stats.AvailabilityPercent = (float64(stats.OnlineSeconds) / float64(stats.TotalSeconds)) * 100.0
	}
	stats.MTBF = mean(online)
	stats.MTTR = mean(offline)

	return stats, nil
}

// availabilityFromCurrentStatus answers for an ONT that logged nothing in the
// window: with no transition to place, the state it is in now is the state it
// was in throughout.
func (s *EventService) availabilityFromCurrentStatus(stats *AvailabilityStats) (*AvailabilityStats, error) {
	var ont models.ONT
	if err := s.db.First(&ont, stats.ONTID).Error; err != nil {
		return nil, err
	}

	if ont.Status == "online" {
		stats.OnlineSeconds = stats.TotalSeconds
		stats.AvailabilityPercent = 100.0
	} else {
		stats.OfflineSeconds = stats.TotalSeconds
		stats.AvailabilityPercent = 0.0
	}
	return stats, nil
}

// accumulateEventDurations adds up how long each state was held, and returns
// the individual stretches so the means can be taken. Events arrive newest
// first, so an event runs until the one before it in the slice.
func accumulateEventDurations(stats *AvailabilityStats, events []models.ONTEvent, endTime time.Time) (online, offline []int64) {
	for i, event := range events {
		end := endTime
		if i > 0 {
			end = events[i-1].EventTime
		}
		duration := int64(end.Sub(event.EventTime).Seconds())

		if event.EventType == models.EventTypeOnline {
			stats.OnlineEvents++
			stats.OnlineSeconds += duration
			online = append(online, duration)
		} else {
			stats.OfflineEvents++
			stats.OfflineSeconds += duration
			offline = append(offline, duration)
		}
	}
	return online, offline
}

// mean is MTBF over the online stretches and MTTR over the offline ones. No
// stretches means no figure, rather than zero.
func mean(durations []int64) float64 {
	if len(durations) == 0 {
		return 0
	}
	var total int64
	for _, d := range durations {
		total += d
	}
	return float64(total) / float64(len(durations))
}

// GetLatestEvent retrieves the most recent event for an ONT
func (s *EventService) GetLatestEvent(ontID uuid.UUID) (*models.ONTEvent, error) {
	var event models.ONTEvent
	err := s.db.Where("ont_id = ?", ontID).Order("event_time DESC").First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// DeleteOldEvents removes events older than the specified duration
func (s *EventService) DeleteOldEvents(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)
	result := s.db.Where("event_time < ?", cutoffTime).Delete(&models.ONTEvent{})
	return result.RowsAffected, result.Error
}
