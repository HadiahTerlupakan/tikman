package netflow

import (
	"math"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	popA     = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")
	popB     = uuid.MustParse("00000000-0000-0000-0000-0000000000a2")
	popWide  = uuid.MustParse("00000000-0000-0000-0000-0000000000a3")
	transit  = uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	ix       = uuid.MustParse("00000000-0000-0000-0000-0000000000b2")
	routerID = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")
)

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func testPrefixes() *PrefixTable {
	return NewPrefixTable([]PrefixEntry{
		// An interface address: host bits set, as /ip/address stores it.
		{Prefix: netip.MustParsePrefix("192.0.2.33/29"), InterfaceID: popA},
		// A second block on the same interface.
		{Prefix: netip.MustParsePrefix("192.0.2.57/30"), InterfaceID: popA},
		{Prefix: netip.MustParsePrefix("203.0.113.0/24"), InterfaceID: popWide},
		// A block routed statically to a POP router, inside the /24.
		{Prefix: netip.MustParsePrefix("203.0.113.64/27"), InterfaceID: popB},
		{Prefix: netip.MustParsePrefix("2001:db8::/32"), InterfaceID: popB},
	})
}

func TestLongestPrefixWinsWhateverTheLengths(t *testing.T) {
	table := testPrefixes()
	cases := map[string]uuid.UUID{
		"192.0.2.38":   popA,     // the /29
		"192.0.2.58":   popA,     // the /30
		"192.0.2.60":   uuid.Nil, // just past the /30
		"203.0.113.70": popB,     // the /27 beats the /24 holding it
		"203.0.113.96": popWide,  // only the /24
		"2001:db8::1":  uuid.Nil, // IPv4 only
	}
	for ip, want := range cases {
		got, ok := table.Lookup(addr(ip))
		assert.Equal(t, want, got, ip)
		assert.Equal(t, want != uuid.Nil, ok, ip)
	}
}

func testRouterMap() *RouterMap {
	return &RouterMap{RouterID: routerID, Sampling: 100,
		Upstreams: map[uint32]uuid.UUID{10: transit, 11: ix}, POPs: testPrefixes()}
}

func TestClassifyPlacesDownloadsAndUploads(t *testing.T) {
	m := testRouterMap()

	down, ok := m.Classify(Flow{Src: addr("198.51.100.7"), Dst: addr("203.0.113.70"),
		Ingress: 10, Egress: 30, Bytes: 1500, Packets: 1})
	require.True(t, ok)
	assert.Equal(t, Classified{POP: popB, Upstream: transit, Direction: Download,
		Remote: addr("198.51.100.7"), Bytes: 150_000, Packets: 100}, down)

	up, ok := m.Classify(Flow{Src: addr("192.0.2.38"), Dst: addr("198.51.100.9"),
		Ingress: 30, Egress: 11, Bytes: 80, Packets: 1})
	require.True(t, ok)
	assert.Equal(t, Classified{POP: popA, Upstream: ix, Direction: Upload,
		Remote: addr("198.51.100.9"), Bytes: 8000, Packets: 100}, up)
}

func TestClassifyDropsTrafficThatNeverCrossesAnUpstream(t *testing.T) {
	m := testRouterMap()
	cases := map[string]Flow{
		"POP to POP":             {Src: addr("192.0.2.38"), Dst: addr("203.0.113.70"), Ingress: 30, Egress: 31, Bytes: 1},
		"to the router itself":   {Src: addr("198.51.100.7"), Dst: addr("198.51.100.0"), Ingress: 10, Egress: 0, Bytes: 1},
		"from the router itself": {Src: addr("198.51.100.0"), Dst: addr("198.51.100.7"), Ingress: 0, Egress: 10, Bytes: 1},
	}
	for name, f := range cases {
		_, ok := m.Classify(f)
		assert.False(t, ok, name)
	}
}

// Review Focus 3: before discovery has stored a POP's blocks, its traffic is
// still counted, under "Lainnya", rather than lost.
func TestClassifyCountsAnAddressInNoPOPAsOther(t *testing.T) {
	c, ok := testRouterMap().Classify(Flow{Src: addr("198.51.100.7"), Dst: addr("192.0.2.200"),
		Ingress: 10, Egress: 30, Bytes: 10, Packets: 1})

	require.True(t, ok)
	assert.Equal(t, uuid.Nil, c.POP)
	assert.Equal(t, transit, c.Upstream)
}

// The counts come off an unauthenticated UDP port: one that scaling would
// wrap, or past what any IPv4 flow could carry, is refused rather than stored.
func TestClassifyRefusesCountsNoIPv4FlowCanHave(t *testing.T) {
	m := testRouterMap()
	base := Flow{Src: addr("198.51.100.7"), Dst: addr("203.0.113.70"), Ingress: 10, Egress: 30}
	cases := map[string]struct{ bytes, packets uint64 }{
		"bytes that wrap when scaled":     {bytes: math.MaxUint64 / 50, packets: 1},
		"bytes over MaxCount once scaled": {bytes: MaxCount/100 + 1, packets: 1},
		"packets that wrap when scaled":   {bytes: 1500, packets: math.MaxUint64 / 50},
		"more packets than MaxCount fits": {bytes: 1500, packets: maxFlowPackets/100 + 1},
	}
	for name, counts := range cases {
		f := base
		f.Bytes, f.Packets = counts.bytes, counts.packets
		_, ok := m.Classify(f)
		assert.False(t, ok, name)
		assert.False(t, m.Plausible(f), name)
	}
}

// The bounds sit above anything real: a record at MaxCount once scaled is
// kept, as is one from a template without packetDeltaCount.
func TestClassifyKeepsCountsUpToTheBounds(t *testing.T) {
	m := testRouterMap()
	cases := map[string]struct{ bytes, packets, wantBytes uint64 }{
		"bytes at MaxCount once scaled": {bytes: MaxCount / 100, packets: 1, wantBytes: MaxCount / 100 * 100},
		"packets at the bound":          {bytes: MaxCount / 100, packets: maxFlowPackets / 100, wantBytes: MaxCount / 100 * 100},
		"no packet count exported":      {bytes: 1500, packets: 0, wantBytes: 150_000},
	}
	for name, counts := range cases {
		f := Flow{Src: addr("198.51.100.7"), Dst: addr("203.0.113.70"), Ingress: 10, Egress: 30,
			Bytes: counts.bytes, Packets: counts.packets}
		c, ok := m.Classify(f)
		require.True(t, ok, name)
		assert.True(t, m.Plausible(f), name)
		assert.Equal(t, counts.wantBytes, c.Bytes, name)
		assert.Equal(t, counts.packets*100, c.Packets, name)
	}
}

func TestClassifyTagsTheMostSpecificOwnPrefix(t *testing.T) {
	m := testRouterMap()
	m.Own = []netip.Prefix{netip.MustParsePrefix("203.0.112.0/23"), netip.MustParsePrefix("203.0.113.0/24")}

	in, ok := m.Classify(Flow{Src: addr("198.51.100.7"), Dst: addr("203.0.113.70"), Ingress: 10, Egress: 30, Bytes: 1, Packets: 1})
	require.True(t, ok)
	assert.Equal(t, netip.MustParsePrefix("203.0.113.0/24"), in.OwnPrefix)

	out, ok := m.Classify(Flow{Src: addr("192.0.2.38"), Dst: addr("198.51.100.9"), Ingress: 30, Egress: 11, Bytes: 1, Packets: 1})
	require.True(t, ok)
	assert.False(t, out.OwnPrefix.IsValid(), "an address outside the announced space")
}
