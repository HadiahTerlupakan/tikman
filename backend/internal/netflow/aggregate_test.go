package netflow

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	routerA = uuid.MustParse("00000000-0000-0000-0000-0000000000d1")
	routerB = uuid.MustParse("00000000-0000-0000-0000-0000000000d2")
	minute0 = time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
)

// docASN returns the i-th documentation AS number (RFC 5398): 64496-64511,
// then 65536-65551.
func docASN(i int) uint32 {
	if i < 16 {
		return 64496 + uint32(i)
	}
	return 65536 + uint32(i-16)
}

func download(pop, upstream uuid.UUID, remote string, bytes uint64) Classified {
	return Classified{POP: pop, Upstream: upstream, Direction: Download,
		Remote: netip.MustParseAddr(remote), Bytes: bytes, Packets: 1}
}

func sumBytes(rows []MinuteRow) uint64 {
	var total uint64
	for _, r := range rows {
		total += r.Bytes
	}
	return total
}

func asnsUnder(rows []MinuteRow, pop uuid.UUID) []uint32 {
	var asns []uint32
	for _, r := range rows {
		if r.POP == pop {
			asns = append(asns, r.ASN)
		}
	}
	return asns
}

func TestCloseMinuteKeepsTheTopASNsAndPreservesTheTotal(t *testing.T) {
	agg := NewAggregator(minute0.Add(10 * time.Second))
	var total uint64
	for i := 0; i < TopASNs+5; i++ {
		bytes := uint64(i+1) * 1000
		// Split over two upstreams: the ranking is per ASN, not per row.
		agg.Add(routerA, download(popA, transit, "198.51.100.7", bytes), docASN(i))
		agg.Add(routerA, download(popA, ix, "198.51.100.8", bytes), docASN(i))
		total += 2 * bytes
	}

	rows, dropped := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Empty(t, dropped)
	assert.Equal(t, total, sumBytes(rows), "folding into ASN 0 loses no bytes")
	kept := map[uint32]bool{}
	var other uint64
	for _, r := range rows {
		assert.Equal(t, minute0, r.Time)
		if r.ASN == OtherASN {
			other += r.Bytes
			continue
		}
		kept[r.ASN] = true
	}
	assert.Len(t, kept, TopASNs)
	for i := 0; i < 5; i++ {
		assert.False(t, kept[docASN(i)], "the five smallest fold into ASN 0")
	}
	assert.Equal(t, uint64(2*(1+2+3+4+5)*1000), other)
	assert.Len(t, rows, 2*(TopASNs+1), "one row per upstream for each kept ASN and for ASN 0")
}

// Each POP has its own top: a small destination of a quiet POP is not
// crowded out by a busy POP's.
func TestTopASNsAreRankedPerPOP(t *testing.T) {
	agg := NewAggregator(minute0)
	for i := 0; i < TopASNs; i++ {
		agg.Add(routerA, download(popA, transit, "198.51.100.7", 1_000_000), docASN(i))
	}
	agg.Add(routerA, download(popB, transit, "198.51.100.7", 10), docASN(TopASNs))

	rows, _ := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Equal(t, []uint32{docASN(TopASNs)}, asnsUnder(rows, popB))
}

func TestKeyCapFoldsNewKeysIntoOtherPerRouter(t *testing.T) {
	agg := NewAggregator(minute0)
	pops := make([]uuid.UUID, MaxKeysPerRouter)
	for i := range pops {
		pops[i] = uuid.New()
		agg.Add(routerA, download(pops[i], transit, "198.51.100.7", 1), docASN(0))
	}
	agg.Add(routerA, download(pops[0], transit, "198.51.100.7", 5), docASN(1))
	agg.Add(routerB, download(popA, transit, "198.51.100.7", 7), docASN(1))

	rows, dropped := agg.CloseMinute(minute0.Add(time.Minute))

	assert.Equal(t, map[uuid.UUID]uint64{routerA: 1}, dropped)
	assert.Equal(t, uint64(MaxKeysPerRouter+5+7), sumBytes(rows))
	assert.Contains(t, rows, MinuteRow{Time: minute0, MinuteKey: MinuteKey{RouterID: routerA, POP: pops[0],
		Upstream: transit, Direction: Download, ASN: OtherASN}, Bytes: 5, Packets: 1})
	assert.Equal(t, []uint32{docASN(1)}, asnsUnder(rows, popA), "another router's keys are not capped by this one's")
}

func TestCloseWindowKeepsTheHundredLargestAddresses(t *testing.T) {
	agg := NewAggregator(minute0)
	for i := 0; i < 150; i++ {
		agg.Add(routerA, download(popA, transit, fmt.Sprintf("203.0.113.%d", i), uint64(i+1)), docASN(0))
	}
	assert.False(t, agg.WindowDue(minute0.Add(4*time.Minute)))
	require.True(t, agg.WindowDue(minute0.Add(TopIPWindow)))

	rows := agg.CloseWindow(minute0.Add(TopIPWindow))

	require.Len(t, rows, TopIPs)
	for _, r := range rows {
		assert.Greater(t, r.Bytes, uint64(50), "only the hundred largest are kept")
		assert.Equal(t, minute0, r.Time)
		assert.Equal(t, docASN(0), r.ASN)
	}
	assert.False(t, agg.WindowDue(minute0.Add(TopIPWindow+time.Minute)), "the next window is open")
}

// Past the cap a new address is not tracked, however large; its bytes still
// count in the minute totals.
func TestIPCounterStopsAtTheCapPerPOP(t *testing.T) {
	agg := NewAggregator(minute0)
	remote := netip.MustParseAddr("2001:db8::")
	for i := 0; i < MaxIPsPerPOP; i++ {
		agg.Add(routerA, Classified{POP: popA, Upstream: transit, Direction: Download, Remote: remote, Bytes: 1}, 0)
		remote = remote.Next()
	}
	late := netip.MustParseAddr("2001:db8:ffff::1")
	agg.Add(routerA, Classified{POP: popA, Upstream: transit, Direction: Download, Remote: late, Bytes: 1 << 40}, 0)

	for _, r := range agg.CloseWindow(minute0.Add(TopIPWindow)) {
		assert.NotEqual(t, late, r.IP)
	}
	minute, _ := agg.CloseMinute(minute0.Add(time.Minute))
	assert.Equal(t, uint64(MaxIPsPerPOP)+1<<40, sumBytes(minute))
}
