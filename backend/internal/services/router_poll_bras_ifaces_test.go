package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

type brasPollFixture struct {
	*pollFixture
	asked  [][]int
	got    []connectivity.IfCounters
	getErr error
	walked int
}

func newBRASPollFixture(t *testing.T) *brasPollFixture {
	t.Helper()
	f := &brasPollFixture{pollFixture: newPollFixture(t)}
	f.router.Role = models.RouterRoleBRAS
	f.service.readCounters = func(context.Context, string, int, string) ([]connectivity.IfCounters, error) {
		f.walked++
		return nil, errors.New("a BRAS must not be walked")
	}
	f.service.getIfCounters = func(_ context.Context, _ string, _ int, _ string, indexes []int) ([]connectivity.IfCounters, error) {
		f.asked = append(f.asked, indexes)
		return f.got, f.getErr
	}
	require.NoError(t, f.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ?", f.router.ID, models.RouterPollKindDiscovery).
		Update("due_at", time.Now().Add(6*time.Hour)).Error)
	return f
}

func (f *brasPollFixture) addMapped(t *testing.T, name string, index int) models.RouterInterface {
	t.Helper()
	iface := f.addInterface(t, name, "", false)
	require.NoError(t, f.db.Model(&iface).Update("if_index", index).Error)
	return iface
}

func (f *brasPollFixture) discoveryDueAt(t *testing.T) time.Time {
	t.Helper()
	var job models.RouterPollJob
	require.NoError(t, f.db.First(&job, "router_id = ? AND kind = ?", f.router.ID, models.RouterPollKindDiscovery).Error)
	return job.DueAt
}

func TestBRASPollAsksOnlyMappedPorts(t *testing.T) {
	f := newBRASPollFixture(t)
	first := f.addMapped(t, "ether-up", 7)
	second := f.addMapped(t, "ether-down", 3)
	f.addInterface(t, "ether-new", "", false)
	start := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

	f.got = []connectivity.IfCounters{
		{IfIndex: 3, Name: "ether-down", InOctets: 1_000, OutOctets: 1_000, OperUp: true},
		{IfIndex: 7, Name: "ether-up", InOctets: 1_000, OutOctets: 1_000, OperUp: true},
	}
	require.NoError(t, f.service.PollInterfaces(f.router, start))
	f.got[0].InOctets += 7_500_000
	f.got[1].InOctets += 75_000_000
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	assert.Equal(t, [][]int{{3, 7}, {3, 7}}, f.asked)
	assert.Zero(t, f.walked)
	for id, want := range map[any]float64{second.ID: 1_000_000, first.ID: 10_000_000} {
		var metric models.RouterInterfaceMetric
		require.NoError(t, f.db.First(&metric, "interface_id = ?", id).Error)
		assert.InDelta(t, want, metric.InBps, 0.01)
	}
	var router models.MikrotikRouter
	require.NoError(t, f.db.First(&router, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterStatusUp, router.Status)
}

func TestBRASPollSkipsRenamedIndex(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addMapped(t, "ether-up", 3)
	reused := f.addMapped(t, "ether-down", 7)
	start := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

	f.got = []connectivity.IfCounters{
		{IfIndex: 3, Name: "ether-up", InOctets: 1_000},
		{IfIndex: 7, Name: "<pppoe-pel-0001>", InOctets: 1_000},
	}
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "id = ?", reused.ID).Error)
	assert.Nil(t, stored.PrevSampleAt)
	assert.Equal(t, models.RouterStatusUp, f.storedRouter(t).Status)
	assert.False(t, f.discoveryDueAt(t).After(start), "discovery must be due to map the port again")
}

func TestBRASPollLeavesDiscoveryAloneWhenEverythingMatches(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addMapped(t, "ether-up", 3)
	before := f.discoveryDueAt(t)

	f.got = []connectivity.IfCounters{{IfIndex: 3, Name: "ether-up", InOctets: 1_000}}
	require.NoError(t, f.service.PollInterfaces(f.router, time.Now()))

	assert.True(t, f.discoveryDueAt(t).Equal(before))
}

func TestBRASPollMapsAgainWhenAnIndexIsGone(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addMapped(t, "ether-up", 3)
	start := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

	f.got = nil
	require.NoError(t, f.service.PollInterfaces(f.router, start))

	assert.False(t, f.discoveryDueAt(t).After(start))
}

// A mismatch discovery cannot fix (an ifName it never sees) must not pull
// discovery forward every minute once it has just run.
func TestBRASPollLeavesARecentDiscoveryAlone(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addMapped(t, "ether-up", 3)
	start := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	ranAt := start.Add(-5 * time.Minute)
	require.NoError(t, f.db.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ?", f.router.ID, models.RouterPollKindDiscovery).
		Update("last_run_at", ranAt).Error)
	before := f.discoveryDueAt(t)

	f.got = []connectivity.IfCounters{{IfIndex: 3, Name: "<pppoe-pel-0001>"}}
	require.NoError(t, f.service.PollInterfaces(f.router, start))
	require.NoError(t, f.service.PollInterfaces(f.router, start.Add(time.Minute)))

	assert.True(t, f.discoveryDueAt(t).Equal(before))
}

// With no port mapped nothing is asked of the router, so nothing says it is up.
func TestBRASPollWithNoMappedPortsLeavesStatusAlone(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addInterface(t, "ether-new", "", false)
	start := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	before := f.storedRouter(t)

	require.NoError(t, f.service.PollInterfaces(f.router, start))

	after := f.storedRouter(t)
	assert.Empty(t, f.asked)
	assert.Equal(t, before.Status, after.Status)
	assert.Nil(t, after.LastSeenAt)
	assert.False(t, f.discoveryDueAt(t).After(start), "discovery must be due to map the ports")
}

func TestBRASPollGetFailureMarksDown(t *testing.T) {
	f := newBRASPollFixture(t)
	f.addMapped(t, "ether-up", 3)
	f.getErr = errors.New("request timeout")

	err := f.service.PollInterfaces(f.router, time.Now())

	assert.Error(t, err)
	assert.Equal(t, models.RouterStatusDown, f.storedRouter(t).Status)
}

func TestBorderPollStillWalks(t *testing.T) {
	f := newPollFixture(t)
	f.addInterface(t, "vlan-transit", "", false)
	f.service.getIfCounters = func(context.Context, string, int, string, []int) ([]connectivity.IfCounters, error) {
		t.Error("a border router must be walked, not asked by index")
		return nil, nil
	}
	f.counters = []connectivity.IfCounters{{IfIndex: 19, Name: "vlan-transit", InOctets: 1}}

	require.NoError(t, f.service.PollInterfaces(f.router, time.Now()))

	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "router_id = ?", f.router.ID).Error)
	assert.NotNil(t, stored.PrevSampleAt)
}
