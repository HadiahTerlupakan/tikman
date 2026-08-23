package connectivity

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

// EPON identifies an ONU by MAC and the device exposes no serial column, so the
// MAC fills SerialNumber too - that is the identity the application keys on.
func TestHSGQInventoryUsesMACAsIdentity(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{
			Name:  hsgqOID(hsgqONUMAC, hsgqIfIndexPON1ONU1),
			Type:  gosnmp.OctetString,
			Value: []byte{0xEC, 0x23, 0x7B, 0xD7, 0x1F, 0xA8},
		},
		{Name: hsgqOID(hsgqONUName, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("ONU01/01")},
		{Name: hsgqOID(hsgqONUModel, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("F460910")},
		{Name: hsgqOID(hsgqONUFirmware, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("V6.0.3P1T1")},
		{Name: hsgqOID(hsgqONUHardware, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("V6.0")},
		// An ONU the caller did not ask about must not appear in the result.
		{Name: hsgqOID(hsgqONUName, hsgqIfIndexPON6ONU2), Type: gosnmp.OctetString, Value: []byte("ONU06/02")},
	})

	loc := ONTLocation{Port: 1, ONTID: 1}
	inventory, err := hsgqDriver{}.Inventory("127.0.0.1", "public", port, []ONTLocation{loc})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}

	if len(inventory) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(inventory), inventory)
	}

	got := inventory[loc]
	want := ONTInventory{
		SerialNumber: "EC237BD71FA8",
		MACAddress:   "EC:23:7B:D7:1F:A8",
		Name:         "ONU01/01",
		DeviceType:   "F460910",
		// The dots must survive: ExtractName would render these "V603P1T1" and
		// "V60", which is a wrong firmware version shown to the operator.
		SoftwareVersion: "V6.0.3P1T1",
		HardwareVersion: "V6.0",
	}
	if got != want {
		t.Errorf("inventory = %+v, want %+v", got, want)
	}
}
