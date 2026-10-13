package services

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func seedFlowMinute(t *testing.T, db *gorm.DB, row models.RouterFlowMinute) {
	t.Helper()
	if row.Packets == 0 {
		row.Packets = 1
	}
	require.NoError(t, db.Create(&row).Error)
}

// bytesIn adds up every series of a destinations read back into bytes.
func bytesIn(t *testing.T, flows *FlowQueryService, routerID uuid.UUID, f FlowFilter) float64 {
	t.Helper()
	out, err := flows.Destinations(routerID, f, 10)
	require.NoError(t, err)
	var total float64
	for _, series := range out.Series {
		for _, p := range series.Points {
			total += p.Bps * float64(out.StepSeconds) / 8
		}
	}
	return total
}

func TestDestinationsStackTheTopASNsAndFoldTheRest(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit := uuid.New(), uuid.New()
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-10 * time.Minute)
	for i := 0; i < 3; i++ {
		at := start.Add(time.Duration(i) * time.Minute)
		for asn, bytes := range map[int64]int64{64496: 6_000_000, 64497: 3_000_000, 64498: 600} {
			seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
				UpstreamInterfaceID: transit, Direction: "download", ASN: asn, Bytes: bytes})
		}
	}
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: end}).Error)

	out, err := flows.Destinations(routerID, FlowFilter{From: start, To: end, Direction: "download"}, 2)

	require.NoError(t, err)
	assert.EqualValues(t, 60, out.StepSeconds)
	require.Len(t, out.Series, 3)
	assert.Equal(t, []int64{64496, 64497, otherASN}, []int64{out.Series[0].ASN, out.Series[1].ASN, out.Series[2].ASN})
	assert.Equal(t, "EXAMPLE-NET-A", out.Series[0].Name)
	assert.Empty(t, out.Series[1].Name, "an AS the dataset does not name")
	assert.Equal(t, otherASNName, out.Series[2].Name)
	require.Len(t, out.Series[0].Points, 3)
	assert.InDelta(t, 6_000_000*8/60.0, out.Series[0].Points[0].Bps, 0.001)
	assert.InDelta(t, 600*8/60.0, out.Series[2].Points[0].Bps, 0.001)
}

func TestDestinationsFilterByPOPOtherPOPAndUpstream(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, pop, transit, ix := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID, POPInterfaceID: &pop,
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
		UpstreamInterfaceID: ix, Direction: "download", ASN: 64496, Bytes: 50})
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: uuid.New(),
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 7})

	window := FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute), Direction: "download"}
	byPOP, other, viaIX := window, window, window
	byPOP.POP = &pop
	other.OtherPOP = true
	viaIX.Upstream = &ix

	assert.InDelta(t, 1050, bytesIn(t, flows, routerID, window), 0.001, "another router's bytes stay out")
	assert.InDelta(t, 1000, bytesIn(t, flows, routerID, byPOP), 0.001)
	assert.InDelta(t, 50, bytesIn(t, flows, routerID, other), 0.001)
	assert.InDelta(t, 50, bytesIn(t, flows, routerID, viaIX), 0.001)
}

func TestDestinationsReadTheFiveMinuteRollupForAMonthOldRange(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit := uuid.New(), uuid.New()
	old := time.Now().UTC().Add(-30 * 24 * time.Hour).Truncate(5 * time.Minute)
	for i := 0; i < 5; i++ {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: old.Add(time.Duration(i) * time.Minute), RouterID: routerID,
			UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})
	}
	// Materialized at once: these raw rows are past their 7-day retention,
	// and the policy job may drop them at any moment.
	require.NoError(t, db.Exec(fmt.Sprintf("CALL refresh_continuous_aggregate('router_flow_5min', '%s', '%s')",
		old.Add(-time.Hour).Format(time.RFC3339), old.Add(time.Hour).Format(time.RFC3339))).Error)

	out, err := flows.Destinations(routerID, FlowFilter{From: old.Add(-24 * time.Hour), To: old.Add(24 * time.Hour)}, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 300, out.StepSeconds)
	require.Len(t, out.Series[0].Points, 1)
	assert.True(t, old.Equal(out.Series[0].Points[0].Time))
	assert.InDelta(t, 5000*8/300.0, out.Series[0].Points[0].Bps, 0.001)
}

func TestSummaryGivesShareAverageAndPeak(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit, ix := uuid.New(), uuid.New(), uuid.New()
	m0 := time.Now().UTC().Truncate(time.Minute).Add(-8 * time.Minute)
	seeds := []models.RouterFlowMinute{
		{Time: m0, UpstreamInterfaceID: transit, ASN: 64496, Bytes: 6000},
		{Time: m0, UpstreamInterfaceID: ix, ASN: 64496, Bytes: 3000},
		{Time: m0.Add(time.Minute), UpstreamInterfaceID: transit, ASN: 64496, Bytes: 1000},
		{Time: m0, UpstreamInterfaceID: transit, ASN: 64497, Bytes: 2000},
	}
	for _, s := range seeds {
		s.RouterID, s.Direction = routerID, "download"
		seedFlowMinute(t, db, s)
	}

	out, err := flows.Summary(routerID, FlowFilter{From: m0, To: m0.Add(10 * time.Minute), Direction: "download"})

	require.NoError(t, err)
	assert.EqualValues(t, 12000, out.TotalBytes)
	assert.EqualValues(t, 60, out.PeakWindowSeconds)
	require.Len(t, out.Rows, 2)
	top := out.Rows[0]
	assert.EqualValues(t, 64496, top.ASN)
	assert.EqualValues(t, 10000, top.Bytes)
	assert.InDelta(t, 83.333, top.Percent, 0.001)
	assert.InDelta(t, 10000*8/600.0, top.AvgBps, 0.001)
	assert.InDelta(t, 9000*8/60.0, top.PeakBps, 0.001, "the busiest minute, both upstreams together")
	assert.Equal(t, []FlowUpstreamShare{{UpstreamInterfaceID: transit, Bytes: 7000},
		{UpstreamInterfaceID: ix, Bytes: 3000}}, top.Upstreams)
}

