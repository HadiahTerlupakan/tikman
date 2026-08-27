package connectivity

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

func TestWalkVLANsReadsTheStaticVLANTable(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: dot1qVlanStaticName + ".100", Type: gosnmp.OctetString, Value: []byte("VLAN0100")},
		{Name: dot1qVlanStaticName + ".1", Type: gosnmp.OctetString, Value: []byte("VLAN0001")},
		{Name: dot1qVlanStaticName + ".213", Type: gosnmp.OctetString, Value: []byte("VLAN0213-PPP")},
	})

	vlans, err := WalkVLANs("127.0.0.1", "public", snmpPort)
	if err != nil {
		t.Fatalf("WalkVLANs: %v", err)
	}

	// Ordered by VLAN ID, because the form renders them straight into a list.
	want := []OLTVLAN{
		{VLANID: 1, Name: "VLAN0001"},
		{VLANID: 100, Name: "VLAN0100"},
		{VLANID: 213, Name: "VLAN0213-PPP"},
	}
	if len(vlans) != len(want) {
		t.Fatalf("got %d VLANs %+v, want %d", len(vlans), vlans, len(want))
	}
	for i, vlan := range vlans {
		if vlan != want[i] {
			t.Errorf("VLAN %d = %+v, want %+v", i, vlan, want[i])
		}
	}
}

// A VLAN ID outside 1-4094 cannot be a real row, so a stray OID must not turn
// into a selectable option.
func TestWalkVLANsSkipsOutOfRangeIndexes(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: dot1qVlanStaticName + ".0", Type: gosnmp.OctetString, Value: []byte("bogus")},
		{Name: dot1qVlanStaticName + ".9999", Type: gosnmp.OctetString, Value: []byte("bogus")},
		{Name: dot1qVlanStaticName + ".7", Type: gosnmp.OctetString, Value: []byte("VLAN0007")},
	})

	vlans, err := WalkVLANs("127.0.0.1", "public", snmpPort)
	if err != nil {
		t.Fatalf("WalkVLANs: %v", err)
	}

	if len(vlans) != 1 || vlans[0].VLANID != 7 {
		t.Fatalf("got %+v, want only VLAN 7", vlans)
	}
}
