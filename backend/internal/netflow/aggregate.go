package netflow

import (
	"math/bits"
	"net/netip"
	"sort"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxKeysPerRouter bounds one router's distinct keys in a minute, so a
	// flood of destinations cannot grow memory without limit.
	MaxKeysPerRouter = 50_000
	// MaxIPsPerPOP bounds the distinct far-side addresses counted per POP
	// per window.
	MaxIPsPerPOP = 100_000
	// MaxSubscriberPairs bounds the (address, subscriber) pairs kept per
	// window to count each address's distinct subscribers. Past it a new
	// pair is not counted, so Subscribers can only undercount.
	MaxSubscriberPairs = 1_000_000
	// TopASNs is how many ASNs per (router, POP, direction, minute) keep a
	// row of their own.
	TopASNs = 20
	// TopIPs is how many addresses per (router, POP, direction, window) are
	// stored.
	TopIPs = 100
	// OtherASN stands for both "no dataset range holds it" and "outside the
	// top": the UI calls it Lainnya.
	OtherASN = 0
	// TopIPWindow is the top-IP counters' window.
	TopIPWindow = 5 * time.Minute
)

// MinuteKey identifies one router_flow_minute row.
type MinuteKey struct {
	RouterID  uuid.UUID
	POP       uuid.UUID
	Upstream  uuid.UUID
	Direction Direction
	ASN       uint32
}

// MinuteRow is one closed minute's total for a key.
type MinuteRow struct {
	Time time.Time
	MinuteKey
	Bytes   uint64
	Packets uint64
}

// TopIPRow is one far-side address's total in a window.
type TopIPRow struct {
	Time      time.Time
	RouterID  uuid.UUID
	POP       uuid.UUID
	Direction Direction
	IP        netip.Addr
	ASN       uint32
	// Country is where IP is located, filled in by flowd from DB-IP.
	Country string
	Bytes   uint64
	// Flows is how many sampled flow records reached IP, a floor on its
	// connections: at 1:N sampling a short connection is often missed.
	Flows uint64
	// Subscribers is how many distinct local addresses those flows came from.
	Subscribers int
}

type counter struct{ bytes, packets uint64 }

type groupKey struct {
	RouterID  uuid.UUID
	POP       uuid.UUID
	Direction Direction
}

type popKey struct{ routerID, pop uuid.UUID }

type ipKey struct {
	groupKey
	ip netip.Addr
}

type ipCounter struct {
	asn         uint32
	bytes       uint64
	flows       uint64
	subscribers map[netip.Addr]struct{}
}

// Aggregator adds placed flows into the open minute and the open top-IP
// window. It is not safe for concurrent use; flowd guards it with a mutex.
type Aggregator struct {
	minute        time.Time
	keys          map[MinuteKey]*counter
	keysPerRouter map[uuid.UUID]int
	foldedFlows   map[uuid.UUID]uint64

	window          time.Time
	ips             map[ipKey]*ipCounter
	ipsPerPOP       map[popKey]int
	subscriberPairs int
}

// NewAggregator opens the minute and window holding now.
func NewAggregator(now time.Time) *Aggregator {
	a := &Aggregator{}
	a.resetMinute(now)
	a.resetWindow(now)
	return a
}

func (a *Aggregator) resetMinute(now time.Time) {
	a.minute = now.Truncate(time.Minute)
	a.keys = map[MinuteKey]*counter{}
	a.keysPerRouter = map[uuid.UUID]int{}
	a.foldedFlows = map[uuid.UUID]uint64{}
}

func (a *Aggregator) resetWindow(now time.Time) {
	a.window = now.Truncate(TopIPWindow)
	a.ips = map[ipKey]*ipCounter{}
	a.ipsPerPOP = map[popKey]int{}
	a.subscriberPairs = 0
}

// Add counts one placed flow whose far side is in asn.
func (a *Aggregator) Add(routerID uuid.UUID, c Classified, asn uint32) {
	a.addMinute(MinuteKey{RouterID: routerID, POP: c.POP, Upstream: c.Upstream, Direction: c.Direction, ASN: asn}, c)
	a.addIP(groupKey{RouterID: routerID, POP: c.POP, Direction: c.Direction}, c, asn)
}

// addMinute folds a new key into its OtherASN key once the router holds
// MaxKeysPerRouter keys this minute: totals stay exact, only the breakdown is
// lost. OtherASN keys are always admitted; there are at most two per POP and
// upstream.
func (a *Aggregator) addMinute(key MinuteKey, c Classified) {
	cnt, ok := a.keys[key]
	if !ok && key.ASN != OtherASN && a.keysPerRouter[key.RouterID] >= MaxKeysPerRouter {
		a.foldedFlows[key.RouterID]++
		key.ASN = OtherASN
		cnt, ok = a.keys[key]
	}
	if !ok {
		cnt = &counter{}
		a.keys[key] = cnt
		a.keysPerRouter[key.RouterID]++
	}
	cnt.bytes = addCount(cnt.bytes, c.Bytes)
	cnt.packets = addCount(cnt.packets, c.Packets)
}

