package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The rules a PON has to break to be drawn. Both exist because neither sees the
// other's fault: a port whose subscribers lose a tenth of the day on one trap
// each, and a port churning at nine hundred traps per ONT while losing nothing.
const (
	ponOutageShareThreshold = 0.05
	ponTrapMedianMultiple   = 5
	ponTrapFloor            = 100
	ponMinONTs              = 5
	ponWorstSubscribers     = 5
)

// PonSubscriber is one subscriber on a troubled PON, named for a technician.
type PonSubscriber struct {
	ONTID       uuid.UUID `json:"ont_id"`
	Label       string    `json:"label"`
	Name        string    `json:"name"`
	TrapCount   int64     `json:"trap_count"`
	DownMinutes int64     `json:"down_minutes"`
}

// PonNode is one PON port that broke a rule, with the subscribers behind it.
type PonNode struct {
	Port        int             `json:"port"`
	ONTCount    int64           `json:"ont_count"`
	TrapPerONT  int64           `json:"trap_per_ont"`
	OutageShare float64         `json:"outage_share"`
	Worst       []PonSubscriber `json:"worst"`
}

// CardNode groups the troubled PON ports on one card.
type CardNode struct {
	Slot     int       `json:"slot"`
	PonCount int       `json:"pon_count"`
	Pons     []PonNode `json:"pons"`
}

// PonHealth is the fault tree for one OLT, pruned to the troubled branches.
type PonHealth struct {
	OLTID            uuid.UUID  `json:"olt_id"`
	OLTName          string     `json:"olt_name"`
	MedianTrapPerONT int64      `json:"median_trap_per_ont"`
	TrapThreshold    int64      `json:"trap_threshold"`
	OutageThreshold  float64    `json:"outage_threshold"`
	Cards            []CardNode `json:"cards"`
}

type ponRow struct {
	Slot        int
	Port        int
	ONTCount    int64
	TrapPerONT  int64
	OutageShare float64
	Median      float64
}

// ponHealthQuery finds every port that broke either rule, and rides the
// network's own median along so the empty case can still report it.
const ponHealthQuery = `
	WITH trap AS (
		SELECT serial_number, count(*) AS c
		FROM ont_trap_events
		WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
		GROUP BY serial_number
	),
	outage AS (
		SELECT ont_id,
		       sum(COALESCE(duration_seconds, EXTRACT(EPOCH FROM (now() - event_time))))
		         FILTER (WHERE event_type = 'offline') AS s
		FROM ont_events WHERE event_time > ? GROUP BY ont_id
	),
	pon AS (
		SELECT COALESCE(n.slot, 0) AS slot, n.port_id AS port,
		       count(*) AS ont_count,
		       (sum(COALESCE(t.c, 0))::numeric / count(*)) AS trap_per_ont,
		       (sum(COALESCE(g.s, 0)) / 60 / count(*) / ?) AS outage_share
		FROM onts n
		LEFT JOIN trap t ON t.serial_number = n.serial_number
		LEFT JOIN outage g ON g.ont_id = n.id
		WHERE n.olt_id = ?
		GROUP BY COALESCE(n.slot, 0), n.port_id
		HAVING count(*) >= ?
	),
	reference AS (
		SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY trap_per_ont) AS median FROM pon
	)
	SELECT p.slot, p.port, p.ont_count,
	       round(p.trap_per_ont)::bigint AS trap_per_ont,
	       p.outage_share, r.median
	FROM pon p CROSS JOIN reference r
	WHERE p.outage_share > ?
	   OR (p.trap_per_ont > r.median * ? AND p.trap_per_ont > ?)
	ORDER BY p.slot, p.port
`

// PonHealthFor ranks the PON ports on one OLT by the two ways they fail, and
// returns the fault tree pruned to the ports that broke a rule.
//
// The window is clamped here as well as at the handler, because the cost this
// query carries belongs to the query, not to whoever remembers to bound it.
func (s *ONTService) PonHealthFor(oltID uuid.UUID, window time.Duration) (PonHealth, error) {
	if window > maxTroubledWindow {
		window = maxTroubledWindow
	}
	since := time.Now().Add(-window)

	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return PonHealth{}, err
	}

	var rows []ponRow
	err := s.db.Raw(ponHealthQuery, oltID, since, since, window.Minutes(), oltID, ponMinONTs,
		ponOutageShareThreshold, ponTrapMedianMultiple, ponTrapFloor).Scan(&rows).Error
	if err != nil {
		return PonHealth{}, err
	}

	health := PonHealth{
		OLTID: olt.ID, OLTName: olt.Name,
		TrapThreshold:   ponTrapFloor,
		OutageThreshold: ponOutageShareThreshold,
	}
	if len(rows) > 0 {
		health.MedianTrapPerONT = int64(rows[0].Median)
	} else if health.MedianTrapPerONT, err = s.ponMedian(oltID, since); err != nil {
		// A silent zero here would put a median on screen beside a rule the
		// server never applied, which is worse than saying nothing.
		return PonHealth{}, err
	}

	worst, err := s.worstByPon(oltID, since)
	if err != nil {
		return PonHealth{}, err
	}
	for _, row := range rows {
		node := PonNode{
			Port: row.Port, ONTCount: row.ONTCount, TrapPerONT: row.TrapPerONT,
			OutageShare: row.OutageShare, Worst: worst[ponKey{Slot: row.Slot, Port: row.Port}],
		}
		health.Cards = appendToCard(health.Cards, row.Slot, node)
	}
	return health, nil
}

