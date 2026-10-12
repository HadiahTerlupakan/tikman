package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FlowPeeringRow is one upstream's part of one destination over the range.
type FlowPeeringRow struct {
	UpstreamInterfaceID uuid.UUID `json:"upstream_interface_id"`
	Bytes               int64     `json:"bytes"`
	Percent             float64   `json:"percent"`
	AvgBps              float64   `json:"avg_bps"`
	PeakBps             float64   `json:"peak_bps"`
}

// FlowPeeringSeries is one upstream's traffic to the destination over time.
type FlowPeeringSeries struct {
	UpstreamInterfaceID uuid.UUID   `json:"upstream_interface_id"`
	Points              []FlowPoint `json:"points"`
}

// FlowPeering shows which upstreams carry one destination, which is what
// tells whether a peering actually takes that destination off transit.
type FlowPeering struct {
	ASN               int64               `json:"asn"`
	Name              string              `json:"name"`
	TotalBytes        int64               `json:"total_bytes"`
	StepSeconds       int64               `json:"step_seconds"`
	PeakWindowSeconds int64               `json:"peak_window_seconds"`
	Rows              []FlowPeeringRow    `json:"rows"`
	Series            []FlowPeeringSeries `json:"series"`
}

type upstreamTotal struct {
	UpstreamInterfaceID uuid.UUID `gorm:"column:upstream_interface_id"`
	Bytes               int64
	Peak                int64
}

type upstreamBucket struct {
	Time                time.Time
	UpstreamInterfaceID uuid.UUID `gorm:"column:upstream_interface_id"`
	Bytes               int64
}

// Peering splits one destination ASN's traffic over the upstreams it used,
// largest first, as totals for the range and as bit/s per step.
func (s *FlowQueryService) Peering(routerID uuid.UUID, f FlowFilter, asn int64) (*FlowPeering, error) {
	// The split is across upstreams: an upstream picked in the tab would
	// narrow it to one row at 100% and say nothing about the peering.
	f.Upstream = nil
	res := resolveFlowRange(f.From, f.To, time.Now())
	where, args := f.where(routerID, res.source.timeColumn)
	where += " AND asn = ?"
	args = withArgs(args, asn)
	totals, err := s.upstreamTotals(res.source, where, args)
	if err != nil {
		return nil, err
	}
	buckets, err := s.upstreamBuckets(res, where, args)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames([]int64{asn})
	if err != nil {
		return nil, err
	}
	out := &FlowPeering{ASN: asn, Name: asnLabel(asn, names), StepSeconds: int64(res.step.Seconds()),
		PeakWindowSeconds: int64(res.source.step.Seconds())}
	for _, t := range totals {
		out.TotalBytes += t.Bytes
	}
	out.Rows = peeringRows(totals, out.TotalBytes, f.To.Sub(f.From), res.source.step)
	out.Series = peeringSeries(totals, buckets, res.step)
	return out, nil
}

// upstreamTotals ranks upstreams by bytes. Peak is the busiest single source
// step on that upstream.
func (s *FlowQueryService) upstreamTotals(src flowSource, where string, args []interface{}) ([]upstreamTotal, error) {
	// table and column come from flowSources, never from the request.
	query := fmt.Sprintf(`SELECT upstream_interface_id, SUM(step_bytes)::bigint AS bytes, MAX(step_bytes)::bigint AS peak
		FROM (SELECT %s AS step, upstream_interface_id, SUM(bytes) AS step_bytes FROM %s WHERE %s GROUP BY 1, 2) per_step
		GROUP BY upstream_interface_id ORDER BY bytes DESC, upstream_interface_id`, src.timeColumn, src.table, where)
	var rows []upstreamTotal
	return rows, s.db.Raw(query, args...).Scan(&rows).Error
}

func (s *FlowQueryService) upstreamBuckets(res flowResolution, where string, args []interface{}) ([]upstreamBucket, error) {
	bucket, interval := res.bucketExpr()
	query := fmt.Sprintf(`SELECT %s AS time, upstream_interface_id, SUM(bytes)::bigint AS bytes
		FROM %s WHERE %s GROUP BY 1, 2 ORDER BY 1`, bucket, res.source.table, where)
	var rows []upstreamBucket
	return rows, s.db.Raw(query, withArgs([]interface{}{interval}, args...)...).Scan(&rows).Error
}

func peeringRows(totals []upstreamTotal, total int64, span, step time.Duration) []FlowPeeringRow {
	rows := make([]FlowPeeringRow, 0, len(totals))
	for _, t := range totals {
		rows = append(rows, FlowPeeringRow{UpstreamInterfaceID: t.UpstreamInterfaceID, Bytes: t.Bytes,
			Percent: percentOf(t.Bytes, total), AvgBps: bitsPerSecond(t.Bytes, span),
			PeakBps: bitsPerSecond(t.Peak, step)})
	}
	return rows
}

// peeringSeries keeps the rows' order, so the chart stacks the largest
// upstream first.
func peeringSeries(totals []upstreamTotal, buckets []upstreamBucket, step time.Duration) []FlowPeeringSeries {
	index := make(map[uuid.UUID]int, len(totals))
	series := make([]FlowPeeringSeries, len(totals))
	for i, t := range totals {
		index[t.UpstreamInterfaceID] = i
		series[i] = FlowPeeringSeries{UpstreamInterfaceID: t.UpstreamInterfaceID, Points: []FlowPoint{}}
	}
	for _, b := range buckets {
		// The two queries share no snapshot: an upstream first seen between
		// them has no row, and must not be folded into the largest series.
		i, ok := index[b.UpstreamInterfaceID]
		if !ok {
			continue
		}
		series[i].Points = append(series[i].Points, FlowPoint{Time: b.Time, Bps: bitsPerSecond(b.Bytes, step)})
	}
	return series
}
