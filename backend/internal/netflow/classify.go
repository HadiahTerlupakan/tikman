package netflow

import (
	"math/bits"
	"net/netip"

	"github.com/google/uuid"
)

// Direction is which way traffic crossed the upstream.
type Direction string

// Each packet is counted at the interface it entered: in from an upstream is
// a download, in from a POP and out to an upstream is an upload.
const (
	Download Direction = "download"
	Upload   Direction = "upload"
)

const (
	// MaxCount bounds every byte and packet count flowd keeps: a flow
	// record's once scaled by sampling, and a minute row's or top-IP
	// window's sum. 2^52 bytes is more than a 400 Gbps port carries in a
	// day, so no real record or row reaches it, not even a minute row that
	// gathered a whole day while a clock correction stalled closes. 1440
	// rows at it sum to under bigint's maximum, so one key's day of minutes
	// cannot fail the daily rollup's SUM(bytes)::bigint.
	MaxCount = 1 << 52
	// maxFlowPackets is as many packets as MaxCount bytes can hold, each at
	// least an IPv4 header's 20 bytes.
	maxFlowPackets = MaxCount / 20
)

// RouterMap is what flowd knows about one exporting router.
type RouterMap struct {
	RouterID uuid.UUID
	// Sampling is the router's 1:N packet sampling; counts are scaled by it.
	Sampling uint64
	// Upstreams maps the ifIndex of each BGP-facing interface to its ID.
	Upstreams map[uint32]uuid.UUID
	// POPs holds the blocks of every interface that is not an upstream.
	POPs *PrefixTable
}

// Classified is a flow placed on the router's topology. POP is uuid.Nil when
// the local address is in no POP's blocks ("Lainnya").
type Classified struct {
	POP       uuid.UUID
	Upstream  uuid.UUID
	Direction Direction
	Remote    netip.Addr
	// Local is the subscriber's side of the flow.
	Local   netip.Addr
	Bytes   uint64
	Packets uint64
}

// Plausible reports whether a flow's counts, scaled by sampling, are ones an
// IPv4 flow can have: at most MaxCount bytes and maxFlowPackets packets. They
// arrive on an unauthenticated UDP port, so a sender can put anything up to
// 2^64 in them, which scaling would wrap.
func (m *RouterMap) Plausible(f Flow) bool {
	_, _, ok := m.scale(f)
	return ok
}

func (m *RouterMap) scale(f Flow) (bytes, packets uint64, ok bool) {
	bytes, bytesOK := scaled(f.Bytes, m.Sampling, MaxCount)
	packets, packetsOK := scaled(f.Packets, m.Sampling, maxFlowPackets)
	return bytes, packets, bytesOK && packetsOK
}

func scaled(n, sampling, limit uint64) (uint64, bool) {
	hi, lo := bits.Mul64(n, sampling)
	return lo, hi == 0 && lo <= limit
}

// Classify places a flow, or reports false for one whose counts are not
// Plausible, that never crossed an upstream (POP to POP) or that the router
// itself sent or received: ifIndex 0 on the far side means no interface,
// which is the router's own stack.
func (m *RouterMap) Classify(f Flow) (Classified, bool) {
	bytes, packets, ok := m.scale(f)
	if !ok {
		return Classified{}, false
	}
	c := Classified{Bytes: bytes, Packets: packets}
	var local netip.Addr
	var otherSide uint32
	if up, ok := m.Upstreams[f.Ingress]; ok {
		c.Upstream, c.Direction, local, c.Remote, otherSide = up, Download, f.Dst, f.Src, f.Egress
	} else if up, ok := m.Upstreams[f.Egress]; ok {
		c.Upstream, c.Direction, local, c.Remote, otherSide = up, Upload, f.Src, f.Dst, f.Ingress
	} else {
		return Classified{}, false
	}
	if otherSide == 0 {
		return Classified{}, false
	}
	c.Local = local
	c.POP, _ = m.POPs.Lookup(local)
	return c, true
}
