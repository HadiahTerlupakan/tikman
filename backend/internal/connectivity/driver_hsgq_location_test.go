package connectivity

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

// Real ifIndexes from the verified HSGQ-XE08ID: 0x01000101 is the ONU the device
// itself names "ONU01/01", 0x01000602 is "ONU06/02".
const (
	hsgqIfIndexPON1ONU1 = 0x01000101
	hsgqIfIndexPON6ONU2 = 0x01000602
)

// The ifIndex layout is 0x0100PPNN. This was checked against the device's own
// ONU names for all 246 ONUs with no mismatch, so it is decoded rather than
// left as an opaque label.
func TestHSGQLocationDecodesPONAndONU(t *testing.T) {
	tests := []struct {
		ifIndex uint32
		want    ONTLocation
		name    string
	}{
		{ifIndex: 0x01000101, want: ONTLocation{Port: 1, ONTID: 1}, name: "ONU01/01"},
		{ifIndex: 0x01000102, want: ONTLocation{Port: 1, ONTID: 2}, name: "ONU01/02"},
		{ifIndex: 0x01000602, want: ONTLocation{Port: 6, ONTID: 2}, name: "ONU06/02"},
		{ifIndex: 0x01000839, want: ONTLocation{Port: 8, ONTID: 57}, name: "ONU08/57"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hsgqLocation(tt.ifIndex); got != tt.want {
				t.Errorf("hsgqLocation(0x%08X) = %+v, want %+v", tt.ifIndex, got, tt.want)
			}
			// QueryONTMetrics rebuilds the ifIndex, so the two must be inverses;
			// a mismatch would query a different ONU than the caller asked for.
			if back := hsgqIfIndexFor(tt.want.Port, tt.want.ONTID); back != tt.ifIndex {
				t.Errorf("hsgqIfIndexFor(%d, %d) = 0x%08X, want 0x%08X", tt.want.Port, tt.want.ONTID, back, tt.ifIndex)
			}
		})
	}
}

// The IF-MIB index is assigned dynamically and only to registered ONUs, so it
// shifts when an ONU drops. Resolution must therefore happen per read, by name.
// A cached mapping would keep reading the old index and bill one subscriber's
// traffic to another.
func TestHSGQResolveIfMibIndexesFollowsRenumbering(t *testing.T) {
	resolve := func(t *testing.T, pdus []gosnmp.SnmpPDU) map[ONTLocation]uint32 {
		t.Helper()
		_, port := newUncfgAgent(t, pdus)
		client, err := newSNMPClient("127.0.0.1", "public", port)
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		defer func() { _ = client.Conn.Close() }()

		indexes, err := hsgqResolveIfMibIndexes(client)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		return indexes
	}

	before := resolve(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("PON01")},
		{Name: hsgqOID(ifDescrOID, 17), Type: gosnmp.OctetString, Value: []byte("ONU01/01")},
		{Name: hsgqOID(ifDescrOID, 18), Type: gosnmp.OctetString, Value: []byte("ONU01/02")},
	})
	if got := before[ONTLocation{Port: 1, ONTID: 2}]; got != 18 {
		t.Fatalf("ONU01/02 = %d, want 18", got)
	}
	// Physical ports must not be picked up as ONUs.
	if len(before) != 2 {
		t.Fatalf("got %d ONU entries, want 2: %+v", len(before), before)
	}

	// ONU01/01 drops and the OLT renumbers, so ONU01/02 now answers on 17.
	after := resolve(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("PON01")},
		{Name: hsgqOID(ifDescrOID, 17), Type: gosnmp.OctetString, Value: []byte("ONU01/02")},
	})
	if got := after[ONTLocation{Port: 1, ONTID: 2}]; got != 17 {
		t.Errorf("after renumbering ONU01/02 = %d, want 17", got)
	}
	if _, stillThere := after[ONTLocation{Port: 1, ONTID: 1}]; stillThere {
		t.Error("ONU01/01 is gone from the OLT but still resolved")
	}
}
