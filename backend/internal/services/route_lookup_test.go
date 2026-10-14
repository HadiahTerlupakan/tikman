package services

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
)

var lookupNow = time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)

func TestLookupRoutesFindsThePrefixHoldingAnAddress(t *testing.T) {
	f := newPollFixture(t)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/23": {
		{Session: "peer-ix-1", ASPath: "64500,64496", LocalPref: "700", Active: "true", Gateway: "192.0.2.1"},
		{Session: "peer-transit-1", ASPath: "64501,64502,64496", LocalPref: "100", Gateway: "192.0.2.9"},
	}}

	out, err := f.service.LookupRoutes(f.router, "198.51.100.7", lookupNow)

	require.NoError(t, err)
	assert.Equal(t, []string{"198.51.100.0/24", "198.51.100.0/23"}, f.api.asked)
	want := []StoredRoute{
		{Session: "peer-ix-1", ASPath: "64500,64496", LocalPref: 700, Active: true, Gateway: "192.0.2.1"},
		{Session: "peer-transit-1", ASPath: "64501,64502,64496", LocalPref: 100, Gateway: "192.0.2.9"},
	}
	assert.Equal(t, RouteLookup{Target: "198.51.100.7", Prefix: "198.51.100.0/23", Routes: want, Queries: 2, CheckedAt: lookupNow}, out)
	assert.Equal(t, map[string][]StoredRoute{"198.51.100.0/23": want}, storedRoutes(t, f), "the hourly advice learns the prefix too")
	assert.Equal(t, 1, f.api.closed)
}

func TestLookupRoutesReadsAPrefixExactly(t *testing.T) {
	f := newPollFixture(t)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/23": {{Session: "peer-ix-1", LocalPref: "700"}}}

	out, err := f.service.LookupRoutes(f.router, "198.51.100.0/23", lookupNow)

	require.NoError(t, err)
	assert.Equal(t, []string{"198.51.100.0/23"}, f.api.asked)
	assert.Equal(t, "198.51.100.0/23", out.Prefix)
}

func TestLookupRoutesUsesAKnownPrefixWithoutSearching(t *testing.T) {
	f := newPollFixture(t)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/23": {{Session: "peer-ix-1", LocalPref: "700"}}}
	_, err := f.service.LookupRoutes(f.router, "198.51.100.0/23", lookupNow)
	require.NoError(t, err)
	f.api.asked = nil

	out, err := f.service.LookupRoutes(f.router, "198.51.101.9", lookupNow.Add(time.Minute))

	require.NoError(t, err)
	assert.Equal(t, []string{"198.51.100.0/23"}, f.api.asked, "the address is inside the prefix stored earlier")
	assert.Equal(t, "198.51.100.0/23", out.Prefix)
	assert.Equal(t, lookupNow.Add(time.Minute), out.CheckedAt)
}

func TestLookupRoutesReportsAnAddressNoPrefixHolds(t *testing.T) {
	f := newPollFixture(t)

	out, err := f.service.LookupRoutes(f.router, "203.0.113.1", lookupNow)

	require.NoError(t, err)
	assert.Equal(t, routeLongestPrefix-routeShortestPrefix+1, out.Queries, "asked /24 down to /8")
	assert.Empty(t, out.Prefix)
	assert.Empty(t, out.Routes)
	assert.Empty(t, storedRoutes(t, f))
}

func TestLookupRoutesRejectsATargetThatIsNotAnIPv4AddressOrPrefix(t *testing.T) {
	f := newPollFixture(t)
	for _, target := range []string{"", "example.com", "2001:db8::1", "198.51.100.0/33", "198.51.100.0/0"} {
		_, err := f.service.LookupRoutes(f.router, target, lookupNow)
		assert.ErrorIs(t, err, ErrLookupTarget, target)
	}
	assert.Empty(t, f.api.asked)
}

func TestLookupRoutesFailsOnARouterErrorWithoutStoring(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = errors.New("read timeout")

	_, err := f.service.LookupRoutes(f.router, "198.51.100.7", lookupNow)

	assert.ErrorContains(t, err, "read timeout")
	assert.Empty(t, storedRoutes(t, f))
}

// Each query scans the router's whole table, so two operators looking up at
// once would double the load; the second is told to wait instead.
func TestLookupRoutesRefusesASecondLookupOnTheSameRouter(t *testing.T) {
	f := newPollFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	f.api.onRoutesTo = func() {
		close(started)
		<-release
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = f.service.LookupRoutes(f.router, "198.51.100.7", lookupNow)
	}()
	<-started

	_, err := f.service.LookupRoutes(f.router, "198.51.100.8", lookupNow)

	assert.ErrorIs(t, err, ErrLookupBusy)
	close(release)
	wg.Wait()
	_, err = f.service.LookupRoutes(f.router, "198.51.100.8", lookupNow)
	assert.NoError(t, err, "the router is free again once the first lookup returns")
}

// The router matches dst-address exactly, so a prefix typed more specific
// than what is announced finds nothing by itself: the covering prefix is
// searched for from one bit shorter.
func TestLookupRoutesFallsBackToTheCoveringPrefix(t *testing.T) {
	f := newPollFixture(t)
	f.api.bgpRoutes = map[string][]mikrotik.BGPRoute{"198.51.100.0/22": {{Session: "peer-ix-1", LocalPref: "700"}}}

	out, err := f.service.LookupRoutes(f.router, "198.51.100.0/24", lookupNow)

	require.NoError(t, err)
	assert.Equal(t, []string{"198.51.100.0/24", "198.51.100.0/23", "198.51.100.0/22"}, f.api.asked)
	assert.Equal(t, "198.51.100.0/22", out.Prefix)
}
