package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestPeeringSplitsOneDestinationOverItsUpstreams(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, ix, transit := uuid.New(), uuid.New(), uuid.New()
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-10 * time.Minute)
	for i, ixBytes := range []int64{3000, 9000} {
		at := start.Add(time.Duration(i) * time.Minute)
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
			UpstreamInterfaceID: ix, Direction: "download", ASN: 64496, Bytes: ixBytes})
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
			UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: 1000})
		// Another destination on the same upstream must not count.
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
			UpstreamInterfaceID: transit, Direction: "download", ASN: 64497, Bytes: 50_000})
	}
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ", UpdatedAt: end}).Error)

	out, err := flows.Peering(routerID, FlowFilter{From: start, To: end, Direction: "download"}, 64496)

	require.NoError(t, err)
	assert.EqualValues(t, 64496, out.ASN)
	assert.Equal(t, "EXAMPLE-NET-A", out.Name)
	assert.EqualValues(t, 14_000, out.TotalBytes)
	assert.EqualValues(t, 60, out.StepSeconds)
	assert.EqualValues(t, 60, out.PeakWindowSeconds)
	require.Len(t, out.Rows, 2)
	assert.Equal(t, ix, out.Rows[0].UpstreamInterfaceID, "largest upstream first")
	assert.EqualValues(t, 12_000, out.Rows[0].Bytes)
	assert.InDelta(t, 12_000*100/14_000.0, out.Rows[0].Percent, 0.001)
	assert.InDelta(t, 12_000*8/(10*60.0), out.Rows[0].AvgBps, 0.001)
	assert.InDelta(t, 9000*8/60.0, out.Rows[0].PeakBps, 0.001)
	assert.Equal(t, transit, out.Rows[1].UpstreamInterfaceID)
	assert.EqualValues(t, 2000, out.Rows[1].Bytes)

	require.Len(t, out.Series, 2)
	assert.Equal(t, ix, out.Series[0].UpstreamInterfaceID, "series follow the rows' order")
	require.Len(t, out.Series[0].Points, 2)
	assert.InDelta(t, 3000*8/60.0, out.Series[0].Points[0].Bps, 0.001)
	assert.InDelta(t, 9000*8/60.0, out.Series[0].Points[1].Bps, 0.001)
}

func TestPeeringSplitsOneDestinationOverPOPsAndTheirUpstreams(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, popA, ix, transit := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	for _, row := range []models.RouterFlowMinute{
		{POPInterfaceID: &popA, UpstreamInterfaceID: ix, Bytes: 6000},
		{POPInterfaceID: &popA, UpstreamInterfaceID: transit, Bytes: 2000},
		{UpstreamInterfaceID: ix, Bytes: 2000}, // an address in no POP's blocks
	} {
		row.Time, row.RouterID, row.Direction, row.ASN = at, routerID, "download", 64496
		seedFlowMinute(t, db, row)
	}

	out, err := flows.Peering(routerID, FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute),
		Direction: "download"}, 64496)

	require.NoError(t, err)
	require.Len(t, out.POPs, 2)
	require.NotNil(t, out.POPs[0].POPInterfaceID, "largest POP first")
	assert.Equal(t, popA, *out.POPs[0].POPInterfaceID)
	assert.EqualValues(t, 8000, out.POPs[0].Bytes)
	assert.InDelta(t, 80, out.POPs[0].Percent, 0.001)
	require.Len(t, out.POPs[0].Upstreams, 2)
	assert.Equal(t, ix, out.POPs[0].Upstreams[0].UpstreamInterfaceID, "the POP's largest upstream first")
	assert.EqualValues(t, 6000, out.POPs[0].Upstreams[0].Bytes)
	assert.Nil(t, out.POPs[1].POPInterfaceID)
	assert.EqualValues(t, 2000, out.POPs[1].Bytes)
}

// The tab may have one upstream selected; the split is across upstreams, so
// that filter must not narrow it to a single 100% row.
func TestPeeringIgnoresTheUpstreamFilter(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, ix, transit := uuid.New(), uuid.New(), uuid.New()
	at := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)
	for upstream, bytes := range map[uuid.UUID]int64{ix: 3000, transit: 1000} {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: at, RouterID: routerID,
			UpstreamInterfaceID: upstream, Direction: "download", ASN: 64496, Bytes: bytes})
	}

	out, err := flows.Peering(routerID, FlowFilter{From: at.Add(-time.Minute), To: at.Add(time.Minute),
		Direction: "download", Upstream: &transit}, 64496)

	require.NoError(t, err)
	require.Len(t, out.Rows, 2)
	assert.EqualValues(t, 4000, out.TotalBytes)
}

func TestPeeringOfADestinationWithNoTrafficIsEmpty(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	end := time.Now().UTC().Truncate(time.Minute)

	out, err := flows.Peering(uuid.New(), FlowFilter{From: end.Add(-time.Hour), To: end}, 64500)

	require.NoError(t, err)
	assert.Zero(t, out.TotalBytes)
	assert.Empty(t, out.Rows)
	assert.Empty(t, out.Series)
	assert.Empty(t, out.Name)
}

func TestPeeringOfLainnyaIsNamedLainnya(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	end := time.Now().UTC().Truncate(time.Minute)

	out, err := flows.Peering(uuid.New(), FlowFilter{From: end.Add(-time.Hour), To: end}, otherASN)

	require.NoError(t, err)
	assert.Equal(t, otherASNName, out.Name)
}
