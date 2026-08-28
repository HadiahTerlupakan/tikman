package connectivity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePortName(t *testing.T) {
	tests := []struct {
		name     string
		ifName   string
		wantKind string
		wantRack int
		wantSlot int
		wantPort int
	}{
		{"gpon port", "gpon_1/3/1", PortKindPON, 1, 3, 1},
		{"epon port", "epon_1/5/16", PortKindPON, 1, 5, 16},
		{"ten gig uplink", "xgei_1/10/1", PortKindUplink, 1, 10, 1},
		{"gig uplink", "gei_1/11/5", PortKindUplink, 1, 11, 5},
		// The C300 lists a management interface with no slot address at all.
		// It has to survive parsing rather than be dropped from the inventory.
		{"management interface", "Mng1", PortKindOther, 0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind, rack, slot, port := parsePortName(tc.ifName)

			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantRack, rack)
			assert.Equal(t, tc.wantSlot, slot)
			assert.Equal(t, tc.wantPort, port)
		})
	}
}

func TestLastOIDSegment(t *testing.T) {
	index, ok := lastOIDSegment(".1.3.6.1.2.1.31.1.1.1.1.285278977")
	assert.True(t, ok)
	assert.Equal(t, 285278977, index)

	_, ok = lastOIDSegment(".1.3.6.1.2.1.31.1.1.1.1.gpon")
	assert.False(t, ok)
}
