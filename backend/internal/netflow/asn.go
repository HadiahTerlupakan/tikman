package netflow

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
)

// asnColumns is start, end, AS number, country, description.
const asnColumns = 5

// maxASNInflated bounds the decompressed dataset, a few tens of megabytes
// today, so a substituted file cannot inflate without limit.
const maxASNInflated = 256 << 20

var errASNTooLarge = errors.New("ip2asn dataset inflates past its size limit")

// ASNInfo names an autonomous system.
type ASNInfo struct {
	Name    string
	Country string
}

type asnRange struct {
	start, end uint32
	asn        uint32
}

// ASNTable maps an IPv4 address to the AS announcing it, from iptoasn.com's
// ip2asn-v4 dataset: sorted ranges, searched by binary search.
type ASNTable struct {
	ranges []asnRange
	names  map[uint32]ASNInfo
}

// ParseASNTable reads the uncompressed TSV. Any malformed line fails the whole
// table, so a truncated or substituted download never replaces a good one.
func ParseASNTable(r io.Reader) (*ASNTable, error) {
	t := &ASNTable{names: map[uint32]ASNInfo{}}
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		if err := t.addLine(scanner.Text()); err != nil {
			return nil, fmt.Errorf("ip2asn line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(t.ranges) == 0 {
		return nil, errors.New("ip2asn: no announced ranges")
	}
	sort.Slice(t.ranges, func(i, j int) bool { return t.ranges[i].start < t.ranges[j].start })
	return t, nil
}

func (t *ASNTable) addLine(line string) error {
	cols := strings.Split(line, "\t")
	if len(cols) != asnColumns {
		return fmt.Errorf("want %d tab-separated columns, got %d", asnColumns, len(cols))
	}
	start, errStart := parseIPv4(cols[0])
	end, errEnd := parseIPv4(cols[1])
	asn, errASN := strconv.ParseUint(cols[2], 10, 32)
	if err := errors.Join(errStart, errEnd, errASN); err != nil {
		return err
	}
	if end < start {
		return errors.New("range ends before it starts")
	}
	// AS 0 is space nobody announces. Leaving it out makes it "not found",
	// which Lookup reports as 0 anyway.
	if asn == 0 {
		return nil
	}
	t.ranges = append(t.ranges, asnRange{start: start, end: end, asn: uint32(asn)})
	t.names[uint32(asn)] = ASNInfo{Name: cols[4], Country: cols[3]}
	return nil
}

func parseIPv4(s string) (uint32, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil || !addr.Is4() {
		return 0, fmt.Errorf("%q is not an IPv4 address", s)
	}
	b := addr.As4()
	return binary.BigEndian.Uint32(b[:]), nil
}

// Lookup returns the AS announcing ip, or 0 when none does.
func (t *ASNTable) Lookup(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	v := binary.BigEndian.Uint32(b[:])
	i := sort.Search(len(t.ranges), func(i int) bool { return t.ranges[i].start > v }) - 1
	if i < 0 || v > t.ranges[i].end {
		return 0
	}
	return t.ranges[i].asn
}

// Names returns every AS the table knows, for asn_names.
func (t *ASNTable) Names() map[uint32]ASNInfo { return t.names }

// LoadASNFile reads a gzipped dataset from disk.
func LoadASNFile(path string) (*ASNTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("ip2asn: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return ParseASNTable(&cappedReader{r: gz, left: maxASNInflated})
}

// cappedReader fails rather than ends at its limit: a silent end could fall
// on a line boundary and pass a truncated table off as whole.
type cappedReader struct {
	r    io.Reader
	left int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errASNTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}
