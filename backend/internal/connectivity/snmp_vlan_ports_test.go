package connectivity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodePortBitmap(t *testing.T) {
	// Byte 80 (0-based) is 0xf8: bridge ports 641-645, which is slot 10's five
	// uplink ports on the OLT this was read from.
	raw := make([]byte, 128)
	raw[80] = 0xf8

	assert.Equal(t, []int{641, 642, 643, 644, 645}, decodePortBitmap(raw))
}

func TestBridgePortAddress(t *testing.T) {
	for _, tc := range []struct {
		bridgePort int
		slot, port int
	}{
		{641, 10, 1},
		{645, 10, 5},
		{705, 11, 1},
		{709, 11, 5},
	} {
		slot, port := bridgePortAddress(tc.bridgePort)
		assert.Equal(t, tc.slot, slot, "slot for bridge port %d", tc.bridgePort)
		assert.Equal(t, tc.port, port, "port for bridge port %d", tc.bridgePort)
	}
}

// A port the VLAN reaches but that is not in the untagged set is carrying the
// VLAN tagged; that is the trunk/access distinction the page shows.
func TestMergeVLANPorts(t *testing.T) {
	ports := mergeVLANPorts([]int{641, 642, 645, 705}, []int{641, 642, 705})

	assert.Equal(t, []VLANPort{
		{Slot: 10, Port: 1, Tagged: false},
		{Slot: 10, Port: 2, Tagged: false},
		{Slot: 10, Port: 5, Tagged: true},
		{Slot: 11, Port: 1, Tagged: false},
	}, ports)
}

// A VLAN with no untagged column at all is entirely tagged, not entirely
// untagged: reading a missing bitmap as "all access" would misreport a trunk.
func TestMergeVLANPortsWithoutUntaggedColumn(t *testing.T) {
	ports := mergeVLANPorts([]int{641}, nil)

	assert.Equal(t, []VLANPort{{Slot: 10, Port: 1, Tagged: true}}, ports)
}
