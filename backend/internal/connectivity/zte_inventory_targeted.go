package connectivity

import (
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// targetedInventoryLimit is the point past which asking per ONU stops paying.
//
// The bulk path walks the IP, MAC and hardware-version tables in full once, and
// then reads each PON port's columns. That is right for a discovery sweep of a
// whole OLT. For a handful of ONUs it is three whole-table walks to read a
// handful of rows — on a busy C300 those walks time out, and a provisioning
// snapshot of one ONU spent 55 of its 67 seconds waiting for them to fail.
const targetedInventoryLimit = 8

// queryZTEInventoryFor reads the inventory of specific ONUs by scoping every
// walk to their own subtree, so each returns a varbind or two instead of the
// whole table.
//
// The subtree is walked rather than fetched with a GET because these tables do
// not share one index shape: hardware version is indexed by ifIndex.onuID while
// IP and MAC carry a further element after it. Scoping to ifIndex.onuID is
// exact for all of them without depending on what follows.
func queryZTEInventoryFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	inventory := make(map[ONTLocation]ONTInventory, len(locations))
	for _, loc := range locations {
		inv := ONTInventory{}
		ifIndexONU := OnuIDIfIndexBase + loc.Slot*OnuIDSlotStride + loc.Port*OnuIDIncrement
		ifIndexType := OnuTypeIfIndexBase + loc.Slot*OnuTypeSlotStride + loc.Port*OnuTypeIncrement

		if value, ok := firstStringUnder(client, BaseOID1+OnuSerialNumberPrefix, ifIndexONU, loc.ONTID, ExtractSerialNumber); ok {
			inv.SerialNumber = value
		}
		for _, column := range []struct {
			oid   string
			apply func(string)
		}{
			{BaseOID2 + OnuIDNamePrefix, func(v string) { inv.Name = v }},
			{BaseOID2 + OnuDescriptionPrefix, func(v string) { inv.Description = v }},
			{BaseOID2 + OnuTypePrefix, func(v string) { inv.DeviceType = v }},
			{BaseOID2 + OnuHardwareVersionPrefix, func(v string) { inv.HardwareVersion = v }},
		} {
			if value, ok := firstStringUnder(client, column.oid, ifIndexType, loc.ONTID, ExtractName); ok {
				column.apply(value)
			}
		}
		if value, ok := firstStringUnder(client, BaseOID2+OnuIPAddressPrefix, ifIndexType, loc.ONTID, readZTEIPAddress); ok {
			inv.IPAddress = value
		}
		if value, ok := firstStringUnder(client, BaseOID2+OnuMACAddressPrefix, ifIndexType, loc.ONTID, formatZTEMACAddress); ok {
			inv.MACAddress = value
		}

		inventory[loc] = inv
	}

	return inventory, nil
}

// readZTEIPAddress reads the management address as the OLT reports it. The
// name decoder cannot be used here: it strips punctuation, which turns
// 10.0.0.9 into 10009. An unset address reads as 0.0.0.0 and counts as absent.
func readZTEIPAddress(value any) string {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case []byte:
		text = string(typed)
	default:
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "0.0.0.0" {
		return ""
	}
	return text
}

// formatZTEMACAddress renders the six raw bytes the OLT reports. Anything of
// another length is not a MAC and is reported as absent rather than as a
// mangled one.
func formatZTEMACAddress(value any) string {
	raw, ok := value.([]byte)
	if !ok || len(raw) != 6 {
		return ""
	}
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		raw[0], raw[1], raw[2], raw[3], raw[4], raw[5])
}

// firstStringUnder returns the value one ONU has in a table column.
//
// The exact instance is fetched first, which is what the tables indexed by
// ifIndex.onuID hold. IP and MAC carry a further element after the ONU, so a
// GET there finds nothing and the ONU's own subtree is walked instead.
//
// Walking the whole PON port was the first attempt and was wrong twice over:
// it pulled fifty ONUs' rows to read one, and on a busy OLT the walk died part
// way through and returned nothing at all. Individually these OIDs answer in
// milliseconds.
func firstStringUnder(client *gosnmp.GoSNMP, tableOID string, ifIndex, onuID int, extract func(any) string) (string, bool) {
	instance := fmt.Sprintf("%s.%d.%d", tableOID, ifIndex, onuID)

	if result, err := client.Get([]string{instance}); err == nil && len(result.Variables) > 0 {
		pdu := result.Variables[0]
		if pdu.Type != gosnmp.NoSuchInstance && pdu.Type != gosnmp.NoSuchObject {
			if value := extract(pdu.Value); value != "" {
				return value, true
			}
		}
	}

	var found string
	_ = client.Walk(instance, func(pdu gosnmp.SnmpPDU) error {
		if found == "" {
			if value := extract(pdu.Value); value != "" {
				found = value
			}
		}
		return nil
	})

	return found, found != ""
}
