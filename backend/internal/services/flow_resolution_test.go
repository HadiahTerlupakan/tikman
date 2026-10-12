package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveFlowRangePicksTheFinestSourceStillHoldingTheStart(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	cases := []struct {
		name     string
		from, to time.Time
		table    string
		step     time.Duration
	}{
		{"15 minutes", now.Add(-15 * time.Minute), now, "router_flow_minute", time.Minute},
		{"24 hours", now.Add(-day), now, "router_flow_minute", time.Minute},
		{"7 days", now.Add(-7 * day), now, "router_flow_minute", 7 * time.Minute},
		{"an hour 30 days back", now.Add(-30 * day), now.Add(-30*day + time.Hour), "router_flow_5min", 5 * time.Minute},
		{"30 days", now.Add(-30 * day), now, "router_flow_5min", 30 * time.Minute},
		{"90 days", now.Add(-90 * day), now, "router_flow_5min", 90 * time.Minute},
		{"1 year", now.Add(-365 * day), now, "router_flow_hourly", 6 * time.Hour},
		{"a day 3 years back", now.Add(-3 * 365 * day), now.Add(-3*365*day + day), "router_flow_daily", day},
		{"5 years", now.Add(-5 * 365 * day), now, "router_flow_daily", 2 * day},
	}
	for _, c := range cases {
		res := resolveFlowRange(c.from, c.to, now)
		assert.Equal(t, c.table, res.source.table, c.name)
		assert.Equal(t, c.step, res.step, c.name)
		assert.Zero(t, res.step%res.source.step, "%s: a whole multiple of the source's step", c.name)
		assert.LessOrEqual(t, int64(c.to.Sub(c.from)/res.step)+1, int64(maxFlowPoints), c.name)
	}
}
