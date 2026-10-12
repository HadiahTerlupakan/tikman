package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

// otherASN is netflow.OtherASN as stored.
const otherASN int64 = netflow.OtherASN

// otherASNName labels ASN 0: destinations outside the stored top, and those
// no dataset range covers.
const otherASNName = "Lainnya"

// FlowQueryService reads the summaries flowd stores. Its range queries use
// time_bucket, so they run on TimescaleDB only.
type FlowQueryService struct {
	db *gorm.DB
}

// NewFlowQueryService creates a flow query service.
func NewFlowQueryService(db *gorm.DB) *FlowQueryService {
	return &FlowQueryService{db: db}
}

// FlowFilter narrows a flow read. OtherPOP selects the traffic of addresses in
// no POP's blocks; with neither it nor POP set, every POP counts.
type FlowFilter struct {
	From, To  time.Time
	POP       *uuid.UUID
	OtherPOP  bool
	Upstream  *uuid.UUID
	Direction string
}

// where builds the WHERE clause every flow query shares. Column names are
// fixed here; every value is bound.
func (f FlowFilter) where(routerID uuid.UUID, timeColumn string) (string, []interface{}) {
	clauses := []string{"router_id = ?", timeColumn + " >= ?", timeColumn + " < ?"}
	args := []interface{}{routerID, f.From, f.To}
	switch {
	case f.OtherPOP:
		clauses = append(clauses, "pop_interface_id IS NULL")
	case f.POP != nil:
		clauses = append(clauses, "pop_interface_id = ?")
		args = append(args, *f.POP)
	}
	if f.Upstream != nil {
		clauses = append(clauses, "upstream_interface_id = ?")
		args = append(args, *f.Upstream)
	}
	if f.Direction != "" {
		clauses = append(clauses, "direction = ?")
		args = append(args, f.Direction)
	}
	return strings.Join(clauses, " AND "), args
}

// withArgs copies before appending, so one filter's arguments can feed
// several queries without one overwriting another's.
func withArgs(args []interface{}, extra ...interface{}) []interface{} {
	out := make([]interface{}, 0, len(args)+len(extra))
	return append(append(out, args...), extra...)
}

func bitsPerSecond(bytes int64, over time.Duration) float64 {
	return float64(bytes) * 8 / over.Seconds()
}

// FlowPoint is one step of a destination's traffic.
type FlowPoint struct {
	Time time.Time `json:"time"`
	Bps  float64   `json:"bps"`
}

// FlowSeries is one destination AS over the range.
type FlowSeries struct {
	ASN    int64       `json:"asn"`
	Name   string      `json:"name"`
	Points []FlowPoint `json:"points"`
}

// FlowDestinations is a stacked chart: the top destinations and Lainnya.
type FlowDestinations struct {
	StepSeconds int64        `json:"step_seconds"`
	Series      []FlowSeries `json:"series"`
}

type asnBucket struct {
	Time  time.Time
	ASN   int64 `gorm:"column:asn"`
	Bytes int64
}

// Destinations returns the limit largest destination ASNs of the range, each
// as bit/s per step, and everything else as one OtherASN series.
func (s *FlowQueryService) Destinations(routerID uuid.UUID, f FlowFilter, limit int) (*FlowDestinations, error) {
	res := resolveFlowRange(f.From, f.To, time.Now())
	where, args := f.where(routerID, res.source.timeColumn)
	top, err := s.topASNs(res.source.table, where, args, limit)
	if err != nil {
		return nil, err
	}
	rows, err := s.bucketedASNs(res, where, args, top)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames(top)
	if err != nil {
		return nil, err
	}
	return &FlowDestinations{StepSeconds: int64(res.step.Seconds()),
		Series: buildSeries(rows, top, names, res.step)}, nil
}

func (s *FlowQueryService) topASNs(table, where string, args []interface{}, limit int) ([]int64, error) {
	// table comes from flowSources, never from the request.
	query := fmt.Sprintf(`SELECT asn FROM %s WHERE %s AND asn <> 0
		GROUP BY asn ORDER BY SUM(bytes) DESC LIMIT ?`, table, where)
	var asns []int64
	return asns, s.db.Raw(query, withArgs(args, limit)...).Scan(&asns).Error
}

// bucketedASNs folds everything outside top into OtherASN in SQL, so the
// response carries limit+1 series however many ASNs the range holds.
func (s *FlowQueryService) bucketedASNs(res flowResolution, where string, args []interface{}, top []int64) ([]asnBucket, error) {
	asnExpr := "0"
	bucket, interval := res.bucketExpr()
	params := []interface{}{interval}
	if len(top) > 0 {
		asnExpr = "CASE WHEN asn IN ? THEN asn ELSE 0 END"
		params = append(params, top)
	}
	query := fmt.Sprintf(`SELECT %s AS time, %s AS asn, SUM(bytes)::bigint AS bytes
		FROM %s WHERE %s GROUP BY 1, 2 ORDER BY 1`, bucket, asnExpr, res.source.table, where)
	var rows []asnBucket
	return rows, s.db.Raw(query, withArgs(params, args...)...).Scan(&rows).Error
}

func buildSeries(rows []asnBucket, top []int64, names map[int64]string, step time.Duration) []FlowSeries {
	order := append(append([]int64{}, top...), otherASN)
	index := make(map[int64]int, len(order))
	series := make([]FlowSeries, len(order))
	for i, asn := range order {
		index[asn] = i
		series[i] = FlowSeries{ASN: asn, Name: asnLabel(asn, names), Points: []FlowPoint{}}
	}
	for _, r := range rows {
		i := index[r.ASN]
		series[i].Points = append(series[i].Points, FlowPoint{Time: r.Time, Bps: bitsPerSecond(r.Bytes, step)})
	}
	return series
}

// asnNames looks the given ASNs up in asn_names.
func (s *FlowQueryService) asnNames(asns []int64) (map[int64]string, error) {
	names := map[int64]string{}
	if len(asns) == 0 {
		return names, nil
	}
	var rows []models.ASNName
	if err := s.db.Where("asn IN ?", asns).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		names[r.ASN] = r.Name
	}
	return names, nil
}

// asnLabel is Lainnya for ASN 0 and the dataset's name otherwise, or "" for
// an AS the dataset does not name; the UI then shows the number.
func asnLabel(asn int64, names map[int64]string) string {
	if asn == otherASN {
		return otherASNName
	}
	return names[asn]
}
