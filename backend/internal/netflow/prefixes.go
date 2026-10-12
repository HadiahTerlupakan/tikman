package netflow

import (
	"net/netip"
	"sort"

	"github.com/google/uuid"
)

// PrefixEntry is one block a POP interface owns: an address on it, or a block
// routed statically to a device behind it.
type PrefixEntry struct {
	Prefix      netip.Prefix
	InterfaceID uuid.UUID
}

// PrefixTable finds the most specific block holding an address. Blocks have
// whatever length the operator gave each POP, so every length present is
// tried, longest first: at most 33 map lookups however many blocks exist.
type PrefixTable struct {
	byLength map[int]map[netip.Prefix]uuid.UUID
	lengths  []int
}

// NewPrefixTable indexes IPv4 entries; IPv6 ones are ignored. When two
// interfaces list the same block the first entry keeps it.
func NewPrefixTable(entries []PrefixEntry) *PrefixTable {
	t := &PrefixTable{byLength: map[int]map[netip.Prefix]uuid.UUID{}}
	for _, e := range entries {
		if !e.Prefix.Addr().Is4() {
			continue
		}
		p := e.Prefix.Masked()
		blocks, ok := t.byLength[p.Bits()]
		if !ok {
			blocks = map[netip.Prefix]uuid.UUID{}
			t.byLength[p.Bits()] = blocks
			t.lengths = append(t.lengths, p.Bits())
		}
		if _, taken := blocks[p]; !taken {
			blocks[p] = e.InterfaceID
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(t.lengths)))
	return t
}

// Lookup returns the interface owning the most specific block holding ip.
func (t *PrefixTable) Lookup(ip netip.Addr) (uuid.UUID, bool) {
	if !ip.Is4() {
		return uuid.Nil, false
	}
	for _, bits := range t.lengths {
		p, err := ip.Prefix(bits)
		if err != nil {
			continue
		}
		if id, ok := t.byLength[bits][p]; ok {
			return id, true
		}
	}
	return uuid.Nil, false
}
