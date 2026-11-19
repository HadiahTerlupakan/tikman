package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

var brasT0 = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

func brasSamples(sessions ...int) []models.RouterBRASSample {
	out := make([]models.RouterBRASSample, len(sessions))
	for i, s := range sessions {
		out[i] = models.RouterBRASSample{Time: brasT0.Add(time.Duration(i) * time.Minute), Sessions: s}
	}
	return out
}

func TestBRASDrops(t *testing.T) {
	cases := []struct {
		name    string
		samples []models.RouterBRASSample
		want    []BRASDrop
	}{
		{"a fall of 120 from 2000", brasSamples(2000, 1880), []BRASDrop{{Time: brasT0.Add(time.Minute), Lost: 120}}},
		{"a fall of 50 is under the floor", brasSamples(2000, 1950), []BRASDrop{}},
		{"140 of 3000 is under 5%", brasSamples(3000, 2860), []BRASDrop{}},
		{"150 of 3000 is 5%", brasSamples(3000, 2850), []BRASDrop{{Time: brasT0.Add(time.Minute), Lost: 150}}},
		{"a rise is no drop", brasSamples(1000, 2000), []BRASDrop{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, brasDrops(c.samples))
		})
	}
}

func TestBRASDropsIgnoresAGapOverTwoMinutes(t *testing.T) {
	samples := brasSamples(2000, 1000)
	samples[1].Time = brasT0.Add(3 * time.Minute)

	assert.Empty(t, brasDrops(samples), "a poller outage is not a drop")

	samples[1].Time = brasT0.Add(2 * time.Minute)
	assert.Len(t, brasDrops(samples), 1, "two minutes apart still counts")
}

func seedBRASRouter(t *testing.T, db *gorm.DB, role models.RouterRole) uuid.UUID {
	t.Helper()
	router := models.MikrotikRouter{Name: "bras-1", Host: "192.0.2.10", Role: role,
		BRAS: []byte(`{"sessions":1200}`), BRASAt: &brasT0}
	require.NoError(t, db.Create(&router).Error)
	return router.ID
}

func seedBRASSample(t *testing.T, db *gorm.DB, id uuid.UUID, at time.Time, sessions int, rejects, timeouts int64) {
	t.Helper()
	require.NoError(t, db.Create(&models.RouterBRASSample{RouterID: id, Time: at, Sessions: sessions,
		RadiusAccepts: rejects * 10, RadiusRejects: rejects, RadiusTimeouts: timeouts}).Error)
}

func TestBRASViewServesTheSnapshotAndSamples(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-15*time.Minute), 1500, 10, 5)
	seedBRASSample(t, db, id, brasT0, 1380, 20, 8)

	view, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0.Add(time.Minute))

	require.NoError(t, err)
	assert.JSONEq(t, `{"sessions":1200}`, string(view.Snapshot))
	assert.Equal(t, brasT0, view.At.UTC())
	assert.Len(t, view.Samples, 2)
	assert.Equal(t, 1500, *view.Sessions15mAgo)
	assert.Empty(t, view.Drops, "samples 15 minutes apart are a gap, not a fall")
}

func TestSessions15mAgo(t *testing.T) {
	cases := []struct {
		name   string
		offset time.Duration
		want   *int
	}{
		{"exact", -15 * time.Minute, intPtr(900)},
		{"two minutes off", -17 * time.Minute, intPtr(900)},
		{"three minutes off", -18 * time.Minute, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			service, db := newRouterServiceForTest(t)
			id := seedBRASRouter(t, db, models.RouterRoleBRAS)
			seedBRASSample(t, db, id, brasT0.Add(c.offset), 900, 0, 0)
			seedBRASSample(t, db, id, brasT0, 1000, 0, 0)

			view, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0)

			require.NoError(t, err)
			assert.Equal(t, c.want, view.Sessions15mAgo)
		})
	}
}

func TestSessions15mAgoPicksTheNearestSample(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-16*time.Minute), 800, 0, 0)
	seedBRASSample(t, db, id, brasT0.Add(-14*time.Minute-30*time.Second), 850, 0, 0)
	seedBRASSample(t, db, id, brasT0, 1000, 0, 0)

	view, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0)

	require.NoError(t, err)
	assert.Equal(t, 850, *view.Sessions15mAgo)
}

func TestRadius1h(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-61*time.Minute), 1000, 100, 40)
	seedBRASSample(t, db, id, brasT0, 1000, 130, 90)

	view, err := service.BRASView(id, brasT0.Add(-10*time.Minute), brasT0)

	require.NoError(t, err)
	assert.Equal(t, int64(30), *view.RadiusRejects1h)
	assert.Equal(t, int64(50), *view.RadiusTimeouts1h)
	assert.Equal(t, int64(300), *view.RadiusAccepts1h, "from samples outside the requested range")
}

func TestRadius1hIsNilWithoutAnHourOfHistory(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-30*time.Minute), 1000, 100, 40)
	seedBRASSample(t, db, id, brasT0, 1000, 130, 90)

	view, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0)

	require.NoError(t, err)
	assert.Nil(t, view.RadiusRejects1h)
	assert.Nil(t, view.RadiusTimeouts1h)
	assert.Nil(t, view.RadiusAccepts1h)
}

func TestRadius1hIsNilAfterACounterReset(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-60*time.Minute), 1000, 500, 40)
	seedBRASSample(t, db, id, brasT0, 1000, 20, 90)

	view, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0)

	require.NoError(t, err)
	assert.Nil(t, view.RadiusRejects1h, "a reboot zeroes the counters; the difference is meaningless")
	assert.Nil(t, view.RadiusAccepts1h)
	assert.Equal(t, int64(50), *view.RadiusTimeouts1h, "each counter is judged alone")
}

func TestBRASViewClampsTheRangeAtThirtyOneDays(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBRAS)
	seedBRASSample(t, db, id, brasT0.Add(-40*24*time.Hour), 1, 0, 0)
	seedBRASSample(t, db, id, brasT0.Add(-30*24*time.Hour), 2, 0, 0)
	seedBRASSample(t, db, id, brasT0, 3, 0, 0)

	view, err := service.BRASView(id, brasT0.Add(-60*24*time.Hour), brasT0)

	require.NoError(t, err)
	assert.Len(t, view.Samples, 2)
}

func TestBRASViewRejectsBorder(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	id := seedBRASRouter(t, db, models.RouterRoleBorder)

	_, err := service.BRASView(id, brasT0.Add(-time.Hour), brasT0)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	_, err = service.BRASView(uuid.New(), brasT0.Add(-time.Hour), brasT0)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestBRASViewWithoutASnapshotYet(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router := models.MikrotikRouter{Name: "bras-2", Host: "192.0.2.11", Role: models.RouterRoleBRAS}
	require.NoError(t, db.Create(&router).Error)

	view, err := service.BRASView(router.ID, brasT0.Add(-time.Hour), brasT0)

	require.NoError(t, err)
	out, err := json.Marshal(view)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"samples":[]`)
	assert.Contains(t, string(out), `"drops":[]`)
	assert.Nil(t, view.Sessions15mAgo)
}
