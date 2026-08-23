package connectivity

import (
	"context"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/models"
)

// zteDriver speaks the ZTE C300/C320 SNMP dialect. C300 and C320 share every
// OID at the firmware versions verified so far, so both models resolve to this
// implementation with a different Model() - a later divergence gets its own
// branch here without touching the registry or any call site.
type zteDriver struct {
	model models.OLTModel
}

func init() {
	Register(zteDriver{model: models.OLTModelZTEC300})
	Register(zteDriver{model: models.OLTModelZTEC320})
}

func (d zteDriver) Model() models.OLTModel { return d.model }

func (zteDriver) WalkStatuses(ipAddress, community string, snmpPort int) (map[ONTLocation]int, error) {
	// ZTE phase states already are the canonical vocabulary (3/4/6/1), which is
	// where that vocabulary came from.
	return zteWalkStatuses(ipAddress, community, snmpPort)
}

func (zteDriver) WalkMetrics(ipAddress, community string, snmpPort int) (map[ONTLocation]ONTMetrics, error) {
	return zteWalkMetrics(ipAddress, community, snmpPort)
}

func (zteDriver) QueryONTMetrics(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONTMetrics, error) {
	return zteQueryONTMetrics(ipAddress, community, snmpPort, slot, port, ontID)
}

func (zteDriver) WalkTrafficRates(ipAddress, community string, snmpPort int) (map[ONTLocation]ONUTrafficRates, error) {
	return zteWalkTrafficRates(ipAddress, community, snmpPort)
}

func (zteDriver) QueryTrafficRates(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONUTrafficRates, error) {
	return zteQueryTrafficRates(ipAddress, community, snmpPort, slot, port, ontID)
}

// WalkUnconfigured bounds the scan at uncfgScanTimeout even when the caller
// passes a context without a deadline: three sequential walks against an
// unresponsive OLT would otherwise outlast the HTTP client waiting on them.
func (zteDriver) WalkUnconfigured(ctx context.Context, ipAddress, community string, snmpPort int) ([]UnconfiguredONU, error) {
	ctx, cancel := context.WithTimeout(ctx, uncfgScanTimeout)
	defer cancel()

	return walkUnconfiguredONUs(ctx, ipAddress, community, snmpPort)
}

// Inventory reads identity data for the given ONTs. Serial, name, description
// and model are indexed per (slot, port) subtree, so those are walked once per
// PON port; IP, MAC and hardware version live in flat tables walked whole.
func (zteDriver) Inventory(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error) {
	inventory := make(map[ONTLocation]ONTInventory, len(locations))
	if len(locations) == 0 {
		return inventory, nil
	}

	set := func(loc ONTLocation, apply func(*ONTInventory)) {
		inv := inventory[loc]
		apply(&inv)
		inventory[loc] = inv
	}

	// Flat tables: one walk each for every ONT on the OLT.
	if ips, err := zteWalkIPAddresses(ipAddress, community, snmpPort); err == nil {
		for loc, ip := range ips {
			set(loc, func(inv *ONTInventory) { inv.IPAddress = ip })
		}
	}
	if macs, err := zteWalkMACAddresses(ipAddress, community, snmpPort); err == nil {
		for loc, mac := range macs {
			set(loc, func(inv *ONTInventory) { inv.MACAddress = mac })
		}
	}
	if hws, err := zteWalkHardwareVersions(ipAddress, community, snmpPort); err == nil {
		for loc, hw := range hws {
			set(loc, func(inv *ONTInventory) { inv.HardwareVersion = hw })
		}
	}

	// Per-port tables, grouped so each subtree is walked once.
	byPort := make(map[ONTLocation][]ONTLocation)
	for _, loc := range locations {
		key := ONTLocation{Slot: loc.Slot, Port: loc.Port}
		byPort[key] = append(byPort[key], loc)
	}

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return inventory, err
	}
	defer func() { _ = client.Conn.Close() }()

	for key, locs := range byPort {
		ifIndexONU := OnuIDIfIndexBase + key.Slot*OnuIDSlotStride + key.Port*OnuIDIncrement
		ifIndexType := OnuTypeIfIndexBase + key.Slot*OnuTypeSlotStride + key.Port*OnuTypeIncrement

		walkPortColumn(client, BaseOID1+OnuSerialNumberPrefix, ifIndexONU, locs, func(loc ONTLocation, pdu gosnmp.SnmpPDU) {
			if serial := ExtractSerialNumber(pdu.Value); serial != "" {
				set(loc, func(inv *ONTInventory) { inv.SerialNumber = serial })
			}
		})
		walkPortColumn(client, BaseOID2+OnuIDNamePrefix, ifIndexType, locs, func(loc ONTLocation, pdu gosnmp.SnmpPDU) {
			if name := ExtractName(pdu.Value); name != "" {
				set(loc, func(inv *ONTInventory) { inv.Name = name })
			}
		})
		walkPortColumn(client, BaseOID2+OnuDescriptionPrefix, ifIndexType, locs, func(loc ONTLocation, pdu gosnmp.SnmpPDU) {
			if desc := ExtractName(pdu.Value); desc != "" {
				set(loc, func(inv *ONTInventory) { inv.Description = desc })
			}
		})
		walkPortColumn(client, BaseOID2+OnuTypePrefix, ifIndexType, locs, func(loc ONTLocation, pdu gosnmp.SnmpPDU) {
			if deviceType := ExtractName(pdu.Value); deviceType != "" {
				set(loc, func(inv *ONTInventory) { inv.DeviceType = deviceType })
			}
		})
	}

	return inventory, nil
}

// walkPortColumn walks one ZTE table column scoped to a single PON port and
// hands each value to visit, matched to the ONT whose ID ends the OID.
func walkPortColumn(client *gosnmp.GoSNMP, tableOID string, ifIndex int, locations []ONTLocation, visit func(ONTLocation, gosnmp.SnmpPDU)) {
	oid := tableOID + "." + strconv.Itoa(ifIndex)

	_ = client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 2 {
			return nil
		}
		onuID, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil
		}
		for _, loc := range locations {
			if loc.ONTID == onuID {
				visit(loc, pdu)
				break
			}
		}
		return nil
	})
}
