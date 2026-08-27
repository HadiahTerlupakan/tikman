package connectivity

import (
	"sort"
	"strconv"
	"testing"

	"github.com/gosnmp/gosnmp"
)

func zteSerialOID(slot, port, ontID int) string {
	ifIndex := OnuIDIfIndexBase + slot*OnuIDSlotStride + port*OnuIDIncrement
	return BaseOID1 + OnuSerialNumberPrefix + "." + strconv.Itoa(ifIndex) + "." + strconv.Itoa(ontID)
}

// Discovery registers ONTs and advances its progress once per instalment, so a
// walk that reported everything in one go would leave the bar at zero for the
// whole of a large OLT.
func TestZTEInventoryByPortReportsOneInstalmentPerPON(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(1, 1, 1), Type: gosnmp.OctetString, Value: []byte("ZTEG11111111")},
		{Name: zteSerialOID(1, 1, 2), Type: gosnmp.OctetString, Value: []byte("ZTEG22222222")},
		{Name: zteSerialOID(1, 2, 1), Type: gosnmp.OctetString, Value: []byte("ZTEG33333333")},
	})

	locations := []ONTLocation{
		{Slot: 1, Port: 1, ONTID: 1},
		{Slot: 1, Port: 1, ONTID: 2},
		{Slot: 1, Port: 2, ONTID: 1},
	}

	byPort := make(map[int][]ONTInventory)
	instalments := 0
	err := zteDriver{}.InventoryByPort("127.0.0.1", "public", snmpPort, locations,
		func(locs []ONTLocation, inventory map[ONTLocation]ONTInventory) {
			instalments++
			if len(locs) == 0 {
				t.Errorf("instalment %d reported no locations", instalments)
				return
			}
			port := locs[0].Port
			for _, loc := range locs {
				if loc.Port != port {
					t.Errorf("instalment mixes PON ports %d and %d", port, loc.Port)
				}
				byPort[port] = append(byPort[port], inventory[loc])
			}
		})
	if err != nil {
		t.Fatalf("InventoryByPort: %v", err)
	}

	if instalments != 2 {
		t.Fatalf("got %d instalments, want one per PON port", instalments)
	}

	serials := make([]string, 0, 3)
	for _, invs := range byPort {
		for _, inv := range invs {
			serials = append(serials, inv.SerialNumber)
		}
	}
	sort.Strings(serials)

	want := []string{"ZTEG11111111", "ZTEG22222222", "ZTEG33333333"}
	if len(serials) != len(want) {
		t.Fatalf("got %d serials %v, want %v", len(serials), serials, want)
	}
	for i, serial := range serials {
		if serial != want[i] {
			t.Errorf("serial %d = %q, want %q", i, serial, want[i])
		}
	}
}

// Inventory is the whole-result wrapper the snapshot path still uses; it must
// keep returning every location the per-port walk found.
func TestZTEInventoryMatchesTheSumOfItsInstalments(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(1, 1, 1), Type: gosnmp.OctetString, Value: []byte("ZTEG11111111")},
		{Name: zteSerialOID(1, 2, 1), Type: gosnmp.OctetString, Value: []byte("ZTEG33333333")},
	})

	locations := []ONTLocation{{Slot: 1, Port: 1, ONTID: 1}, {Slot: 1, Port: 2, ONTID: 1}}

	inventory, err := zteDriver{}.Inventory("127.0.0.1", "public", snmpPort, locations)
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}

	if got := inventory[locations[0]].SerialNumber; got != "ZTEG11111111" {
		t.Errorf("port 1 serial = %q, want ZTEG11111111", got)
	}
	if got := inventory[locations[1]].SerialNumber; got != "ZTEG33333333" {
		t.Errorf("port 2 serial = %q, want ZTEG33333333", got)
	}
}
