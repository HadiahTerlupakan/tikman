package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// maxTroubledWindow caps how far back the ranking will look.
//
// It matches the trap table's retention, past which there is nothing to count,
// and it bounds the cost: the aggregate reads every trap in the window, and the
// older chunks are compressed.
const maxTroubledWindow = 7 * 24 * time.Hour

// TroubledONT is one subscriber and how much trouble it has been in.
type TroubledONT struct {
	ONTID        uuid.UUID        `json:"ont_id"`
	SerialNumber string           `json:"serial_number"`
	Name         string           `json:"name"`
	OLTName      string           `json:"olt_name"`
	PortID       int              `json:"port_id"`
	ONTNumber    int              `json:"ont_number"`
	Status       models.ONTStatus `json:"status"`
	TrapCount    int64            `json:"trap_count"`
	DownMinutes  int64            `json:"down_minutes"`
}

// TroubledONTs ranks subscribers by how much they have been churning.
//
// The status column alone hides the worst faults: an ONU that drops and returns
// every few seconds reads "online" whenever anyone looks at it, while sending
// thousands of traps a day. Counting the traps is what makes such a subscriber
// visible, and the accumulated outage beside it says what the churn has cost.
//
// Every trap is counted, not only the alarms. Severity lives in the community
// string, which has only been kept since it was found to matter, so filtering on
// it would leave most of the window empty. Alarms and their clears arrive in
// pairs, so the total still measures the churn — it just counts each fault twice.
func (s *ONTService) TroubledONTs(window time.Duration, limit int) ([]TroubledONT, error) {
	if window > maxTroubledWindow {
		window = maxTroubledWindow
	}
	since := time.Now().Add(-window)

	var troubled []TroubledONT
	err := s.db.Raw(`
		WITH trap AS (
			SELECT olt_id, serial_number, count(*) AS trap_count
			FROM ont_trap_events
			WHERE received_at > ? AND serial_number IS NOT NULL
			GROUP BY olt_id, serial_number
		),
		outage AS (
			SELECT ont_id,
			       sum(COALESCE(duration_seconds, EXTRACT(EPOCH FROM (now() - event_time))))
			         FILTER (WHERE event_type = 'offline') AS down_seconds
			FROM ont_events
			WHERE event_time > ?
			GROUP BY ont_id
		)
		SELECT n.id AS ont_id, n.serial_number, n.name, o.name AS olt_name,
		       n.port_id, n.ont_id AS ont_number, n.status,
		       COALESCE(t.trap_count, 0) AS trap_count,
		       (COALESCE(g.down_seconds, 0) / 60)::bigint AS down_minutes
		FROM onts n
		JOIN olts o ON o.id = n.olt_id
		LEFT JOIN trap t ON t.olt_id = n.olt_id AND t.serial_number = n.serial_number
		LEFT JOIN outage g ON g.ont_id = n.id
		WHERE COALESCE(t.trap_count, 0) > 0 OR COALESCE(g.down_seconds, 0) > 0
		ORDER BY trap_count DESC, down_minutes DESC
		LIMIT ?
	`, since, since, limit).Scan(&troubled).Error

	return troubled, err
}
