package main

import (
	"math"
	"math/rand"
	"net/http"
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
	"go.uber.org/zap/zaptest/observer"
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
	geo := &geoSource{logger: zap.NewNop()}
	geoTable, err := netflow.ParseGeoTable(strings.NewReader(
		"network,country,country_code,continent,continent_code,asn,as_name,as_domain\n" +
			"198.51.100.0/24,Japan,JP,Asia,AS,,,\n"))
	require.NoError(t, err)
	geo.table.Store(geoTable)
	anon := newAnonymitySource(t.TempDir(), http.DefaultClient, zap.NewNop())
	tor, err := netflow.ParseIPSet(strings.NewReader("198.51.100.7\n"))
	require.NoError(t, err)
	anon.tor.set.Store(tor)
	c := newCollector(asns, geo, anon, minute0)
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
	rows, seen, folded := c.closeMinute(minute0.Add(time.Minute))

	require.Len(t, rows, 1)
	assert.Equal(t, netflow.MinuteRow{Time: minute0, MinuteKey: netflow.MinuteKey{RouterID: testRouterID,
		POP: testPOP, Upstream: testTransit, Direction: netflow.Download, ASN: 64497},
		Bytes: 150_000, Packets: 300}, rows[0])
	assert.Equal(t, []uuid.UUID{testRouterID}, seen)
	assert.Empty(t, folded)
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

// A router whose flows keep tripping a bug would otherwise write an error
// line per datagram; the malformed counter already carries the total.
func TestHandleSafelyLogsOnePanicPerInterval(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	(*c.routers.Load())[routerAddr].POPs = nil
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	c.handleSafely(routerAddr, flowPacket(), logger)
	c.handleSafely(routerAddr, flowPacket(), logger)
	assert.Equal(t, 1, logs.Len())
	c.panicLoggedAt = c.panicLoggedAt.Add(-panicLogInterval)
	c.handleSafely(routerAddr, flowPacket(), logger)

	assert.Equal(t, 2, logs.Len())
	assert.EqualValues(t, 3, c.malformed.Load())
	assert.Equal(t, routerAddr.String(), logs.All()[0].ContextMap()["source"])
}

func dataOnlyPacket() []byte {
	return ipfixtest.Message(1,
		ipfixtest.DataSet(256, ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)))
}

// A router dropped from the mapping takes its templates with it, so removed
// or renumbered routers do not accumulate state; one that stays keeps them
// and still decodes data sent without a fresh template.
func TestReloadPrunesTemplatesOfRemovedRoutersOnly(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	otherAddr := netip.MustParseAddr("192.0.2.2")
	otherID := uuid.MustParse("00000000-0000-0000-0000-0000000000c2")
	other := &netflow.RouterMap{RouterID: otherID, Sampling: 1, POPs: netflow.NewPrefixTable(nil)}
	kept := *c.routers.Load()
	c.handle(routerAddr, flowPacket())
	c.setRouters(map[netip.Addr]*netflow.RouterMap{otherAddr: other})
	c.handle(otherAddr, flowPacket())

	assert.NotContains(t, c.templates, testRouterID)
	assert.Contains(t, c.templates, otherID)

	c.setRouters(map[netip.Addr]*netflow.RouterMap{routerAddr: kept[routerAddr], otherAddr: other})
	c.handle(otherAddr, dataOnlyPacket())
	c.handle(routerAddr, flowPacket())
	c.closeMinute(minute0.Add(time.Minute))
	c.setRouters(map[netip.Addr]*netflow.RouterMap{routerAddr: kept[routerAddr]})
	c.handle(routerAddr, dataOnlyPacket())
	rows, _, _ := c.closeMinute(minute0.Add(2 * time.Minute))

	assert.NotContains(t, c.templates, otherID)
	require.Len(t, rows, 1)
	assert.EqualValues(t, 150_000, rows[0].Bytes)
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
	assert.Equal(t, "JP", rows[0].Country, "located from the country dataset")
	assert.Equal(t, "tor", rows[0].Anonymity, "flagged from the anonymity lists")
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

func packetOf(records ...[]byte) []byte {
	return ipfixtest.Message(1, ipfixtest.TemplateSet(256, ipfixtest.FlowFields...),
		ipfixtest.DataSet(256, records...))
}

// A record whose counts would wrap once scaled is dropped and counted; added
// in, it would have wrapped the real flow's row beside it to a small count.
func TestHandleDropsAndCountsImplausibleRecords(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})

	c.handle(routerAddr, packetOf(
		ipfixtest.FlowRecord("198.51.100.9", "203.0.113.6", 10, 20, math.MaxUint64/50, 1),
		ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)))
	rows, _, _ := c.closeMinute(minute0.Add(time.Minute))

	require.Len(t, rows, 1)
	assert.EqualValues(t, 150_000, rows[0].Bytes)
	assert.EqualValues(t, 300, rows[0].Packets)
	assert.EqualValues(t, 1, c.implausible.Load())
}

// Records each within the bounds share no allowance with the real flows: a
// forger at the bounds, under another key, leaves every real flow counted.
func TestForgedRecordsAtTheBoundsLeaveRealFlowsCounted(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	c.handle(routerAddr, packetOf(
		ipfixtest.FlowRecord("192.0.2.99", "203.0.113.6", 10, 20, 0, (1<<43)/100),
		ipfixtest.FlowRecord("192.0.2.99", "203.0.113.6", 10, 20, netflow.MaxCount/100, 1)))

	for range 1000 {
		c.handle(routerAddr, flowPacket())
	}
	rows, _, _ := c.closeMinute(minute0.Add(time.Minute))

	require.Len(t, rows, 2)
	for _, r := range rows {
		if r.ASN == 64497 {
			assert.EqualValues(t, 1000*150_000, r.Bytes)
			assert.EqualValues(t, 1000*300, r.Packets)
		}
	}
	assert.Zero(t, c.implausible.Load())
}

// Dropped records are warned of with the minute, as folded keys are, and
// each minute reports only its own.
func TestFinishMinuteWarnsOfDroppedRecords(t *testing.T) {
	c := testCollector(t, map[uint32]uuid.UUID{10: testTransit})
	forged := ipfixtest.FlowRecord("198.51.100.9", "203.0.113.6", 10, 20, netflow.MaxCount, 1)
	c.handle(routerAddr, packetOf(forged, forged))
	core, logs := observer.New(zap.WarnLevel)
	sink := &fakeSink{}
	w := &retryWriter{sink: sink, logger: zap.NewNop()}

	finishMinute(c, w, sink, zap.New(core), minute0.Add(time.Minute))
	finishMinute(c, w, sink, zap.New(core), minute0.Add(2*time.Minute))

	require.Equal(t, 1, logs.Len())
	assert.EqualValues(t, 2, logs.All()[0].ContextMap()["dropped_flows"])
}
