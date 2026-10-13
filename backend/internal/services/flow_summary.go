package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// summaryLimit is how many destinations the summary table lists.
	summaryLimit = 50
	// popTopASNs is how many destinations the POP table names per POP.
	popTopASNs = 3
)

// FlowUpstreamShare is one upstream's part of a destination's bytes.
type FlowUpstreamShare struct {
	UpstreamInterfaceID uuid.UUID `json:"upstream_interface_id" gorm:"column:upstream_interface_id"`
	Bytes               int64     `json:"bytes"`
}

// FlowSummaryRow is one destination over the whole range.
type FlowSummaryRow struct {
	ASN       int64               `json:"asn"`
	Name      string              `json:"name"`
	Bytes     int64               `json:"bytes"`
	Percent   float64             `json:"percent"`
	AvgBps    float64             `json:"avg_bps"`
	PeakBps   float64             `json:"peak_bps"`
	Upstreams []FlowUpstreamShare `json:"upstreams"`
}

// FlowSummary is the destination table. PeakWindowSeconds is the step the
// peak was measured over.
type FlowSummary struct {
	TotalBytes        int64            `json:"total_bytes"`
	PeakWindowSeconds int64            `json:"peak_window_seconds"`
	Rows              []FlowSummaryRow `json:"rows"`
}

type asnTotal struct {
	ASN   int64 `gorm:"column:asn"`
	Bytes int64
	Peak  int64
}

// Summary ranks the range's destinations with their share, average, peak and
// split over upstreams.
func (s *FlowQueryService) Summary(routerID uuid.UUID, f FlowFilter) (*FlowSummary, error) {
	src := resolveFlowRange(f.From, f.To, time.Now()).source
	where, args := f.where(routerID, src.timeColumn)
	total, err := s.totalBytes(src.table, where, args)
	if err != nil {
		return nil, err
	}
	totals, err := s.asnTotals(src, where, args)
	if err != nil {
		return nil, err
	}
	asns := make([]int64, len(totals))
	for i, t := range totals {
		asns[i] = t.ASN
	}
	shares, err := s.upstreamShares(src.table, where, args, asns)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	return &FlowSummary{TotalBytes: total, PeakWindowSeconds: int64(src.step.Seconds()),
		Rows: summaryRows(totals, shares, names, total, f.To.Sub(f.From), src.step)}, nil
}

func summaryRows(totals []asnTotal, shares map[int64][]FlowUpstreamShare, names map[int64]string,
	total int64, span, step time.Duration) []FlowSummaryRow {
	rows := make([]FlowSummaryRow, 0, len(totals))
	for _, t := range totals {
		upstreams := shares[t.ASN]
		if upstreams == nil {
			upstreams = []FlowUpstreamShare{}
		}
		rows = append(rows, FlowSummaryRow{ASN: t.ASN, Name: asnLabel(t.ASN, names), Bytes: t.Bytes,
			Percent: percentOf(t.Bytes, total), AvgBps: bitsPerSecond(t.Bytes, span),
			PeakBps: bitsPerSecond(t.Peak, step), Upstreams: upstreams})
	}
	return rows
}

func percentOf(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) * 100 / float64(total)
}

