package netflow

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPSetHoldsAddressesAndNetworks(t *testing.T) {
	set, err := ParseIPSet(strings.NewReader(`# exits
198.51.100.7
192.0.2.0/25

2001:db8::/32
`))
	require.NoError(t, err)

	assert.True(t, set.Contains(addr("198.51.100.7")))
	assert.False(t, set.Contains(addr("198.51.100.8")))
	assert.True(t, set.Contains(addr("192.0.2.0")))
	assert.True(t, set.Contains(addr("192.0.2.127")))
	assert.False(t, set.Contains(addr("192.0.2.128")))
	assert.False(t, set.Contains(addr("2001:db8::1")), "IPv6 is not loaded")
}

// The lists overlap themselves: a /24 inside a /22 that starts earlier must
// not hide the /22's later addresses from the binary search.
func TestIPSetMergesOverlappingNetworks(t *testing.T) {
	set, err := ParseIPSet(strings.NewReader("203.0.112.0/22\n203.0.113.0/24\n203.0.116.0/24\n203.0.120.0/24\n"))
	require.NoError(t, err)

	assert.True(t, set.Contains(addr("203.0.115.255")), "past the inner /24, still in the /22")
	assert.True(t, set.Contains(addr("203.0.116.200")), "an adjacent network joins")
	assert.False(t, set.Contains(addr("203.0.117.0")), "the gap stays out")
	assert.True(t, set.Contains(addr("203.0.120.9")))
	assert.Len(t, set.ranges, 2)
}

func TestIPSetCoversTheTopOfTheAddressSpace(t *testing.T) {
	set, err := ParseIPSet(strings.NewReader("255.255.255.0/24\n255.255.255.128/25\n"))
	require.NoError(t, err)

	assert.True(t, set.Contains(addr("255.255.255.255")))
	assert.Len(t, set.ranges, 1)
}

func TestParseIPSetRejectsABadList(t *testing.T) {
	for name, body := range map[string]string{
		"html":     "<html>rate limited</html>\n",
		"no IPv4":  "# empty\n2001:db8::/32\n",
		"bad bits": "192.0.2.0/40\n",
	} {
		_, err := ParseIPSet(strings.NewReader(body))
		assert.Error(t, err, name)
	}
}

func TestNilIPSetContainsNothing(t *testing.T) {
	var set *IPSet
	assert.False(t, set.Contains(addr("192.0.2.1")))
}
