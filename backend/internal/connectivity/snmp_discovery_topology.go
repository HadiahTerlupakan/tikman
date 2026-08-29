package connectivity

import (
	"fmt"
	"log"
)

// GPONSlot represents a line-card slot with its PON ports
type GPONSlot struct {
	Slot  int        `json:"slot"`
	Ports []GponPort `json:"ports"`
}

// GponPort represents a PON port within a slot
type GponPort struct {
	PortID int             `json:"port_id"`
	ONTs   []DiscoveredONT `json:"onts"`
}

// mapPhaseToRunState converts a canonical phase state to the legacy RunState value.
func mapPhaseToRunState(phaseState int) int {
	return phaseState
}

// DiscoverOLTTopology enumerates the ONTs an OLT knows about and returns them as
// Slot -> Port -> ONTs with metrics and identity data attached.
//
// The status table is the enumeration primitive: whatever it reports is the set
// of ONTs that exist. Metrics and inventory are enrichment - a driver that does
// not expose them (or an OLT that fails mid-walk) yields ONTs with empty fields
// rather than no ONTs at all.
func DiscoverOLTTopology(driver Driver, ipAddress, community string, snmpPort int) ([]GPONSlot, error) {
	log.Printf("[Topology] Starting discovery for OLT %s:%d (model %s)", ipAddress, snmpPort, driver.Model())

	statuses, err := driver.WalkStatuses(ipAddress, community, snmpPort)
	if err != nil {
		return nil, fmt.Errorf("failed to walk phase state table: %w", err)
	}
	log.Printf("[Topology] Found %d ONTs in phase state table", len(statuses))

	// Read for the ONUs the status table just named. Sweeping the optical tables
	// does not finish on a populated OLT: the RX power walk returned 96 of 200
	// rows before timing out, so most ONTs came back with empty fields.
	metrics, err := readMetricsForKnownONTs(driver, ipAddress, community, snmpPort, statuses)
	if err != nil {
		log.Printf("[Topology] Warning: failed to read metrics: %v", err)
		metrics = make(map[ONTLocation]ONTMetrics)
	}
	log.Printf("[Topology] Retrieved metrics for %d ONTs", len(metrics))

	// Group ONT locations by slot then port
	slotMap := make(map[int]map[int][]ONTLocation)
	locations := make([]ONTLocation, 0, len(statuses))
	for loc := range statuses {
		if slotMap[loc.Slot] == nil {
			slotMap[loc.Slot] = make(map[int][]ONTLocation)
		}
		slotMap[loc.Slot][loc.Port] = append(slotMap[loc.Slot][loc.Port], loc)
		locations = append(locations, loc)
	}
	log.Printf("[Topology] Found %d slots with ONTs", len(slotMap))

	inventory, err := driver.Inventory(ipAddress, community, snmpPort, locations)
	if err != nil {
		log.Printf("[Topology] Warning: failed to read ONT inventory: %v", err)
		inventory = make(map[ONTLocation]ONTInventory)
	}
	log.Printf("[Topology] Retrieved inventory for %d out of %d ONTs", len(inventory), len(statuses))

	topology := buildTopologyStructure(slotMap, statuses, inventory, metrics)

	log.Printf("[Topology] Discovered %d slots with full details", len(topology))
	return topology, nil
}

// readMetricsForKnownONTs prefers a driver that can read named ONUs, falling
// back to the table sweep for one that cannot.
func readMetricsForKnownONTs(driver Driver, ipAddress, community string, snmpPort int, statuses map[ONTLocation]int) (map[ONTLocation]ONTMetrics, error) {
	querier, direct := driver.(MetricsQuerier)
	if !direct || len(statuses) == 0 {
		return driver.WalkMetrics(ipAddress, community, snmpPort)
	}

	locations := make([]ONTLocation, 0, len(statuses))
	for loc := range statuses {
		locations = append(locations, loc)
	}
	return querier.QueryMetricsFor(ipAddress, community, snmpPort, locations)
}
