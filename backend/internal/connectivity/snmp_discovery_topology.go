package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// GPONSlot represents a line-card slot with its PON ports
type GPONSlot struct {
	Slot  int         `json:"slot"`
	Ports []GponPort  `json:"ports"`
}

// GponPort represents a PON port within a slot
type GponPort struct {
	PortID int             `json:"port_id"`
	ONTs   []DiscoveredONT `json:"onts"`
}

// mapPhaseToRunState converts a ZXGPON phase state to the legacy RunState value.
func mapPhaseToRunState(phaseState int) int {
	return phaseState
}

// DiscoverOLTTopology walks the ZTE-AN-MIB tables following the exact approach
// from https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte:
// 1. Walk phase state table to discover all ONTs across all slots/ports
// 2. Extract ONU IDs from OID suffixes
// 3. Query serial numbers, names, and metrics for each ONT
// 4. Return hierarchy: Slot -> Port -> ONTs with full details
func DiscoverOLTTopology(ipAddress, community string, snmpPort int) ([]GPONSlot, error) {
	log.Printf("[Topology] Starting discovery for OLT %s:%d", ipAddress, snmpPort)

	statuses, err := WalkONTStatuses(ipAddress, community, snmpPort)
	if err != nil {
		return nil, fmt.Errorf("failed to walk phase state table: %w", err)
	}

	log.Printf("[Topology] Found %d ONTs in phase state table", len(statuses))

	// Walk metrics table once for all ONTs
	allMetrics, err := WalkONTMetrics(ipAddress, community, snmpPort)
	if err != nil {
		log.Printf("[Topology] Warning: failed to walk metrics: %v", err)
		allMetrics = make(map[ONTLocation]ONTMetrics)
	}
	log.Printf("[Topology] Retrieved metrics for %d ONTs", len(allMetrics))

	// Walk IP addresses table once for all ONTs
	allIPAddresses, err := WalkONTIPAddresses(ipAddress, community, snmpPort)
	if err != nil {
		log.Printf("[Topology] Warning: failed to walk IP addresses: %v", err)
		allIPAddresses = make(map[ONTLocation]string)
	}
	log.Printf("[Topology] Retrieved IP addresses for %d ONTs", len(allIPAddresses))

	// Walk MAC addresses table once for all ONTs
	allMACAddresses, err := WalkONTMACAddresses(ipAddress, community, snmpPort)
	if err != nil {
		log.Printf("[Topology] Warning: failed to walk MAC addresses: %v", err)
		allMACAddresses = make(map[ONTLocation]string)
	}
	log.Printf("[Topology] Retrieved MAC addresses for %d ONTs", len(allMACAddresses))

	// Walk hardware versions table once for all ONTs
	allHWVersions, err := WalkONTHardwareVersions(ipAddress, community, snmpPort)
	if err != nil {
		log.Printf("[Topology] Warning: failed to walk hardware versions: %v", err)
		allHWVersions = make(map[ONTLocation]string)
	}
	log.Printf("[Topology] Retrieved hardware versions for %d ONTs", len(allHWVersions))

	// Build data structures for serials, names, descriptions, device info, and metrics
	serials := make(map[ONTLocation]string)
	names := make(map[ONTLocation]string)
	descriptions := make(map[ONTLocation]string)
	deviceTypes := make(map[ONTLocation]string)
	hwVersions := make(map[ONTLocation]string)
	swVersions := make(map[ONTLocation]string)
	ipAddresses := make(map[ONTLocation]string)
	macAddresses := make(map[ONTLocation]string)
	metrics := make(map[ONTLocation]*ONTMetrics)

	// Group ONT locations by slot then port first
	slotMap := make(map[int]map[int][]ONTLocation)
	for loc := range statuses {
		if slotMap[loc.Slot] == nil {
			slotMap[loc.Slot] = make(map[int][]ONTLocation)
		}
		slotMap[loc.Slot][loc.Port] = append(slotMap[loc.Slot][loc.Port], loc)
	}

	log.Printf("[Topology] Found %d slots with ONTs", len(slotMap))

	// For each (slot, port) combination, query serial numbers, names, and metrics
	for slot, portMap := range slotMap {
		for port, ontLocations := range portMap {
			queryPortONTAttributes(ipAddress, community, snmpPort, slot, port, ontLocations,
				serials, names, descriptions, deviceTypes, allHWVersions, hwVersions,
				allIPAddresses, ipAddresses, allMACAddresses, macAddresses,
				allMetrics, metrics)
		}
	}

	log.Printf("[Topology] Retrieved %d out of %d ONTs with serial numbers", len(serials), len(statuses))

	topology := buildTopologyStructure(slotMap, statuses, serials, names, descriptions, deviceTypes,
		hwVersions, swVersions, ipAddresses, macAddresses, metrics)

	log.Printf("[Topology] Discovered %d slots with full details", len(topology))
	return topology, nil
}

// buildTopologyStructure assembles the hierarchical topology from attribute maps.

