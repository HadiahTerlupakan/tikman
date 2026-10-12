package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestDestinationsKeepWIBDaysSeparateOnTheDailyRollup(t *testing.T) {
	db := setupPostgresTestDB(t)
	flows := NewFlowQueryService(db)
	routerID, transit := uuid.New(), uuid.New()
	wib := time.FixedZone("WIB", 7*3600)
	threeYearsAgo := time.Now().In(wib).AddDate(-3, 0, 0)
	day0 := time.Date(threeYearsAgo.Year(), threeYearsAgo.Month(), threeYearsAgo.Day(), 0, 0, 0, 0, wib).UTC()
	day1 := day0.Add(24 * time.Hour)
	require.Equal(t, 17, day0.Hour(), "a WIB day starts at 17:00 UTC")
	for i, day := range []time.Time{day0, day1} {
		seedFlowMinute(t, db, models.RouterFlowMinute{Time: day.Add(time.Hour), RouterID: routerID,
			UpstreamInterfaceID: transit, Direction: "download", ASN: 64496, Bytes: int64(1000 * (i + 1))})
	}
	require.NoError(t, db.Exec(fmt.Sprintf("CALL refresh_continuous_aggregate('router_flow_daily', '%s', '%s')",
		day0.Add(-24*time.Hour).Format(time.RFC3339), day1.Add(48*time.Hour).Format(time.RFC3339))).Error)

	out, err := flows.Destinations(routerID, FlowFilter{From: day0.Add(-12 * time.Hour), To: day1.Add(36 * time.Hour)}, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 86400, out.StepSeconds)
	require.Len(t, out.Series[0].Points, 2)
	assert.True(t, day0.Equal(out.Series[0].Points[0].Time), "got %s", out.Series[0].Points[0].Time)
	assert.True(t, day1.Equal(out.Series[0].Points[1].Time), "got %s", out.Series[0].Points[1].Time)
	assert.InDelta(t, 1000*8/86400.0, out.Series[0].Points[0].Bps, 0.001)
	assert.InDelta(t, 2000*8/86400.0, out.Series[0].Points[1].Bps, 0.001)
}
