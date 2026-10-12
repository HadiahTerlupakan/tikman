package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
)

// pollInbound polls once per minute, adding the given inbound octets each
// time, and returns the stored interface after the last poll.
func pollInbound(t *testing.T, f *pollFixture, name string, start time.Time, inbound ...uint64) (models.RouterInterface, time.Time) {
	t.Helper()
	at := start
	for _, octets := range inbound {
		f.counters[0].InOctets += octets
		f.counters[0].OutOctets += 100
		require.NoError(t, f.service.PollInterfaces(f.router, at))
		at = at.Add(time.Minute)
	}
	var stored models.RouterInterface
	require.NoError(t, f.db.First(&stored, "name = ?", name).Error)
	return stored, at
}

func silenceFixture(t *testing.T) (*pollFixture, time.Time) {
	t.Helper()
	f := newPollFixture(t)
	f.addInterface(t, "vlan-pop-a", "", false)
	f.counters = []connectivity.IfCounters{{IfIndex: 71, Name: "vlan-pop-a", OperUp: true}}
	return f, time.Date(2026, 10, 12, 11, 50, 0, 0, time.UTC)
}

// The POP's VLAN stays up while the site behind the carrier is gone; the only
// sign is that nothing comes in. Three silent samples mark it.
func TestAnInterfaceThatStopsReceivingIsMarkedSilent(t *testing.T) {
	f, start := silenceFixture(t)

	// The first poll only primes the counters; then traffic, then silence.
	stored, _ := pollInbound(t, f, "vlan-pop-a", start, 0, 1_000_000, 0, 0)
	assert.Nil(t, stored.SilentSince, "two silent samples are not yet a stop")
	require.NotNil(t, stored.LastTrafficAt)

	stored, _ = pollInbound(t, f, "vlan-pop-a", start.Add(4*time.Minute), 0)
	require.NotNil(t, stored.SilentSince)
	assert.Equal(t, start.Add(2*time.Minute), stored.SilentSince.UTC(), "dated from the first silent sample")
	assert.Equal(t, "up", stored.OperStatus, "the router still reports the VLAN up")
}

func TestTrafficReturningClearsSilence(t *testing.T) {
	f, start := silenceFixture(t)
	stored, next := pollInbound(t, f, "vlan-pop-a", start, 0, 1_000_000, 0, 0, 0)
	require.NotNil(t, stored.SilentSince)

	stored, _ = pollInbound(t, f, "vlan-pop-a", next, 5_000)

	assert.Nil(t, stored.SilentSince)
	assert.Equal(t, next.UTC(), stored.LastTrafficAt.UTC())
}

// A spare VLAN that has never carried traffic is quiet, not down.
func TestAnInterfaceThatNeverHadTrafficIsNotMarked(t *testing.T) {
	f, start := silenceFixture(t)

	stored, _ := pollInbound(t, f, "vlan-pop-a", start, 0, 0, 0, 0, 0, 0)

	assert.Nil(t, stored.SilentSince)
	assert.Nil(t, stored.LastTrafficAt)
}
