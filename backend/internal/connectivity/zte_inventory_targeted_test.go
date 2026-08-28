package connectivity

import (
	"strconv"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeSpaceOID(prefix string, slot, port, ontID int, extra string) string {
	ifIndex := OnuTypeIfIndexBase + slot*OnuTypeSlotStride + port*OnuTypeIncrement
	oid := BaseOID2 + prefix + "." + strconv.Itoa(ifIndex) + "." + strconv.Itoa(ontID)
	if extra != "" {
		oid += "." + extra
	}
	return oid
}

// A snapshot of one ONU used to walk the IP, MAC and hardware-version tables of
// the whole OLT. Asking for the ONU's own rows has to return the same values.
func TestQueryZTEInventoryForReadsOneONU(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(3, 1, 15), Type: gosnmp.OctetString, Value: []byte("HWTCB403E8A0")},
		{Name: typeSpaceOID(OnuIDNamePrefix, 3, 1, 15, ""), Type: gosnmp.OctetString, Value: []byte("Bapak Budi")},
		{Name: typeSpaceOID(OnuTypePrefix, 3, 1, 15, ""), Type: gosnmp.OctetString, Value: []byte("HG8245H5")},
		// Hardware version is indexed by ifIndex.onuID, IP and MAC carry a
		// further element after it. Both shapes have to resolve.
		{Name: typeSpaceOID(OnuHardwareVersionPrefix, 3, 1, 15, ""), Type: gosnmp.OctetString, Value: []byte("V1.0")},
		{Name: typeSpaceOID(OnuIPAddressPrefix, 3, 1, 15, "1"), Type: gosnmp.OctetString, Value: []byte("10.0.0.9")},
		{Name: typeSpaceOID(OnuMACAddressPrefix, 3, 1, 15, "1"), Type: gosnmp.OctetString,
			Value: []byte{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E}},
	})

	loc := ONTLocation{Slot: 3, Port: 1, ONTID: 15}
	inventory, err := queryZTEInventoryFor("127.0.0.1", "public", snmpPort, []ONTLocation{loc})
	require.NoError(t, err)

	got := inventory[loc]
	assert.Equal(t, "HWTCB403E8A0", got.SerialNumber)
	assert.Equal(t, "Bapak Budi", got.Name)
	assert.Equal(t, "HG8245H5", got.DeviceType)
	// ExtractName strips punctuation, so the OLT's "V1.0" arrives as "V10".
	// The bulk path decodes it the same way; a snapshot that differed would
	// report drift against a value discovery had just written.
	assert.Equal(t, "V10", got.HardwareVersion)
	assert.Equal(t, "10.0.0.9", got.IPAddress)
	assert.Equal(t, "00:1A:2B:3C:4D:5E", got.MACAddress)
}

// The ONUs of one PON share every column, so the scoped walk has to pick out
// the one asked for rather than the first row it meets.
func TestQueryZTEInventoryForDoesNotTakeANeighbourValue(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(3, 1, 1), Type: gosnmp.OctetString, Value: []byte("RTEGC609E381")},
		{Name: zteSerialOID(3, 1, 15), Type: gosnmp.OctetString, Value: []byte("HWTCB403E8A0")},
	})

	loc := ONTLocation{Slot: 3, Port: 1, ONTID: 15}
	inventory, err := queryZTEInventoryFor("127.0.0.1", "public", snmpPort, []ONTLocation{loc})
	require.NoError(t, err)

	assert.Equal(t, "HWTCB403E8A0", inventory[loc].SerialNumber)
}

// Six bytes is a MAC. Anything else is not, and reporting it as one would put a
// mangled address in a provisioning snapshot.
func TestFormatZTEMACAddressRejectsAnythingElse(t *testing.T) {
	assert.Equal(t, "00:1A:2B:3C:4D:5E",
		formatZTEMACAddress([]byte{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E}))
	assert.Equal(t, "", formatZTEMACAddress([]byte{0x00, 0x1A}))
	assert.Equal(t, "", formatZTEMACAddress("not bytes"))
}

// Inventory routes by size: a handful of ONUs is read directly, a sweep still
// goes through the bulk path that walks each table once.
func TestZTEInventoryUsesTheTargetedPathForAFewONUs(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(3, 1, 15), Type: gosnmp.OctetString, Value: []byte("HWTCB403E8A0")},
	})

	loc := ONTLocation{Slot: 3, Port: 1, ONTID: 15}
	inventory, err := zteDriver{}.Inventory("127.0.0.1", "public", snmpPort, []ONTLocation{loc})
	require.NoError(t, err)

	assert.Equal(t, "HWTCB403E8A0", inventory[loc].SerialNumber)
}

// A sweep is now the same fetch as a single lookup, so a whole port has to come
// back complete rather than only its first ONU.
func TestZTEInventoryReadsEveryRequestedONU(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(3, 1, 1), Type: gosnmp.OctetString, Value: []byte("RTEGC609E381")},
		{Name: zteSerialOID(3, 1, 15), Type: gosnmp.OctetString, Value: []byte("HWTCB403E8A0")},
		{Name: zteSerialOID(3, 2, 4), Type: gosnmp.OctetString, Value: []byte("ZTEGC4556B50")},
	})

	locations := []ONTLocation{
		{Slot: 3, Port: 1, ONTID: 1},
		{Slot: 3, Port: 1, ONTID: 15},
		{Slot: 3, Port: 2, ONTID: 4},
	}

	inventory, err := zteDriver{}.Inventory("127.0.0.1", "public", snmpPort, locations)
	require.NoError(t, err)

	assert.Equal(t, "RTEGC609E381", inventory[locations[0]].SerialNumber)
	assert.Equal(t, "HWTCB403E8A0", inventory[locations[1]].SerialNumber)
	assert.Equal(t, "ZTEGC4556B50", inventory[locations[2]].SerialNumber)
}

// Discovery advances its progress once per instalment, so a sweep that reported
// everything in one go would leave the bar at zero for the whole of a large OLT.
func TestZTEInventoryByPortStillReportsPerPort(t *testing.T) {
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: zteSerialOID(3, 1, 1), Type: gosnmp.OctetString, Value: []byte("RTEGC609E381")},
		{Name: zteSerialOID(3, 2, 4), Type: gosnmp.OctetString, Value: []byte("ZTEGC4556B50")},
	})

	instalments := 0
	err := zteDriver{}.InventoryByPort("127.0.0.1", "public", snmpPort,
		[]ONTLocation{{Slot: 3, Port: 1, ONTID: 1}, {Slot: 3, Port: 2, ONTID: 4}},
		func(locs []ONTLocation, _ map[ONTLocation]ONTInventory) {
			instalments++
			assert.Len(t, locs, 1)
		})

	require.NoError(t, err)
	assert.Equal(t, 2, instalments, "one instalment per PON port")
}
