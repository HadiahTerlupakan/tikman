package netflow

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"sort"
	"strings"
)

// maxIPSetFile bounds a list file, a few hundred KB today.
const maxIPSetFile = 64 << 20

var errIPSetTooLarge = errors.New("address list is larger than its size limit")

type ipv4Range struct{ start, end uint32 }

// IPSet is a list of IPv4 addresses and networks, merged into disjoint
// sorted ranges. The lists it reads (Tor exits, VPN and datacenter networks)
// overlap themselves, which a plain binary search would get wrong.
type IPSet struct {
	ranges []ipv4Range
}

// ParseIPSet reads one address or CIDR network per line. Blank lines and
// lines starting with # are skipped, as are IPv6 entries, since flows are
// IPv4 only. Any other malformed line fails the whole list.
func ParseIPSet(r io.Reader) (*IPSet, error) {
	var ranges []ipv4Range
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") || strings.Contains(text, ":") {
			continue
		}
		prefix, err := parseNetwork(text)
		if err != nil || !prefix.Addr().Is4() {
			return nil, fmt.Errorf("address list line %d: %q is not an IPv4 address or network", line, text)
		}
		ranges = append(ranges, prefixRange(prefix))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(ranges) == 0 {
		return nil, errors.New("address list has no IPv4 entries")
	}
	return &IPSet{ranges: mergeRanges(ranges)}, nil
}

func prefixRange(prefix netip.Prefix) ipv4Range {
	b := prefix.Masked().Addr().As4()
	start := binary.BigEndian.Uint32(b[:])
	return ipv4Range{start: start, end: start | (1<<(32-prefix.Bits()) - 1)}
}

// mergeRanges sorts the ranges and joins those that overlap or touch.
func mergeRanges(ranges []ipv4Range) []ipv4Range {
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if last.end == ^uint32(0) || r.start <= last.end+1 {
			last.end = max(last.end, r.end)
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// Contains reports whether ip is in the list.
func (s *IPSet) Contains(ip netip.Addr) bool {
	if s == nil || !ip.Is4() {
		return false
	}
	b := ip.As4()
	v := binary.BigEndian.Uint32(b[:])
	i := sort.Search(len(s.ranges), func(i int) bool { return s.ranges[i].start > v }) - 1
	return i >= 0 && v <= s.ranges[i].end
}

// LoadIPSetFile reads a plain-text list from disk.
func LoadIPSetFile(path string) (*IPSet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return ParseIPSet(&cappedReader{r: file, left: maxIPSetFile, err: errIPSetTooLarge})
}
