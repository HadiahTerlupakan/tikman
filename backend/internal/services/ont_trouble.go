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
	ONTID        uuid.UUID `json:"ont_id"`
	SerialNumber string    `json:"serial_number"`
	Name         string    `json:"name"`
	OLTName      string    `json:"olt_name"`
	// Slot is the line card. Discovery leaves it NULL on rows it could not
	// place, and those are carried as card 0 — the same card the PON topology
	// already groups them under — so narrowing to one PON matches them exactly
	// there and nowhere else, rather than on every card.
	Slot        int              `json:"slot"`
	PortID      int              `json:"port_id"`
	ONTNumber   int              `json:"ont_number"`
	Status      models.ONTStatus `json:"status"`
	TrapCount   int64            `json:"trap_count"`
	DownMinutes int64            `json:"down_minutes"`
}

// TroubledSummary is the whole picture the ranking is a page of.
//
// Counted over every ONT that matched, not the page returned: with five hundred
// subscribers churning on one chassis, a total drawn from the fifty shown would
// tell an operator a tenth of the truth.
type TroubledSummary struct {
	ONTCount         int64 `json:"ont_count"`
	TotalDownMinutes int64 `json:"total_down_minutes"`
}

// TroubledFilter is what a request asks the ranking for.
//
// A struct rather than a parameter list because the list had reached three and
// was about to reach four, and ONTListFilter in this same service already
// settled how this codebase carries query options.
type TroubledFilter struct {
	Window time.Duration
	Limit  int
	OLTID  *uuid.UUID
	Status *models.ONTStatus
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
func (s *ONTService) TroubledONTs(filter TroubledFilter) ([]TroubledONT, TroubledSummary, error) {
	window := filter.Window
	if window > maxTroubledWindow {
		window = maxTroubledWindow
	}
	since := time.Now().Add(-window)

	// The totals ride along as window functions: those see every matching row,
	// while LIMIT applies afterwards, so one pass answers both the page and the
	// picture it is a page of.
	// Named without adjacent capitals on purpose: GORM's naming strategy splits
	// TotalONTs into total_on_ts, which is the same trap that cost this codebase
	// three migrations over trap_o_id.
	var rows []struct {
		TroubledONT
		TotalRows        int64
		TotalDownSeconds int64
	}

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
		       COALESCE(n.slot, 0) AS slot, n.port_id, n.ont_id AS ont_number, n.status,
		       COALESCE(t.trap_count, 0) AS trap_count,
		       (COALESCE(g.down_seconds, 0) / 60)::bigint AS down_minutes,
		       count(*) OVER () AS total_rows,
		       (sum(COALESCE(g.down_seconds, 0)) OVER ())::bigint AS total_down_seconds
		FROM onts n
		JOIN olts o ON o.id = n.olt_id
		LEFT JOIN trap t ON t.olt_id = n.olt_id AND t.serial_number = n.serial_number
		LEFT JOIN outage g ON g.ont_id = n.id
		WHERE (COALESCE(t.trap_count, 0) > 0 OR COALESCE(g.down_seconds, 0) > 0)
		  AND (?::uuid IS NULL OR n.olt_id = ?::uuid)
		  AND (?::text IS NULL OR n.status = ?::text)
		ORDER BY trap_count DESC, down_minutes DESC
		LIMIT ?
	`, since, since, filter.OLTID, filter.OLTID, filter.Status, filter.Status, filter.Limit).Scan(&rows).Error
	if err != nil {
		return nil, TroubledSummary{}, err
	}

	troubled := make([]TroubledONT, 0, len(rows))
	for _, row := range rows {
		troubled = append(troubled, row.TroubledONT)
	}

	var summary TroubledSummary
	if len(rows) > 0 {
		summary = TroubledSummary{
			ONTCount:         rows[0].TotalRows,
			TotalDownMinutes: rows[0].TotalDownSeconds / 60,
		}
	}
	return troubled, summary, nil
}
