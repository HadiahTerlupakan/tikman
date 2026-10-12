package netflow

import (
	"bytes"
	"compress/gzip"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const asnFixture = "192.0.2.0\t192.0.2.63\t64496\tZZ\tEXAMPLE-NET-A\n" +
	"192.0.2.128\t192.0.2.255\t64496\tZZ\tEXAMPLE-NET-A\n" +
	"198.51.100.0\t198.51.100.127\t64497\tZZ\tEXAMPLE-NET-B\n" +
	"198.51.100.128\t198.51.100.255\t0\tNone\tNot routed\n" +
	"203.0.113.0\t203.0.113.127\t64498\tZZ\tEXAMPLE-NET-C\n"

func gzipped(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// Both ends of a range are inside it; one past the end is not.
func TestLookupFindsTheRangeHoldingAnAddress(t *testing.T) {
	table, err := ParseASNTable(strings.NewReader(asnFixture))
	require.NoError(t, err)

	cases := map[string]uint32{
		"192.0.2.0":      64496,
		"192.0.2.255":    64496,
		"198.51.100.127": 64497,
		"198.51.100.128": 0, // announced by nobody
		"203.0.113.77":   64498,
		"127.0.0.1":      0, // before the first range
		"192.0.2.100":    0, // in the gap between two ranges
		"203.0.113.200":  0, // above the last range
		"2001:db8::1":    0, // IPv4 only
	}
	for ip, want := range cases {
		assert.Equal(t, want, table.Lookup(netip.MustParseAddr(ip)), ip)
	}
	assert.Equal(t, ASNInfo{Name: "EXAMPLE-NET-A", Country: "ZZ"}, table.Names()[64496])
	assert.NotContains(t, table.Names(), uint32(0))
}

// A malformed line fails the whole table, so a truncated or substituted file
// never replaces a good one.
func TestParseRefusesAMalformedDataset(t *testing.T) {
	cases := map[string]string{
		"AS prefix":         asnFixture + "192.0.2.0\t192.0.2.255\tAS64496\tZZ\tX\n",
		"four columns":      asnFixture + "192.0.2.0\t192.0.2.255\t64496\tZZ\n",
		"end before start":  asnFixture + "192.0.2.255\t192.0.2.0\t64496\tZZ\tX\n",
		"IPv6 range":        asnFixture + "2001:db8::\t2001:db8::ff\t64496\tZZ\tX\n",
		"nothing announced": "198.51.100.128\t198.51.100.255\t0\tNone\tNot routed\n",
		"empty":             "",
	}
	for name, text := range cases {
		_, err := ParseASNTable(strings.NewReader(text))
		assert.Error(t, err, name)
	}
}

// Lookup's binary search is only right for disjoint ranges, so a table whose
// ranges overlap is refused rather than answering 0 for some covered addresses.
func TestParseRefusesOverlappingRanges(t *testing.T) {
	text := "192.0.2.0\t192.0.2.255\t64496\tZZ\tWIDE\n" +
		"192.0.2.64\t192.0.2.127\t64497\tZZ\tNARROW\n"

	_, err := ParseASNTable(strings.NewReader(text))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "192.0.2.64")
	assert.Contains(t, err.Error(), "192.0.2.255")
}

func TestLoadASNFileReadsTheGzippedDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	require.NoError(t, os.WriteFile(path, gzipped(t, asnFixture), 0o644))

	table, err := LoadASNFile(path)

	require.NoError(t, err)
	assert.Equal(t, uint32(64498), table.Lookup(netip.MustParseAddr("203.0.113.1")))
}
