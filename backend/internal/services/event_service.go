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

// LogStatusChange detects and logs status changes with duration calculation
func (s *EventService) LogStatusChange(ontID uuid.UUID, newStatus string, reason string) error {
	// Get last event for this ONT
	var lastEvent models.ONTEvent
	err := s.db.Where("ont_id = ?", ontID).Order("event_time DESC").First(&lastEvent).Error

	now := time.Now()

	// If no previous event exists, just log the current state
	if err == gorm.ErrRecordNotFound {
		return s.LogEvent(&models.ONTEvent{
			ONTID:     ontID,
			EventType: newStatus,
			EventTime: now,
			Reason:    reason,
		})
	}

	if err != nil {
		return err
	}

	// If status hasn't changed, do nothing
	if lastEvent.EventType == newStatus {
		return nil
	}

	// Calculate duration for the previous event
	duration := int64(now.Sub(lastEvent.EventTime).Seconds())
	
	// Update previous event with duration
	if err := s.db.Model(&lastEvent).Update("duration_seconds", duration).Error; err != nil {
		return err
	}

	// Log new event
	return s.LogEvent(&models.ONTEvent{
		ONTID:     ontID,
		EventType: newStatus,
		EventTime: now,
		Reason:    reason,
	})
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
	ONTID              uuid.UUID `json:"ont_id"`
	StartTime          time.Time `json:"start_time"`
	EndTime            time.Time `json:"end_time"`
	TotalSeconds       int64     `json:"total_seconds"`
	OnlineSeconds      int64     `json:"online_seconds"`
	OfflineSeconds     int64     `json:"offline_seconds"`
	AvailabilityPercent float64   `json:"availability_percent"`
	TotalEvents        int       `json:"total_events"`
	OnlineEvents       int       `json:"online_events"`
	OfflineEvents      int       `json:"offline_events"`
	MTBF               float64   `json:"mtbf"` // Mean Time Between Failures (average online duration)
	MTTR               float64   `json:"mttr"` // Mean Time To Repair (average offline duration)
}

// CalculateAvailability computes availability metrics for a given time range
func (s *EventService) CalculateAvailability(ontID uuid.UUID, startTime, endTime time.Time) (*AvailabilityStats, error) {
	stats := &AvailabilityStats{
		ONTID:     ontID,
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Calculate total time window in seconds
	stats.TotalSeconds = int64(endTime.Sub(startTime).Seconds())

	// Get all events in the time range
	events, err := s.GetEventsInTimeRange(ontID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	stats.TotalEvents = len(events)

	if len(events) == 0 {
		// No events - check current status
		var ont models.ONT
		if err := s.db.First(&ont, ontID).Error; err != nil {
			return nil, err
		}

		// Assume current status for the entire period
		if ont.Status == "online" {
			stats.OnlineSeconds = stats.TotalSeconds
			stats.AvailabilityPercent = 100.0
		} else {
			stats.OfflineSeconds = stats.TotalSeconds
			stats.AvailabilityPercent = 0.0
		}

		return stats, nil
	}

	// Process events to calculate online/offline time
	var onlineDurations []int64
	var offlineDurations []int64

	for i := 0; i < len(events); i++ {
		event := events[i]

		// Count event types
		if event.EventType == models.EventTypeOnline {
			stats.OnlineEvents++
		} else {
			stats.OfflineEvents++
		}

		// Calculate duration for this event
		var eventEnd time.Time
		if i > 0 {
			// Next event marks the end of this period
			eventEnd = events[i-1].EventTime
		} else {
			// Last event extends to endTime
			eventEnd = endTime
		}

		duration := int64(eventEnd.Sub(event.EventTime).Seconds())

		if event.EventType == models.EventTypeOnline {
			stats.OnlineSeconds += duration
			onlineDurations = append(onlineDurations, duration)
		} else {
			stats.OfflineSeconds += duration
			offlineDurations = append(offlineDurations, duration)
		}
	}

	// Handle time before first event (assume same state as first event)
	firstEventInWindow := events[len(events)-1]
	if firstEventInWindow.EventTime.After(startTime) {
		preDuration := int64(firstEventInWindow.EventTime.Sub(startTime).Seconds())
		if firstEventInWindow.EventType == models.EventTypeOnline {
			stats.OnlineSeconds += preDuration
		} else {
			stats.OfflineSeconds += preDuration
		}
	}

	// Calculate availability percentage
	if stats.TotalSeconds > 0 {
		stats.AvailabilityPercent = (float64(stats.OnlineSeconds) / float64(stats.TotalSeconds)) * 100.0
	}

	// Calculate MTBF (Mean Time Between Failures)
	if len(onlineDurations) > 0 {
		var totalOnline int64
		for _, d := range onlineDurations {
			totalOnline += d
		}
		stats.MTBF = float64(totalOnline) / float64(len(onlineDurations))
	}

	// Calculate MTTR (Mean Time To Repair)
	if len(offlineDurations) > 0 {
		var totalOffline int64
		for _, d := range offlineDurations {
			totalOffline += d
		}
		stats.MTTR = float64(totalOffline) / float64(len(offlineDurations))
	}

	return stats, nil
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