// appendToCard groups a PON under its card, keeping the card order the query
// already sorted by.
func appendToCard(cards []CardNode, slot int, node PonNode) []CardNode {
	for i := range cards {
		if cards[i].Slot == slot {
			cards[i].Pons = append(cards[i].Pons, node)
			cards[i].PonCount = len(cards[i].Pons)
			return cards
		}
	}
	return append(cards, CardNode{Slot: slot, PonCount: 1, Pons: []PonNode{node}})
}

// ponMedian answers what a normal port looks like on this OLT when no port
// broke a rule, so the screen can still state the reference it judged against.
func (s *ONTService) ponMedian(oltID uuid.UUID, since time.Time) (int64, error) {
	var median float64
	err := s.db.Raw(`
		WITH trap AS (
			SELECT serial_number, count(*) AS c FROM ont_trap_events
			WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
			GROUP BY serial_number
		),
		pon AS (
			SELECT (sum(COALESCE(t.c, 0))::numeric / count(*)) AS trap_per_ont
			FROM onts n LEFT JOIN trap t ON t.serial_number = n.serial_number
			WHERE n.olt_id = ? GROUP BY COALESCE(n.slot, 0), n.port_id
			HAVING count(*) >= ?
		)
		SELECT COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY trap_per_ont), 0) FROM pon
	`, oltID, since, oltID, ponMinONTs).Scan(&median).Error
	return int64(median), err
}

// ponKey addresses a port by the card it sits on as well as its number, because
// port numbers repeat across cards on the same chassis.
type ponKey struct {
	Slot int
	Port int
}

// worstOnPonQuery ranks every port's subscribers on one OLT in a single pass.
//
// Per-port queries meant re-reading the whole window once per flagged port, and
// a chassis with thirty of them paid that thirty-one times a minute while the
// topology tab was open. The window function does the same narrowing inside one
// scan; the ports that broke no rule are dropped by the caller.
//
// The label names the card, not just the port: ONU-8:12 on card 8 and on card 9
// are different subscribers reading as the same line.
const worstOnPonQuery = `
	WITH trap AS (
		SELECT serial_number, count(*) AS c FROM ont_trap_events
		WHERE olt_id = ? AND received_at > ? AND serial_number IS NOT NULL
		GROUP BY serial_number
	),
	outage AS (
		SELECT e.ont_id,
		       sum(COALESCE(e.duration_seconds, EXTRACT(EPOCH FROM (now() - e.event_time))))
		         FILTER (WHERE e.event_type = 'offline') AS s
		FROM ont_events e JOIN onts n ON n.id = e.ont_id
		WHERE n.olt_id = ? AND e.event_time > ?
		GROUP BY e.ont_id
	),
	ranked AS (
		SELECT COALESCE(n.slot, 0) AS slot, n.port_id AS port, n.id AS ont_id,
		       'ONU-' || COALESCE(n.slot, 0) || '/' || n.port_id || ':' || n.ont_id AS label,
		       n.name,
		       COALESCE(t.c, 0) AS trap_count,
		       (COALESCE(g.s, 0) / 60)::bigint AS down_minutes,
		       row_number() OVER (
		           PARTITION BY COALESCE(n.slot, 0), n.port_id
		           ORDER BY COALESCE(t.c, 0) DESC, (COALESCE(g.s, 0) / 60)::bigint DESC
		       ) AS rn
		FROM onts n
		LEFT JOIN trap t ON t.serial_number = n.serial_number
		LEFT JOIN outage g ON g.ont_id = n.id
		WHERE n.olt_id = ?
	)
	SELECT slot, port, ont_id, label, name, trap_count, down_minutes
	FROM ranked WHERE rn <= ?
	ORDER BY slot, port, rn
`

// worstByPon names the subscribers a technician would look at first, for every
// port on the OLT at once, keyed by card and port.
func (s *ONTService) worstByPon(oltID uuid.UUID, since time.Time) (map[ponKey][]PonSubscriber, error) {
	var rows []struct {
		PonSubscriber
		Slot int
		Port int
	}
	err := s.db.Raw(worstOnPonQuery, oltID, since, oltID, since, oltID, ponWorstSubscribers).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	worst := make(map[ponKey][]PonSubscriber)
	for _, row := range rows {
		key := ponKey{Slot: row.Slot, Port: row.Port}
		worst[key] = append(worst[key], row.PonSubscriber)
	}
	return worst, nil
}