func TestPOPsListEachPOPsTotalAndTopThree(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, pop, transit := uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-3 * time.Minute)
	for asn, bytes := range map[int64]int64{64496: 400, 64497: 300, 64498: 200, 64499: 100} {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID, POPInterfaceID: &pop,
			UpstreamInterfaceID: transit, Direction: "download", ASN: asn, Bytes: bytes})
	}
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 50})

	rows, err := flows.POPs(routerID, FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute), Direction: "download"})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NotNil(t, rows[0].POPInterfaceID)
	assert.Equal(t, pop, *rows[0].POPInterfaceID)
	assert.EqualValues(t, 1000, rows[0].Bytes)
	var asns []int64
	for _, a := range rows[0].TopASNs {
		asns = append(asns, a.ASN)
	}
	assert.Equal(t, []int64{64496, 64497, 64498}, asns)
	assert.Nil(t, rows[1].POPInterfaceID, "addresses in no POP's blocks")
	assert.EqualValues(t, 50, rows[1].Bytes)
}

func TestTopIPsSumAnAddressOverItsWindows(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID := uuid.New()
	w0 := time.Now().UTC().Truncate(5 * time.Minute).Add(-10 * time.Minute)
	for _, r := range []models.RouterFlowTopIP{
		{Time: w0, IP: "203.0.113.5", ASN: 64498, Bytes: 100},
		{Time: w0.Add(5 * time.Minute), IP: "203.0.113.5", ASN: 64498, Bytes: 50},
		{Time: w0, IP: "203.0.113.6", ASN: 64498, Bytes: 120},
	} {
		r.RouterID, r.Direction = routerID, "download"
		require.NoError(t, db.Create(&r).Error)
	}
	require.NoError(t, db.Create(&models.ASNName{ASN: 64498, Name: "EXAMPLE-NET-C", Country: "ZZ", UpdatedAt: w0}).Error)

	rows, err := flows.TopIPs(routerID, FlowFilter{From: w0, To: w0.Add(15 * time.Minute)}, time.Now())

	require.NoError(t, err)
	assert.Equal(t, []FlowTopIP{
		{IP: "203.0.113.5", ASN: 64498, Name: "EXAMPLE-NET-C", Country: "ZZ", Bytes: 150},
		{IP: "203.0.113.6", ASN: 64498, Name: "EXAMPLE-NET-C", Country: "ZZ", Bytes: 120},
	}, rows)
}

// The POP and upstream filters take UUIDs from the query string, so one that
// belongs to another router must answer with nothing, not that router's rows.
func TestDestinationsFilteredByAnotherRoutersPOPIsEmpty(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	mine, theirs, theirPOP, transit := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: theirs, POPInterfaceID: &theirPOP,
		UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})

	f := FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute), POP: &theirPOP}
	assert.Zero(t, bytesIn(t, flows, mine, f))
	f.POP, f.Upstream = nil, &transit
	assert.Zero(t, bytesIn(t, flows, mine, f))
}

// Each stored row is at most netflow.MaxCount, but a sender spoofing a router
// can store many: every read holds a sum past bigint at its maximum rather
// than failing with "bigint out of range", and Go-side totals do not wrap.
func TestFlowReadsHoldSumsPastBigintAtItsMaximum(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit, ix := uuid.New(), uuid.New(), uuid.New()
	m0 := time.Now().UTC().Truncate(5 * time.Minute).Add(-10 * time.Minute)
	const half = int64(1) << 62 // two of them are 2^63, one past bigint
	for _, up := range []uuid.UUID{transit, ix} {
		for _, at := range []time.Time{m0, m0.Add(time.Minute)} {
			seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
				UpstreamInterfaceID: up, Direction: "download", ASN: 64496, Bytes: half})
		}
	}
	for _, at := range []time.Time{m0, m0.Add(5 * time.Minute)} {
		require.NoError(t, db.Create(&models.RouterFlowTopIP{Time: at, RouterID: routerID, Direction: "download",
			IP: "203.0.113.5", ASN: 64496, Bytes: half}).Error)
	}
	f := FlowFilter{From: m0, To: m0.Add(10 * time.Minute), Direction: "download"}
	const most = int64(math.MaxInt64)

	summary, err := flows.Summary(routerID, f)
	require.NoError(t, err)
	assert.Equal(t, most, summary.TotalBytes)
	require.Len(t, summary.Rows, 1)
	assert.Equal(t, most, summary.Rows[0].Bytes)
	assert.Equal(t, bitsPerSecond(most, time.Minute), summary.Rows[0].PeakBps)

	pops, err := flows.POPs(routerID, f)
	require.NoError(t, err)
	require.Len(t, pops, 1)
	assert.Equal(t, most, pops[0].Bytes)

	peering, err := flows.Peering(routerID, f, 64496)
	require.NoError(t, err)
	assert.Equal(t, most, peering.TotalBytes, "two upstreams at bigint's maximum")
	require.Len(t, peering.POPs, 1)
	assert.Equal(t, most, peering.POPs[0].Bytes)

	destinations, err := flows.Destinations(routerID, f, 10)
	require.NoError(t, err)
	assert.Equal(t, bitsPerSecond(most, time.Minute), destinations.Series[0].Points[0].Bps)

	ips, err := flows.TopIPs(routerID, f, time.Now())
	require.NoError(t, err)
	require.Len(t, ips, 1)
	assert.Equal(t, most, ips[0].Bytes)
}
