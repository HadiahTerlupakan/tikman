package main

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// collector turns datagrams into aggregator counts. handle runs on the one
// goroutine reading the socket, which alone touches templates; the
// aggregator is shared with the minute ticker, hence the mutex.
type collector struct {
	routers   atomic.Pointer[map[netip.Addr]*netflow.RouterMap]
	asns      *asnSource
	templates map[uuid.UUID]*netflow.Templates

	mu         sync.Mutex
	agg        *netflow.Aggregator
	seen       map[uuid.UUID]bool
	openMinute time.Time
	openWindow time.Time

	unknownSources atomic.Uint64
	malformed      atomic.Uint64
}

func newCollector(asns *asnSource, now time.Time) *collector {
	c := &collector{asns: asns, templates: map[uuid.UUID]*netflow.Templates{},
		agg: netflow.NewAggregator(now), seen: map[uuid.UUID]bool{},
		openMinute: now.Truncate(time.Minute), openWindow: now.Truncate(netflow.TopIPWindow)}
	c.setRouters(map[netip.Addr]*netflow.RouterMap{})
	return c
}

// setRouters swaps in a freshly loaded mapping; a datagram being handled
// keeps the one it started with.
func (c *collector) setRouters(routers map[netip.Addr]*netflow.RouterMap) {
	c.routers.Store(&routers)
}

// handle decodes one datagram from src. Anything not from a router collecting
// flows is dropped before parsing: the port is open to the internet.
func (c *collector) handle(src netip.Addr, packet []byte) {
	router, ok := (*c.routers.Load())[src.Unmap()]
	if !ok {
		c.unknownSources.Add(1)
		return
	}
	res, err := c.templatesFor(router.RouterID).Decode(packet)
	if err != nil {
		c.malformed.Add(1)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.seen[router.RouterID] = true
	}
	for _, flow := range res.Flows {
		if placed, ok := router.Classify(flow); ok {
			c.agg.Add(router.RouterID, placed, c.asns.lookup(placed.Remote))
		}
	}
}

// handleSafely is handle for the read loop. The decoder bounds-checks every
// length it reads; this only makes a bug it still has cost one datagram
// instead of the listener, since one goroutine reads for every router.
func (c *collector) handleSafely(src netip.Addr, packet []byte, logger *zap.Logger) {
	defer func() {
		if r := recover(); r != nil {
			c.malformed.Add(1)
			logger.Error("Recovered from a panic handling an IPFIX datagram",
				zap.Any("panic", r), zap.String("source", src.String()), zap.Int("bytes", len(packet)))
		}
	}()
	c.handle(src, packet)
}

// templatesFor keeps each router's templates across mapping reloads, keyed by
// router rather than address, so a reload does not forget them.
func (c *collector) templatesFor(routerID uuid.UUID) *netflow.Templates {
	t, ok := c.templates[routerID]
	if !ok {
		t = netflow.NewTemplates()
		c.templates[routerID] = t
	}
	return t
}

// closeMinute returns the closed minute's rows, the routers heard from in it,
// and the keys each router had to fold for the cap. A now that is not past
// the open minute closes nothing: the Aggregator would close it again under
// a Time already stored, so the flows wait for the open minute's own close.
// That happens when the clock steps back or a tick lands early.
func (c *collector) closeMinute(now time.Time) ([]netflow.MinuteRow, []uuid.UUID, map[uuid.UUID]uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !now.Truncate(time.Minute).After(c.openMinute) {
		return nil, nil, nil
	}
	c.openMinute = now.Truncate(time.Minute)
	rows, dropped := c.agg.CloseMinute(now)
	seen := make([]uuid.UUID, 0, len(c.seen))
	for id := range c.seen {
		seen = append(seen, id)
	}
	c.seen = map[uuid.UUID]bool{}
	return rows, seen, dropped
}

// closeWindowIfDue returns the top-IP rows once now is past their window,
// with the same guard as closeMinute: only a later window closes the open one.
func (c *collector) closeWindowIfDue(now time.Time) []netflow.TopIPRow {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !now.Truncate(netflow.TopIPWindow).After(c.openWindow) {
		return nil
	}
	c.openWindow = now.Truncate(netflow.TopIPWindow)
	return c.agg.CloseWindow(now)
}
