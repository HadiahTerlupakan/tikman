package netflow

import (
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
	Bytes     uint64
	Packets   uint64
}

// Classify places a flow, or reports false for one that never crossed an
// upstream (POP to POP) or that the router itself sent or received: ifIndex 0
// on the far side means no interface, which is the router's own stack.
func (m *RouterMap) Classify(f Flow) (Classified, bool) {
	c := Classified{Bytes: f.Bytes * m.Sampling, Packets: f.Packets * m.Sampling}
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
	c.POP, _ = m.POPs.Lookup(local)
	return c, true
}
