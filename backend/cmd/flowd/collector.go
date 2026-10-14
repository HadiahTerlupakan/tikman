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

// panicLogInterval is how often a recovered panic is logged, matching how
// often refreshRouters reports the malformed count.
const panicLogInterval = mappingRefresh

// collector turns datagrams into aggregator counts. handle runs on the one
// goroutine reading the socket, which alone touches templates, prunedFor and
// panicLoggedAt; the aggregator is shared with the minute ticker, hence the
// mutex.
type collector struct {
	routers       atomic.Pointer[map[netip.Addr]*netflow.RouterMap]
	asns          *asnSource
	geo           *geoSource
	anon          *anonymitySource
	templates     map[uuid.UUID]*netflow.Templates
	prunedFor     *map[netip.Addr]*netflow.RouterMap
	panicLoggedAt time.Time

	mu         sync.Mutex
	agg        *netflow.Aggregator
	seen       map[uuid.UUID]bool
	openMinute time.Time
	openWindow time.Time

	unknownSources atomic.Uint64
	malformed      atomic.Uint64
	// implausible counts records dropped for counts no IPv4 flow can have.
	implausible atomic.Uint64
}

func newCollector(asns *asnSource, geo *geoSource, anon *anonymitySource, now time.Time) *collector {
	c := &collector{asns: asns, geo: geo, anon: anon, templates: map[uuid.UUID]*netflow.Templates{},
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
	routers := c.routers.Load()
	if routers != c.prunedFor {
		c.pruneTemplates(*routers)
		c.prunedFor = routers
	}
	router, ok := (*routers)[src.Unmap()]
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
		if !router.Plausible(flow) {
			c.implausible.Add(1)
			continue
		}
		if placed, ok := router.Classify(flow); ok {
			c.agg.Add(router.RouterID, placed, c.asns.lookup(placed.Remote))
		}
	}
}

// handleSafely is handle for the read loop. The decoder bounds-checks every
// length it reads; this only makes a bug it still has cost one datagram
// instead of the listener, since one goroutine reads for every router.
// Only the first panic per panicLogInterval is logged: a router that keeps
// tripping the same bug would otherwise write a line per datagram, and the
// malformed counter refreshRouters reports already carries the rest.
func (c *collector) handleSafely(src netip.Addr, packet []byte, logger *zap.Logger) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		c.malformed.Add(1)
		if time.Since(c.panicLoggedAt) < panicLogInterval {
			return
		}
		c.panicLoggedAt = time.Now()
		logger.Error("Recovered from a panic handling an IPFIX datagram; further ones this interval are only counted",
			zap.Any("panic", r), zap.String("source", src.String()), zap.Int("bytes", len(packet)))
	}()
	c.handle(src, packet)
}

// pruneTemplates forgets the templates of routers no longer collecting flows,
// so routers removed or disabled over months leave nothing behind. It runs on
// the read goroutine, which owns templates, the first time a datagram meets a
// newly loaded mapping.
func (c *collector) pruneTemplates(routers map[netip.Addr]*netflow.RouterMap) {
	current := make(map[uuid.UUID]bool, len(routers))
	for _, m := range routers {
		current[m.RouterID] = true
	}
	for id := range c.templates {
		if !current[id] {
			delete(c.templates, id)
		}
	}
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
// That happens when the clock steps back or a tick lands early. A clock that
// ran ahead and is then corrected stalls closes for as long as it was ahead:
// everything received meanwhile lands in one later row. Totals stay exact;
// only their placement in time is coarser for that stretch.
func (c *collector) closeMinute(now time.Time) ([]netflow.MinuteRow, []uuid.UUID, map[uuid.UUID]uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !now.Truncate(time.Minute).After(c.openMinute) {
		return nil, nil, nil
	}
	c.openMinute = now.Truncate(time.Minute)
	rows, folded := c.agg.CloseMinute(now)
	seen := make([]uuid.UUID, 0, len(c.seen))
	for id := range c.seen {
		seen = append(seen, id)
	}
	c.seen = map[uuid.UUID]bool{}
	return rows, seen, folded
}

// closeWindowIfDue returns the window's rows once now is past it, with the
// same guard as closeMinute: only a later window closes the open one.
func (c *collector) closeWindowIfDue(now time.Time) netflow.WindowRows {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !now.Truncate(netflow.TopIPWindow).After(c.openWindow) {
		return netflow.WindowRows{}
	}
	c.openWindow = now.Truncate(netflow.TopIPWindow)
	window := c.agg.CloseWindow(now)
	// Located once per kept address rather than per flow: a window keeps only
	// its top addresses, so most flows' addresses are never written.
	for i := range window.TopIPs {
		window.TopIPs[i].Country = c.geo.lookup(window.TopIPs[i].IP)
		window.TopIPs[i].Anonymity = c.anon.classify(window.TopIPs[i].IP)
	}
	return window
}