// queryPortONTAttributes queries SNMP tables for a specific slot/port and populates attribute maps.
func queryPortONTAttributes(ipAddress, community string, snmpPort int, slot, port int, ontLocations []ONTLocation,
	serials, names, descriptions, deviceTypes map[ONTLocation]string,
	allHWVersions, hwVersions map[ONTLocation]string,
	allIPAddresses, ipAddresses map[ONTLocation]string,
	allMACAddresses, macAddresses map[ONTLocation]string,
	allMetrics map[ONTLocation]ONTMetrics, metrics map[ONTLocation]*ONTMetrics) {

	// Calculate ifIndex for this slot/port combination
	ifIndexONU := OnuIDIfIndexBase + slot*OnuIDSlotStride + port*OnuIDIncrement
	ifIndexType := OnuTypeIfIndexBase + slot*OnuTypeSlotStride + port*OnuTypeIncrement

	log.Printf("[Topology] Processing slot %d, port %d (ifIndexONU=%d, ifIndexType=%d)", slot, port, ifIndexONU, ifIndexType)

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return
	}
	defer func() { _ = client.Conn.Close() }()

	walkSerialNumbers(client, ifIndexONU, ontLocations, serials)
	walkONUNames(client, ifIndexType, ontLocations, names)
	walkONUDescriptions(client, ifIndexType, ontLocations, descriptions)
	walkONUDeviceTypes(client, ifIndexType, ontLocations, deviceTypes)
	copyONTAttributes(ontLocations, allHWVersions, hwVersions, allIPAddresses, ipAddresses,
		allMACAddresses, macAddresses, allMetrics, metrics, slot, port)
}

func walkSerialNumbers(client *gosnmp.GoSNMP, ifIndexONU int, ontLocations []ONTLocation, serials map[ONTLocation]string) {
	serialOID := BaseOID1 + OnuSerialNumberPrefix + "." + strconv.Itoa(int(ifIndexONU))
	count := 0
	_ = client.Walk(serialOID, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 2 {
			return nil
		}
		onuIDStr := parts[len(parts)-1]
		onuID, err := strconv.Atoi(onuIDStr)
		if err != nil {
			return nil
		}
		for _, loc := range ontLocations {
			if loc.ONTID == onuID {
				serial := ExtractSerialNumber(pdu.Value)
				if serial != "" {
					serials[loc] = serial
					count++
				}
				break
			}
		}
		return nil
	})
}

func walkONUNames(client *gosnmp.GoSNMP, ifIndexType int, ontLocations []ONTLocation, names map[ONTLocation]string) {
	nameOID := BaseOID2 + OnuIDNamePrefix + "." + strconv.Itoa(int(ifIndexType))
	_ = client.Walk(nameOID, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 2 {
			return nil
		}
		onuIDStr := parts[len(parts)-1]
		onuID, err := strconv.Atoi(onuIDStr)
		if err != nil {
			return nil
		}
		for _, loc := range ontLocations {
			if loc.ONTID == onuID {
				name := ExtractName(pdu.Value)
				if name != "" {
					names[loc] = name
				}
				break
			}
		}
		return nil
	})
}

func walkONUDescriptions(client *gosnmp.GoSNMP, ifIndexType int, ontLocations []ONTLocation, descriptions map[ONTLocation]string) {
	descOID := BaseOID2 + OnuDescriptionPrefix + "." + strconv.Itoa(int(ifIndexType))
	_ = client.Walk(descOID, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 2 {
			return nil
		}
		onuIDStr := parts[len(parts)-1]
		onuID, err := strconv.Atoi(onuIDStr)
		if err != nil {
			return nil
		}
		for _, loc := range ontLocations {
			if loc.ONTID == onuID {
				desc := ExtractName(pdu.Value)
				if desc != "" {
					descriptions[loc] = desc
				}
				break
			}
		}
		return nil
	})
}

func walkONUDeviceTypes(client *gosnmp.GoSNMP, ifIndexType int, ontLocations []ONTLocation, deviceTypes map[ONTLocation]string) {
	typeOID := BaseOID2 + OnuTypePrefix + "." + strconv.Itoa(int(ifIndexType))
	_ = client.Walk(typeOID, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 2 {
			return nil
		}
		onuIDStr := parts[len(parts)-1]
		onuID, err := strconv.Atoi(onuIDStr)
		if err != nil {
			return nil
		}
		for _, loc := range ontLocations {
			if loc.ONTID == onuID {
				deviceType := ExtractName(pdu.Value)
				if deviceType != "" {
					deviceTypes[loc] = deviceType
				}
				break
			}
		}
		return nil
	})
}

func copyONTAttributes(ontLocations []ONTLocation,
	allHWVersions, hwVersions map[ONTLocation]string,
	allIPAddresses, ipAddresses map[ONTLocation]string,
	allMACAddresses, macAddresses map[ONTLocation]string,
	allMetrics map[ONTLocation]ONTMetrics, metrics map[ONTLocation]*ONTMetrics,
	slot, port int) {

	for _, loc := range ontLocations {
		if hwVer, ok := allHWVersions[loc]; ok {
			hwVersions[loc] = hwVer
		}
	}
	for _, loc := range ontLocations {
		if ip, ok := allIPAddresses[loc]; ok {
			ipAddresses[loc] = ip
		}
	}
	for _, loc := range ontLocations {
		if mac, ok := allMACAddresses[loc]; ok {
			macAddresses[loc] = mac
		}
	}
	for _, loc := range ontLocations {
		if m, ok := allMetrics[loc]; ok {
			if m.RxPower != nil || m.TxPower != nil || m.Distance > 0 {
				metricsCopy := m
				metrics[loc] = &metricsCopy
			}
		}
	}
}