func (s *FlowQueryService) totalBytes(table, where string, args []interface{}) (int64, error) {
	var total int64
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE %s`, cappedBigint("COALESCE(SUM(bytes), 0)"), table, where)
	return total, s.db.Raw(query, args...).Scan(&total).Error
}

// asnTotals ranks destinations by bytes. Peak is the busiest single source
// step, summed over the upstreams and POPs the filter keeps.
func (s *FlowQueryService) asnTotals(src flowSource, where string, args []interface{}) ([]asnTotal, error) {
	query := fmt.Sprintf(`SELECT asn, %s AS bytes, %s AS peak
		FROM (SELECT %s AS step, asn, SUM(bytes) AS step_bytes FROM %s WHERE %s GROUP BY 1, 2) per_step
		GROUP BY asn ORDER BY bytes DESC LIMIT ?`,
		cappedBigint("SUM(step_bytes)"), cappedBigint("MAX(step_bytes)"), src.timeColumn, src.table, where)
	var rows []asnTotal
	return rows, s.db.Raw(query, withArgs(args, summaryLimit)...).Scan(&rows).Error
}

func (s *FlowQueryService) upstreamShares(table, where string, args []interface{}, asns []int64) (map[int64][]FlowUpstreamShare, error) {
	out := map[int64][]FlowUpstreamShare{}
	if len(asns) == 0 {
		return out, nil
	}
	var rows []struct {
		ASN int64 `gorm:"column:asn"`
		FlowUpstreamShare
	}
	query := fmt.Sprintf(`SELECT asn, upstream_interface_id, %s AS bytes FROM %s
		WHERE %s AND asn IN ? GROUP BY 1, 2 ORDER BY 3 DESC`, cappedBigint("SUM(bytes)"), table, where)
	if err := s.db.Raw(query, withArgs(args, asns)...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ASN] = append(out[r.ASN], r.FlowUpstreamShare)
	}
	return out, nil
}

// FlowASNBytes is one destination's bytes within a POP.
type FlowASNBytes struct {
	ASN   int64  `json:"asn"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// FlowPOPRow is one POP's total and its largest destinations. A nil POP is
// the traffic of addresses in no POP's blocks.
type FlowPOPRow struct {
	POPInterfaceID *uuid.UUID     `json:"pop_interface_id"`
	Bytes          int64          `json:"bytes"`
	TopASNs        []FlowASNBytes `json:"top_asns"`
}

type popASN struct {
	POPInterfaceID *uuid.UUID `gorm:"column:pop_interface_id"`
	ASN            int64      `gorm:"column:asn"`
	Bytes          int64
}

// POPs lists each POP's total, largest first, with its popTopASNs largest
// destinations.
func (s *FlowQueryService) POPs(routerID uuid.UUID, f FlowFilter) ([]FlowPOPRow, error) {
	src := resolveFlowRange(f.From, f.To, time.Now()).source
	where, args := f.where(routerID, src.timeColumn)
	var totals []popASN
	query := fmt.Sprintf(`SELECT pop_interface_id, %s AS bytes FROM %s WHERE %s
		GROUP BY 1 ORDER BY 2 DESC`, cappedBigint("SUM(bytes)"), src.table, where)
	if err := s.db.Raw(query, args...).Scan(&totals).Error; err != nil {
		return nil, err
	}
	tops, err := s.popTopASNs(src.table, where, args)
	if err != nil {
		return nil, err
	}
	asns := make([]int64, len(tops))
	for i, t := range tops {
		asns[i] = t.ASN
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	return buildPOPRows(totals, tops, names), nil
}

func (s *FlowQueryService) popTopASNs(table, where string, args []interface{}) ([]popASN, error) {
	query := fmt.Sprintf(`SELECT pop_interface_id, asn, bytes FROM (
			SELECT pop_interface_id, asn, %s AS bytes,
				ROW_NUMBER() OVER (PARTITION BY pop_interface_id ORDER BY SUM(bytes) DESC, asn) AS position
			FROM %s WHERE %s AND asn <> 0 GROUP BY 1, 2) ranked
		WHERE position <= ? ORDER BY bytes DESC`, cappedBigint("SUM(bytes)"), table, where)
	var rows []popASN
	return rows, s.db.Raw(query, withArgs(args, popTopASNs)...).Scan(&rows).Error
}

func popOf(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func buildPOPRows(totals, tops []popASN, names map[int64]string) []FlowPOPRow {
	byPOP := map[uuid.UUID][]FlowASNBytes{}
	for _, t := range tops {
		key := popOf(t.POPInterfaceID)
		byPOP[key] = append(byPOP[key], FlowASNBytes{ASN: t.ASN, Name: asnLabel(t.ASN, names), Bytes: t.Bytes})
	}
	rows := make([]FlowPOPRow, 0, len(totals))
	for _, t := range totals {
		top := byPOP[popOf(t.POPInterfaceID)]
		if top == nil {
			top = []FlowASNBytes{}
		}
		rows = append(rows, FlowPOPRow{POPInterfaceID: t.POPInterfaceID, Bytes: t.Bytes, TopASNs: top})
	}
	return rows
}
