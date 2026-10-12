package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func TestRouterTrafficSourceFollowsSpanAndRetention(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		from, to time.Time
		want     routerTrafficSource
	}{
		{"last six hours", now.Add(-6 * time.Hour), now, routerTrafficRaw},
		{"last day", now.Add(-24 * time.Hour), now, routerTraffic5Min},
		{"last week", now.Add(-7 * 24 * time.Hour), now, routerTraffic5Min},
		{"last month", now.Add(-30 * 24 * time.Hour), now, routerTrafficHourly},
		// One hour, but ten days back: raw is kept seven days, so the short
		// span alone would read rows retention has already dropped.
		{"one hour ten days ago", now.Add(-10 * 24 * time.Hour), now.Add(-10*24*time.Hour + time.Hour), routerTraffic5Min},
		{"one hour forty days ago", now.Add(-40 * 24 * time.Hour), now.Add(-40*24*time.Hour + time.Hour), routerTrafficHourly},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, routerTrafficSourceFor(c.from, c.to, now), c.name)
	}
}

func TestInterfaceTrafficReadsRawSamplesInOrder(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)
	now := time.Now().UTC().Truncate(time.Minute)
	for i, bps := range []float64{3e8, 1e8, 2e8} {
		require.NoError(t, f.db.Create(&models.RouterInterfaceMetric{
			Time: now.Add(-time.Duration(3-i) * time.Minute), InterfaceID: iface.ID, InBps: bps, OutBps: bps / 10,
		}).Error)
	}
	queries := NewRouterQueryService(f.db)

	points, err := queries.InterfaceTraffic(f.router.ID, iface.ID, now.Add(-time.Hour), now)

	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.InDelta(t, 3e8, points[0].InBps, 1)
	assert.InDelta(t, 2e7, points[2].OutBps, 1)
	assert.True(t, points[0].Time.Before(points[1].Time))
}

func TestInterfaceTrafficRefusesAnInterfaceOfAnotherRouter(t *testing.T) {
	f := newPollFixture(t)
	iface := f.addInterface(t, "vlan-transit", "", false)

	_, err := NewRouterQueryService(f.db).InterfaceTraffic(uuid.New(), iface.ID, time.Now().Add(-time.Hour), time.Now())

	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestBGPSummaryCountsEstablishedSessionsPerRouter(t *testing.T) {
	f := newPollFixture(t)
	f.api.connections, f.api.sessions = borderConnections(), borderSessions()
	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	summary, err := NewRouterQueryService(f.db).BGPSummary()

	require.NoError(t, err)
	assert.Equal(t, 4, summary[f.router.ID].Established)
	assert.Equal(t, 5, summary[f.router.ID].Total)
}

func TestListEventsIsNewestFirstWithinTheRange(t *testing.T) {
	f := newPollFixture(t)
	base := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, f.db.Create(&models.RouterBGPEvent{RouterID: f.router.ID, SessionName: "p",
			FromState: models.BGPStateEstablished, ToState: models.BGPStateDown,
			OccurredAt: base.Add(time.Duration(i) * time.Hour)}).Error)
	}

	events, err := NewRouterQueryService(f.db).ListEvents(f.router.ID, base.Add(30*time.Minute), base.Add(3*time.Hour))

	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.True(t, events[0].OccurredAt.After(events[1].OccurredAt))
}