// addIP ignores a new address once its POP holds MaxIPsPerPOP; the bytes
// are already in the minute totals.
func (a *Aggregator) addIP(g groupKey, c Classified, asn uint32) {
	key := ipKey{groupKey: g, ip: c.Remote}
	cnt, ok := a.ips[key]
	if !ok {
		pk := popKey{routerID: g.RouterID, pop: g.POP}
		if a.ipsPerPOP[pk] >= MaxIPsPerPOP {
			return
		}
		a.ipsPerPOP[pk]++
		cnt = &ipCounter{asn: asn, subscribers: map[netip.Addr]struct{}{}}
		a.ips[key] = cnt
	}
	cnt.bytes = addCount(cnt.bytes, c.Bytes)
	cnt.flows++
	if _, seen := cnt.subscribers[c.Local]; !seen && a.subscriberPairs < MaxSubscriberPairs {
		cnt.subscribers[c.Local] = struct{}{}
		a.subscriberPairs++
	}
}

// CloseMinute returns the open minute's rows, keeping TopASNs per (router,
// POP, direction) and folding the rest into OtherASN, with how many flows each
// router had folded into OtherASN by the key cap (flows, not distinct keys:
// every flow past the cap counts once). It then opens the minute holding now.
func (a *Aggregator) CloseMinute(now time.Time) ([]MinuteRow, map[uuid.UUID]uint64) {
	keep := topASNs(a.keys)
	merged := make(map[MinuteKey]*counter, len(a.keys))
	for key, cnt := range a.keys {
		if !keep[groupKey{RouterID: key.RouterID, POP: key.POP, Direction: key.Direction}][key.ASN] {
			key.ASN = OtherASN
		}
		addCounter(merged, key, cnt)
	}
	rows := make([]MinuteRow, 0, len(merged))
	for key, cnt := range merged {
		rows = append(rows, MinuteRow{Time: a.minute, MinuteKey: key, Bytes: cnt.bytes, Packets: cnt.packets})
	}
	folded := a.foldedFlows
	a.resetMinute(now)
	return rows, folded
}

func addCounter(into map[MinuteKey]*counter, key MinuteKey, cnt *counter) {
	sum, ok := into[key]
	if !ok {
		sum = &counter{}
		into[key] = sum
	}
	sum.bytes = addCount(sum.bytes, cnt.bytes)
	sum.packets = addCount(sum.packets, cnt.packets)
}

// addCount holds a sum at MaxCount. Every flow is within it, but nothing
// bounds how many records a sender spoofing a router's address can send, and
// a wrapped sum would be stored as a small or, past bigint, negative count.
// No real row reaches it, so only forged traffic is held there.
func addCount(sum, n uint64) uint64 {
	if total, carry := bits.Add64(sum, n, 0); carry == 0 && total < MaxCount {
		return total
	}
	return MaxCount
}

// topASNs ranks each group's ASNs by bytes summed over upstreams. OtherASN
// never takes one of the places.
func topASNs(keys map[MinuteKey]*counter) map[groupKey]map[uint32]bool {
	totals := map[groupKey]map[uint32]uint64{}
	for key, cnt := range keys {
		if key.ASN == OtherASN {
			continue
		}
		g := groupKey{RouterID: key.RouterID, POP: key.POP, Direction: key.Direction}
		if totals[g] == nil {
			totals[g] = map[uint32]uint64{}
		}
		totals[g][key.ASN] += cnt.bytes
	}
	keep := make(map[groupKey]map[uint32]bool, len(totals))
	for g, byASN := range totals {
		keep[g] = largestASNs(byASN, TopASNs)
	}
	return keep
}

func largestASNs(byASN map[uint32]uint64, n int) map[uint32]bool {
	asns := make([]uint32, 0, len(byASN))
	for asn := range byASN {
		asns = append(asns, asn)
	}
	sort.Slice(asns, func(i, j int) bool {
		if byASN[asns[i]] != byASN[asns[j]] {
			return byASN[asns[i]] > byASN[asns[j]]
		}
		return asns[i] < asns[j]
	})
	if len(asns) > n {
		asns = asns[:n]
	}
	set := make(map[uint32]bool, len(asns))
	for _, asn := range asns {
		set[asn] = true
	}
	return set
}

// WindowDue reports whether now has left the open top-IP window.
func (a *Aggregator) WindowDue(now time.Time) bool {
	return !now.Truncate(TopIPWindow).Equal(a.window)
}

// CloseWindow returns the TopIPs largest addresses per (router, POP,
// direction) of the open window and opens the window holding now.
func (a *Aggregator) CloseWindow(now time.Time) []TopIPRow {
	groups := map[groupKey][]TopIPRow{}
	for key, cnt := range a.ips {
		groups[key.groupKey] = append(groups[key.groupKey], TopIPRow{Time: a.window, RouterID: key.RouterID,
			POP: key.POP, Direction: key.Direction, IP: key.ip, ASN: cnt.asn, Bytes: cnt.bytes,
			Flows: cnt.flows, Subscribers: len(cnt.subscribers)})
	}
	var rows []TopIPRow
	for _, group := range groups {
		sort.Slice(group, func(i, j int) bool { return group[i].Bytes > group[j].Bytes })
		if len(group) > TopIPs {
			group = group[:TopIPs]
		}
		rows = append(rows, group...)
	}
	a.resetWindow(now)
	return rows
}
