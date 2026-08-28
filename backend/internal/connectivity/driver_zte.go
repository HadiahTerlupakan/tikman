package connectivity

import (
	"context"

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
func (d zteDriver) Inventory(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error) {
	// One shape for every caller: the fetch is a handful of batched GETs
	// whether it is asked for one ONU or two hundred. It used to branch on the
	// count because the bulk path walked whole tables, which a snapshot of a
	// single ONU had no business doing.
	return queryZTEInventoryFor(ipAddress, community, snmpPort, locations)
}

// InventoryByPort reports one PON port per instalment. The flat tables are
// walked once up front: scoping them to an instalment instead would repeat
// three full-table walks per port, which on a 198-ONT OLT meant the MAC table
// timing out and no ONT registering for minutes.
func (zteDriver) InventoryByPort(ipAddress, community string, snmpPort int, locations []ONTLocation, report func([]ONTLocation, map[ONTLocation]ONTInventory)) error {
	if len(locations) == 0 {
		return nil
	}

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return err
	}
	defer func() { _ = client.Conn.Close() }()

	// Grouped by PON port so discovery still reports one instalment per port,
	// which is what moves the progress bar on a large OLT.
	byPort := make(map[ONTLocation][]ONTLocation)
	order := make([]ONTLocation, 0)
	for _, loc := range locations {
		key := ONTLocation{Slot: loc.Slot, Port: loc.Port}
		if _, seen := byPort[key]; !seen {
			order = append(order, key)
		}
		byPort[key] = append(byPort[key], loc)
	}

	for _, key := range order {
		locs := byPort[key]
		report(locs, fetchZTEInventory(client, locs))
	}

	return nil
}
