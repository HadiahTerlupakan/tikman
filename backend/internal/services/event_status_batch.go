package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// eventInsertBatch is how many opened events go into one INSERT. It matters
// only on the first pass over a newly added OLT, where every ONT needs a
// baseline; after that a page opens events for the few that actually changed.
const eventInsertBatch = 500

// StatusChange is one ONT's status as a poll cycle found it.
type StatusChange struct {
	ONTID     uuid.UUID
	EventType string
	Reason    string
}

// LogStatusChanges records a page of ONT statuses, reading the event history
// once for the whole page.
//
// It is LogStatusChange applied to many ONTs, and exists because that method
// reads the last event per ONT: at a hundred thousand subscribers a one-minute
// status tier spends a hundred thousand queries a minute establishing, for
// almost every one of them, that nothing changed. The reads collapse into one;
// the writes stay per-ONT because only a handful of ONTs transition in a cycle.
func (s *EventService) LogStatusChanges(changes []StatusChange) error {
	if len(changes) == 0 {
		return nil
	}

	// Validated before anything is written, so one bad entry cannot leave half
	// a page recorded.
	for _, change := range changes {
		if change.EventType != models.EventTypeOnline && change.EventType != models.EventTypeOffline {
			return fmt.Errorf("invalid event type: %s", change.EventType)
		}
	}

	ontIDs := make([]uuid.UUID, 0, len(changes))
	for _, change := range changes {
		ontIDs = append(ontIDs, change.ONTID)
	}

	latest, err := s.latestEventPerONT(ontIDs)
	if err != nil {
		return err
	}

	now := time.Now()
	opened := make([]models.ONTEvent, 0, len(changes))

	for _, change := range changes {
		previous, hasHistory := latest[change.ONTID]
		if hasHistory && previous.EventType == change.EventType {
			continue
		}

		if hasHistory {
			duration := int64(now.Sub(previous.EventTime).Seconds())
			if err := s.db.Model(&models.ONTEvent{}).Where("id = ?", previous.ID).
				Update("duration_seconds", duration).Error; err != nil {
				return err
			}
		}

		opened = append(opened, models.ONTEvent{
			ONTID:     change.ONTID,
			EventType: change.EventType,
			EventTime: now,
			Reason:    change.Reason,
		})
	}

	if len(opened) == 0 {
		return nil
	}
	return s.db.CreateInBatches(&opened, eventInsertBatch).Error
}

// latestEventPerONT returns each ONT's most recent event, keyed by ONT.
//
// The ranking is done in SQL rather than by fetching the ONTs' events and
// picking in Go: an ONT accumulates an event per transition for as long as it
// exists, so reading them all to keep one would grow with history rather than
// with the page.
func (s *EventService) latestEventPerONT(ontIDs []uuid.UUID) (map[uuid.UUID]models.ONTEvent, error) {
	var rows []models.ONTEvent

	// id breaks the tie because a cycle records every ONT with the same
	// timestamp, so event_time alone does not order two events of one ONT
	// written in the same second.
	err := s.db.Raw(`
		SELECT id, ont_id, event_type, event_time
		FROM (
			SELECT id, ont_id, event_type, event_time,
			       ROW_NUMBER() OVER (PARTITION BY ont_id ORDER BY event_time DESC, id DESC) AS rank
			FROM ont_events
			WHERE ont_id IN ?
		) ranked
		WHERE rank = 1`, ontIDs).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	latest := make(map[uuid.UUID]models.ONTEvent, len(rows))
	for _, row := range rows {
		latest[row.ONTID] = row
	}
	return latest, nil
}
