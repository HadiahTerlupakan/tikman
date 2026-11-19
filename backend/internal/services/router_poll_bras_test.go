package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

func newBRASFixture(t *testing.T) *pollFixture {
	t.Helper()
	f := newPollFixture(t)
	f.api.resource = mikrotik.SystemResource{Version: "7.16", BoardName: "CCR2116-12G-4S+", CPULoad: "65",
		Uptime: "1w2d", TotalMemory: "100", FreeMemory: "40"}
	f.api.ppp = 1200
	f.api.cores = []mikrotik.CPULoad{{CPU: "cpu0", Load: "40"}, {CPU: "cpu1", Load: "90"}}
	f.api.pools = []mikrotik.IPPool{
		{Name: "pool-a", Ranges: "198.51.100.1-198.51.100.254", Total: "254", Used: "200", Available: "54"},
		{Name: "pool-b", Ranges: "not-a-range"},
	}
	f.api.poolUsed = map[string]int{"pool-b": 7}
	f.api.radius = []mikrotik.RadiusServer{{ID: "*1", Address: "192.0.2.10", Service: "ppp"}}
	f.api.radiusStats = map[string]mikrotik.RadiusStats{
		"*1": {Requests: "500", Accepts: "450", Rejects: "30", Timeouts: "20", Pending: "2", LastRequestRTT: "12ms"},
	}
	for i := 1; i <= 12; i++ {
		f.api.queues = append(f.api.queues, mikrotik.SimpleQueue{
			Name: fmt.Sprintf("<pppoe-pel-%04d>", i), Target: "<pppoe-pel>", Rate: fmt.Sprintf("%d/%d", i*10, i*1000),
		})
	}
	return f
}

func (f *pollFixture) storedSnapshot(t *testing.T) (models.MikrotikRouter, BRASSnapshot) {
	t.Helper()
	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	var snapshot BRASSnapshot
	require.NoError(t, json.Unmarshal(stored.BRAS, &snapshot))
	return stored, snapshot
}

func TestPollBRASWritesSnapshotAndSample(t *testing.T) {
	f := newBRASFixture(t)
	now := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

	require.NoError(t, f.service.PollBRAS(f.router, now))

	var sample models.RouterBRASSample
	require.NoError(t, f.db.First(&sample, "router_id = ?", f.router.ID).Error)
	assert.Equal(t, 1200, sample.Sessions)
	assert.Equal(t, int16(90), sample.CPUMax)
	assert.Equal(t, int16(65), sample.CPUAvg)
	require.NotNil(t, sample.PoolMinFree)
	assert.Equal(t, 54, *sample.PoolMinFree, "pool-b's total is unknown, so only pool-a counts")
	assert.Equal(t, int64(450), sample.RadiusAccepts)
	assert.Equal(t, int64(30), sample.RadiusRejects)
	assert.Equal(t, int64(20), sample.RadiusTimeouts)

	stored, snapshot := f.storedSnapshot(t)
	assert.Equal(t, models.RouterAPIStatusOK, stored.APIStatus)
	require.NotNil(t, stored.BoardName)
	assert.Equal(t, "CCR2116-12G-4S+", *stored.BoardName)
	require.NotNil(t, stored.BRASAt)
	assert.True(t, stored.BRASAt.Equal(now))

	assert.Equal(t, 1200, snapshot.Sessions)
	assert.Equal(t, []BRASCore{{CPU: "cpu0", Load: 40}, {CPU: "cpu1", Load: 90}}, snapshot.Cores)
	require.Len(t, snapshot.Pools, 2)
	assert.Equal(t, 7, snapshot.Pools[1].Used, "a pool without totals is counted from /ip/pool/used")
	assert.Nil(t, snapshot.Pools[1].Total)
	require.Len(t, snapshot.Radius, 1)
	require.NotNil(t, snapshot.Radius[0].RTTMs)
	assert.Equal(t, int64(12), *snapshot.Radius[0].RTTMs)
	assert.Nil(t, snapshot.Radius[0].Answering, "no earlier reading to judge against")

	assert.True(t, snapshot.QueuesFound)
	require.Len(t, snapshot.TopUsers, 10)
	assert.Equal(t, BRASUser{Name: "<pppoe-pel-0012>", DownBps: 12000, UpBps: 120}, snapshot.TopUsers[0])
	assert.Equal(t, "<pppoe-pel-0003>", snapshot.TopUsers[9].Name)
}

func TestPollBRASJudgesRadiusAgainstThePreviousSnapshot(t *testing.T) {
	f := newBRASFixture(t)
	now := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	require.NoError(t, f.service.PollBRAS(f.router, now))

	f.api.radiusStats["*1"] = mikrotik.RadiusStats{Requests: "540", Accepts: "450", Rejects: "30", Timeouts: "60"}
	require.NoError(t, f.service.PollBRAS(f.router, now.Add(time.Minute)))

	_, snapshot := f.storedSnapshot(t)
	require.NotNil(t, snapshot.Radius[0].Answering)
	assert.False(t, *snapshot.Radius[0].Answering)
	assert.Nil(t, snapshot.Radius[0].RTTMs, "an empty rtt is unknown, not zero")

	var samples int64
	f.db.Model(&models.RouterBRASSample{}).Where("router_id = ?", f.router.ID).Count(&samples)
	assert.Equal(t, int64(2), samples)
}

func TestPollBRASNoRadius(t *testing.T) {
	f := newBRASFixture(t)
	f.api.radius = nil
	f.api.queues = nil

	require.NoError(t, f.service.PollBRAS(f.router, time.Now()))

	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(stored.BRAS, &raw))
	assert.JSONEq(t, `[]`, string(raw["radius"]))
	assert.JSONEq(t, `[]`, string(raw["top_users"]))
	assert.JSONEq(t, `false`, string(raw["queues_found"]))

	var sample models.RouterBRASSample
	require.NoError(t, f.db.First(&sample, "router_id = ?", f.router.ID).Error)
	assert.Zero(t, sample.RadiusAccepts)
	assert.Zero(t, sample.RadiusRejects)
	assert.Zero(t, sample.RadiusTimeouts)
}

func TestPollBRASReadErrorFailsJob(t *testing.T) {
	f := newBRASFixture(t)
	f.api.pppErr = errors.New("ppp read timed out")

	err := f.service.PollBRAS(f.router, time.Now())
	require.ErrorContains(t, err, "ppp read timed out")

	var samples int64
	f.db.Model(&models.RouterBRASSample{}).Count(&samples)
	assert.Zero(t, samples)
	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusError, stored.APIStatus)
	assert.Nil(t, stored.BRASAt)
}
