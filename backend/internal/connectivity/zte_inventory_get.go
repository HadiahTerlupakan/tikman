package connectivity

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// zteGetBatchSize is how many instances go in one GET. SNMP carries many
// varbinds per request, and the C300's own packetsize is 8192 bytes; forty
// short OIDs sit well inside that while cutting a 200-ONU sweep to a handful
// of round trips.
const zteGetBatchSize = 40

// zteInventoryBatchSize is smaller than the shared one because these columns
// answer with text - serial, MAC, name, description, both versions - and forty
// of them overflow a single datagram. The fragments do not always survive the
// path to the OLT, and the batch then goes unanswered: the same read that
// takes twenty seconds from the host never finished from inside the container.
// The metric columns are integers and stay well inside one datagram at forty.
const zteInventoryBatchSize = 10

// zteReadDeadline bounds one batched read of an OLT.
//
// gosnmp's own Timeout did not: the client is built with context.Background(),
// which never cancels, and a Get sat in it for over ten minutes. The worker
// runs its cycles one after another, so that single call froze every ONT's
// status and metrics until the process was killed.
//
// Sized to catch a hang, not to cap real work: reading two hundred ONUs takes
// about twenty seconds, while the hang this exists for ran past ten minutes.
// A variable only so the test covering it need not spend the wait.
var zteReadDeadline = 2 * time.Minute

// inventoryColumn is one field of an ONU's inventory and where to read it.
type inventoryColumn struct {
	oid   string
	apply func(*ONTInventory, any)
}

// zteInventoryColumns lists the exact instances that make up one ONU's
// inventory.
//
// Every one is a direct GET. The tables were walked before — the whole table
// for IP, MAC and hardware version, a whole PON port for the rest — and on a
// busy OLT those walks timed out and returned nothing, which is why those
// columns were empty everywhere. Individually each answers in milliseconds.
//
// IP and MAC carry a further element after the ONU; only index 1 holds the
// management address, which is what the walk they replace also took.
func zteInventoryColumns(loc ONTLocation) []inventoryColumn {
	onuSpace := OnuIDIfIndexBase + loc.Slot*OnuIDSlotStride + loc.Port*OnuIDIncrement
	typeSpace := OnuTypeIfIndexBase + loc.Slot*OnuTypeSlotStride + loc.Port*OnuTypeIncrement

	instance := func(table string, ifIndex int, tail string) string {
		return fmt.Sprintf("%s.%d.%d%s", table, ifIndex, loc.ONTID, tail)
	}

	return []inventoryColumn{
		{instance(BaseOID1+OnuSerialNumberPrefix, onuSpace, ""), func(inv *ONTInventory, v any) {
			if serial := ExtractSerialNumber(v); serial != "" {
				inv.SerialNumber = serial
			}
		}},
		{instance(BaseOID2+OnuIDNamePrefix, typeSpace, ""), func(inv *ONTInventory, v any) {
			inv.Name = ExtractName(v)
		}},
		{instance(BaseOID2+OnuDescriptionPrefix, typeSpace, ""), func(inv *ONTInventory, v any) {
			inv.Description = ExtractName(v)
		}},
		{instance(BaseOID2+OnuTypePrefix, typeSpace, ""), func(inv *ONTInventory, v any) {
			inv.DeviceType = ExtractName(v)
		}},
		{instance(BaseOID2+OnuHardwareVersionPrefix, typeSpace, ""), func(inv *ONTInventory, v any) {
			inv.HardwareVersion = ExtractName(v)
		}},
		{instance(BaseOID2+OnuIPAddressPrefix, typeSpace, ".1"), func(inv *ONTInventory, v any) {
			inv.IPAddress = readZTEIPAddress(v)
		}},
		{instance(BaseOID2+OnuMACAddressPrefix, typeSpace, ".1"), func(inv *ONTInventory, v any) {
			inv.MACAddress = formatZTEMACAddress(v)
		}},
	}
}

// fetchZTEInventory reads the inventory of the given ONUs with batched GETs.
// A batch that fails leaves those fields unset rather than failing the sweep:
// a missing hardware version is not a reason to report no ONUs.
func fetchZTEInventory(client *gosnmp.GoSNMP, locations []ONTLocation) map[ONTLocation]ONTInventory {
	inventory := make(map[ONTLocation]ONTInventory, len(locations))

	type request struct {
		loc    ONTLocation
		column inventoryColumn
	}
	pending := make([]request, 0, len(locations)*7)
	for _, loc := range locations {
		inventory[loc] = ONTInventory{}
		for _, column := range zteInventoryColumns(loc) {
			pending = append(pending, request{loc: loc, column: column})
		}
	}

	for start := 0; start < len(pending); start += zteInventoryBatchSize {
		end := start + zteInventoryBatchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[start:end]

		oids := make([]string, len(batch))
		for i, item := range batch {
			oids[i] = item.column.oid
		}

		result, err := client.Get(oids)
		if err != nil || result == nil {
			continue
		}
		for i, pdu := range result.Variables {
			if i >= len(batch) {
				break
			}
			if pdu.Type == gosnmp.NoSuchInstance || pdu.Type == gosnmp.NoSuchObject || pdu.Type == gosnmp.EndOfMibView {
				continue
			}
			inv := inventory[batch[i].loc]
			batch[i].column.apply(&inv, pdu.Value)
			inventory[batch[i].loc] = inv
		}
	}

	return inventory
}
