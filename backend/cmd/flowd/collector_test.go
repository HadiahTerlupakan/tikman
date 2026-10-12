package main

import (
	"math/rand"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"github.com/tikman/olt-provisioning/internal/netflow/ipfixtest"
	"go.uber.org/zap"
)

var (
	testRouterID = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
	testTransit  = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	testPOP      = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	routerAddr   = netip.MustParseAddr("192.0.2.1")
	minute0      = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
)

func testCollector(t *testing.T, upstreams map[uint32]uuid.UUID) *collector {
	t.Helper()
	asns := &asnSource{logger: zap.NewNop()}
	table, err := netflow.ParseASNTable(strings.NewReader("198.51.100.0\t198.51.100.255\t64497\tZZ\tEXAMPLE-NET-B\n"))
	require.NoError(t, err)
	asns.table.Store(table)
	c := newCollector(asns, minute0)
	c.setRouters(map[netip.Addr]*netflow.RouterMap{routerAddr: {
		RouterID: testRouterID, Sampling: 100, Upstreams: upstreams,
		POPs: netflow.NewPrefixTable([]netflow.PrefixEntry{
			{Prefix: netip.MustParsePrefix("203.0.113.0/27"), InterfaceID: testPOP}}),
	}})
	return c
}

func flowPacket() []byte {
	return ipfixtest.Message(1, ipfixtest.TemplateSet(256, ipfixtest.FlowFields...),
		ipfixtest.DataSet(256, ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)))
}

func TestHandleCountsAKnownRoutersFlows(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(routerAddr, flowPacket())
	rows, seen, dropped := c.closeMinute(minute0.Add(time.Minute))

	require.Len(t, rows, 1)
	assert.Equal(t, netflow.MinuteRow{Time: minute0, MinuteKey: netflow.MinuteKey{RouterID: testRouterID,
		POP: testPOP, Upstream: testTransit, Direction: netflow.Download, ASN: 64497},
		Bytes: 150_000, Packets: 300}, rows[0])
	assert.Equal(t, []uuid.UUID{testRouterID}, seen)
	assert.Empty(t, dropped)
}

// A dual-stack socket reports an IPv4 sender as ::ffff:a.b.c.d.
func TestHandleRecognisesAnIPv4MappedSender(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(netip.AddrFrom16(routerAddr.As16()), flowPacket())
	rows, _, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Len(t, rows, 1)
}

// The port is open to the internet: a datagram from anyone else is not parsed,
// and leaves no template state behind for a sender to grow.
func TestHandleIgnoresUnknownSenders(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(netip.MustParseAddr("198.51.100.9"), flowPacket())
	rows, seen, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Empty(t, rows)
	assert.Empty(t, seen)
	assert.EqualValues(t, 1, c.unknownSources.Load())
	assert.Empty(t, c.templates)
}

// Review Focus 3: a router whose upstreams discovery has not matched yet
// sends flows nothing can place. They are dropped, but the router is still
// marked as heard from, so the Router tab says IPFIX is arriving.
func TestHandleMarksARouterHeardEvenWhenNoFlowCanBePlaced(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{})

	c.handle(routerAddr, flowPacket())
	rows, seen, _ := c.closeMinute(minute0.Add(time.Minute))

	assert.Empty(t, rows)
	assert.Equal(t, []uuid.UUID{testRouterID}, seen)
}

func TestHandleCountsGarbageWithoutFailing(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(routerAddr, []byte{0, 10, 0})

	assert.EqualValues(t, 1, c.malformed.Load())
}

// A registered router is still a source of arbitrary bytes: every truncation
// and a spread of corruptions of a real message must leave the collector
// counting the next good one.
func TestHandleSurvivesCorruptPacketsFromARegisteredRouter(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	good := flowPacket()
	rng := rand.New(rand.NewSource(1))

	for n := range good {
		c.handle(routerAddr, good[:n])
	}
	for range 2000 {
		bad := append([]byte(nil), good...)
		for range 1 + rng.Intn(4) {
			bad[rng.Intn(len(bad))] = byte(rng.Intn(256))
		}
		c.handle(routerAddr, bad)
	}
	c.closeMinute(minute0.Add(time.Minute))
	c.handle(routerAddr, good)
	rows, _, _ := c.closeMinute(minute0.Add(2 * time.Minute))

	assert.NotZero(t, c.malformed.Load())
	require.Len(t, rows, 1)
	assert.EqualValues(t, 150_000, rows[0].Bytes)
}

// The decoder's bounds checks are what keep bad bytes out; the guard is for
// the bug nobody has found yet, which must cost one datagram, not the
// listener. A map without a POP table stands in for such a bug.
func TestHandleSafelyKeepsTheReadLoopAliveThroughAPanic(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	(*c.routers.Load())[routerAddr].POPs = nil

	assert.NotPanics(t, func() { c.handleSafely(routerAddr, flowPacket(), zap.NewNop()) })
	assert.EqualValues(t, 1, c.malformed.Load())
	assert.True(t, c.mu.TryLock(), "a panic must not leave the aggregator locked")
}

// Review Focus 4: a clock stepped back, or a tick that lands in the minute
// already closed, must not close it again: its rows would be stored twice.
// The flows stay in the open minute and go out with its close.
func TestCloseMinuteNeverClosesAMinuteTwice(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	c.handle(routerAddr, flowPacket())
	first, _, _ := c.closeMinute(minute0.Add(time.Minute))
	c.handle(routerAddr, flowPacket())

	again, againSeen, _ := c.closeMinute(minute0.Add(time.Minute + 30*time.Second))
	back, _, _ := c.closeMinute(minute0.Add(30 * time.Second))
	next, nextSeen, _ := c.closeMinute(minute0.Add(2 * time.Minute))

	require.Len(t, first, 1)
	assert.Equal(t, minute0, first[0].Time)
	assert.Empty(t, again)
	assert.Empty(t, againSeen)
	assert.Empty(t, back)
	require.Len(t, next, 1)
	assert.Equal(t, minute0.Add(time.Minute), next[0].Time)
	assert.EqualValues(t, 150_000, next[0].Bytes)
	assert.Equal(t, []uuid.UUID{testRouterID}, nextSeen)
}

func TestTopIPsCloseOnlyAtTheWindowBoundary(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	c.handle(routerAddr, flowPacket())

	assert.Empty(t, c.closeWindowIfDue(minute0.Add(time.Minute)))
	rows := c.closeWindowIfDue(minute0.Add(netflow.TopIPWindow))

	require.Len(t, rows, 1)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), rows[0].IP)
	assert.EqualValues(t, 64497, rows[0].ASN)
}

// The same guard for the top-IP window: a clock stepped back into an earlier
// window must not close the open one under a time already written.
func TestTopIPWindowNeverClosesTwice(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	c.handle(routerAddr, flowPacket())
	first := c.closeWindowIfDue(minute0.Add(netflow.TopIPWindow))
	c.handle(routerAddr, flowPacket())

	back := c.closeWindowIfDue(minute0.Add(time.Minute))
	next := c.closeWindowIfDue(minute0.Add(2 * netflow.TopIPWindow))

	require.Len(t, first, 1)
	assert.Equal(t, minute0, first[0].Time)
	assert.Empty(t, back)
	require.Len(t, next, 1)
	assert.Equal(t, minute0.Add(netflow.TopIPWindow), next[0].Time)
}
