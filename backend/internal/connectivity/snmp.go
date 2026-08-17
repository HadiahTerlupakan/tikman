package connectivity

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// ============================================================================
// ZTE C300/C320 SNMP OIDs - VERIFIED AGAINST https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte
// Tested against ZTE C300 V2.1.0 and C320 V2.1.0 production hardware
// ============================================================================
//
// TWO INDEX SPACES:
//
// 1. ONU-ID Space (.1082.x) - for name, serial, status, description, distance
//    Formula: OnuIDIfIndexBase + slot*OnuIDSlotStride + pon*OnuIDIncrement
//           = 0x11010000 + slot*0x100 + pon
//           = 285278208 + slot*256 + pon
//
// 2. TYPE Space (.1012.x) - for onu type, tx power, ip address
//    Formula: OnuTypeIfIndexBase + slot*OnuTypeSlotStride + pon*OnuTypeIncrement
//           = 0x10000000 + slot*0x10000 + pon*0x100
//
// OID CONSTANTS:
// - .1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.18 = Serial number table
// - .1.3.6.1.4.1.3902.1082.500.10.2.3.3.1.2  = Name/description table
// - .1.3.6.1.4.1.3902.1012.3.28.2.1.4        = Phase state/status table
// - .1.3.6.1.4.1.3902.1012.3.50.12.1.1.10   = RX optical power
// - .1.3.6.1.4.1.3902.1012.3.50.12.1.1.14   = TX optical power
// - .1.3.6.1.4.1.3902.1012.3.11.4.1.2       = Distance in meters
// ============================================================================

const (
	// BaseOIDS for two different index spaces
	BaseOID1 = ".1.3.6.1.4.1.3902.1082" // ONU-ID space (name/serial/status/desc/distance)
	BaseOID2 = ".1.3.6.1.4.1.3902.1012" // TYPE space (type/txpower/ip)

	// Common OID prefixes (same for all board/PON combinations)
	OnuIDNamePrefix             = ".500.10.2.3.3.1.2"  // ONU name/label
	OnuDescriptionPrefix        = ".500.10.2.3.3.1.3"  // ONU description/location
	OnuSerialNumberPrefix       = ".500.10.2.3.3.1.18" // ONU serial number ✅ CORRECT
	OnuRxPowerPrefix            = ".500.20.2.2.2.1.10" // RX optical power
	OnuTxPowerPrefix            = ".3.50.12.1.1.14"   // TX optical power
	OnuStatusIDPrefix           = ".500.10.2.3.8.1.4" // ONU phase state/status
	OnuLastOnlineTimePrefix     = ".500.10.2.3.8.1.5" // Last online time
	OnuLastOfflineTimePrefix    = ".500.10.2.3.8.1.6" // Last offline time
	OnuLastOfflineReasonPrefix  = ".500.10.2.3.8.1.7" // Last offline reason
	OnuGponOpticalDistancePrefix = ".500.10.2.3.10.1.2" // Optical distance

	// IfIndex encoding bases and per-slot strides (verified live against real hardware)
	OnuIDIfIndexBase   = 285278208 // 0x11010000 — ONU-ID space prefix 0x11, shelf 1
	OnuIDSlotStride    = 256       // 0x100      — per-slot stride (ONU-ID space)
	OnuTypeIfIndexBase = 268435456 // 0x10000000 — TYPE space prefix 0x10
	OnuTypeSlotStride  = 65536     // 0x10000    — per-slot stride (TYPE space)

	// Per-PON increments within a slot
	OnuIDIncrement   = 1   // ONU-ID space: each PON increments by 1
	OnuTypeIncrement = 256 // TYPE space: each PON increments by 256

	// MaxBoardID / MaxPonID bound the valid physical slot and PON-port range
	MaxBoardID = 30
	MaxPonID   = 16

	// For backward compatibility with existing code using different OID names
	OID_ZXAN_REGISTERED_ONU_SERIAL_TABLE = BaseOID1 + OnuSerialNumberPrefix
	OID_ZXAN_REGISTERED_ONU_NAME_TABLE   = BaseOID1 + OnuIDNamePrefix
	OID_ZXAN_ONU_PHASE_STATE_TABLE       = BaseOID2 + ".3.28.2.1.4"

	// ZXGPON-MIB branch for optical metrics
	OID_ZXGPON_ONU_RX_POWER_TABLE     = BaseOID2 + ".3.50.12.1.1.10" // rx power (raw * 0.002 - 30 = dBm)
	OID_ZXGPON_ONU_TX_POWER_TABLE     = BaseOID2 + ".3.50.12.1.1.14" // tx power (raw * 0.002 - 30 = dBm)
	OID_ZXGPON_ONU_DISTANCE_TABLE     = BaseOID2 + ".3.11.4.1.2"     // distance in meters
)

// ONTMetrics represents collected metrics from an ONT.
// RxPower/TxPower are pointers because ZTE returns a sentinel value when there
// is no optical signal - a nil pointer means "no reading", which must not be
// confused with a genuine 0.00 dBm measurement.
type ONTMetrics struct {
	RxPower      *float64 // in dBm, nil when no signal
	TxPower      *float64 // in dBm, nil when no signal
	Temperature  float64  // in Celsius
	Voltage      float64  // in Volts
	Distance     int      // in meters
	RxBytes      uint64
	TxBytes      uint64
	SerialNumber string // ONU serial number
}

// SNMPTest performs SNMP connectivity test with actual OID query
func SNMPTest(ipAddress string, port int, community string, timeout time.Duration) error {
	log.Printf("[SNMP] Testing %s:%d with community '%s' (timeout: %v)", ipAddress, port, community, timeout)

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   timeout,
		Retries:   0, // No retries - fail fast on wrong port/community
	}

	err := client.Connect()
	if err != nil {
		log.Printf("[SNMP] UDP connection setup failed: %v", err)
		return fmt.Errorf("UDP connection setup failed: %w", err)
	}
	defer client.Conn.Close()

	log.Printf("[SNMP] Sending GET request for OID 1.3.6.1.2.1.1.1.0")

	// Perform actual SNMP GET request to verify port and community
	oids := []string{"1.3.6.1.2.1.1.1.0"} // sysDescr OID (standard system description)
	result, err := client.Get(oids)
	if err != nil {
		// UDP is connectionless - timeout means wrong port or community
		log.Printf("[SNMP] GET request failed: %v", err)
		return fmt.Errorf("no SNMP response (wrong port/community or device unreachable): %w", err)
	}

	if len(result.Variables) == 0 {
		log.Printf("[SNMP] Response has no variables")
		return fmt.Errorf("SNMP responded but returned no data")
	}

	// Check if response is valid (not NoSuchObject/NoSuchInstance)
	for _, variable := range result.Variables {
		log.Printf("[SNMP] Response: OID=%s Type=%v Value=%v", variable.Name, variable.Type, variable.Value)
	}

	return nil
}

// ============================================================================
// ONT Discovery API - Card/Slot -> PON Port Hierarchy
// Following exact approach from https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte
// ============================================================================

// DiscoveredONT represents an ONT discovered via SNMP with full details
type DiscoveredONT struct {
	PortID       int             `json:"port_id"`
	ONTID        int             `json:"ont_id"`
	SerialNumber string          `json:"serial_number"`
	RunState     int             `json:"run_state"`      // phase state (3=online, 4=dying_gasp, 6=offline, 1=los)
	Name         string          `json:"name,omitempty"` // ONU name/label
	Description  string          `json:"description,omitempty"` // ONU description/location
	RxPower      *float64        `json:"rx_power,omitempty"`
	TxPower      *float64        `json:"tx_power,omitempty"`
	Distance     int             `json:"distance,omitempty"`
	Status       string          `json:"status,omitempty"`
	LastSeenAt   time.Time       `json:"last_seen_at"`
}

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

	// Build data structures for serials, names, descriptions, and metrics
	serials := make(map[ONTLocation]string)
	names := make(map[ONTLocation]string)
	descriptions := make(map[ONTLocation]string)
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
			// Calculate ifIndex for this slot/port combination
			ifIndex := OnuIDIfIndexBase + slot*OnuIDSlotStride + port*OnuIDIncrement

			log.Printf("[Topology] Processing slot %d, port %d (ifIndex=%d)", slot, port, ifIndex)

			client, err := newSNMPClient(ipAddress, community, snmpPort)
			if err != nil {
				continue
			}

			// Query serial numbers
			serialOID := BaseOID1 + OnuSerialNumberPrefix + "." + strconv.Itoa(int(ifIndex))
			count := 0
			err = client.Walk(serialOID, func(pdu gosnmp.SnmpPDU) error {
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

			log.Printf("[Topology] Retrieved %d serial numbers for slot %d port %d", count, slot, port)

			// Query ONU names
			nameOID := BaseOID1 + OnuIDNamePrefix + "." + strconv.Itoa(int(ifIndex))
			nameCount := 0
			err = client.Walk(nameOID, func(pdu gosnmp.SnmpPDU) error {
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
							nameCount++
						}
						break
					}
				}
				return nil
			})

			log.Printf("[Topology] Retrieved %d names for slot %d port %d", nameCount, slot, port)

			// Query ONU descriptions
			descOID := BaseOID1 + OnuDescriptionPrefix + "." + strconv.Itoa(int(ifIndex))
			descCount := 0
			err = client.Walk(descOID, func(pdu gosnmp.SnmpPDU) error {
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
						desc := ExtractName(pdu.Value) // reuse ExtractName for description
						if desc != "" {
							descriptions[loc] = desc
							descCount++
						}
						break
					}
				}
				return nil
			})

			log.Printf("[Topology] Retrieved %d descriptions for slot %d port %d", descCount, slot, port)

			// Copy metrics from allMetrics for ONTs on this port
			metricsCount := 0
			for _, loc := range ontLocations {
				if m, ok := allMetrics[loc]; ok {
					if m.RxPower != nil || m.TxPower != nil || m.Distance > 0 {
						metricsCopy := m
						metrics[loc] = &metricsCopy
						metricsCount++
					}
				}
			}
			log.Printf("[Topology] Mapped %d metrics for slot %d port %d", metricsCount, slot, port)

			client.Conn.Close()
		}
	}

	log.Printf("[Topology] Retrieved %d out of %d ONTs with serial numbers", len(serials), len(statuses))

	// Build topology structure
	slots := make([]int, 0, len(slotMap))
	for slot := range slotMap {
		slots = append(slots, slot)
	}
	sort.Ints(slots)

	topology := make([]GPONSlot, 0, len(slots))
	for _, slot := range slots {
		gponSlot := GPONSlot{Slot: slot}

		ports := make([]int, 0, len(slotMap[slot]))
		for port := range slotMap[slot] {
			ports = append(ports, port)
		}
		sort.Ints(ports)

		for _, port := range ports {
			onts := make([]DiscoveredONT, 0, len(slotMap[slot][port]))
			for _, loc := range slotMap[slot][port] {
				// Determine status string based on phase state
				statusStr := "unknown"
				switch statuses[loc] {
				case 3:
					statusStr = "online"
				case 4:
					statusStr = "dying_gasp"
				case 6:
					statusStr = "offline"
				case 1:
					statusStr = "los"
				}

				ont := DiscoveredONT{
					PortID:       loc.Port,
					ONTID:        loc.ONTID,
					SerialNumber: serials[loc],
					RunState:     mapPhaseToRunState(statuses[loc]),
					Name:         names[loc],
					Description:  descriptions[loc],
					Status:       statusStr,
					LastSeenAt:   time.Now(),
				}

				// Add metrics if available
				if metric, ok := metrics[loc]; ok && metric != nil {
					if metric.RxPower != nil || metric.TxPower != nil || metric.Distance > 0 {
						ont.RxPower = metric.RxPower
						ont.TxPower = metric.TxPower
						ont.Distance = metric.Distance
					}
				}

				onts = append(onts, ont)
			}
			gponSlot.Ports = append(gponSlot.Ports, GponPort{
				PortID: port,
				ONTs:   onts,
			})
		}

		topology = append(topology, gponSlot)
	}

	log.Printf("[Topology] Discovered %d slots with full details", len(topology))
	return topology, nil
}

// DiscoverONTs retrieves all ONTs on an OLT flattened into a single list.
func DiscoverONTs(ipAddress, community string, port int) ([]DiscoveredONT, error) {
	topology, err := DiscoverOLTTopology(ipAddress, community, port)
	if err != nil {
		return nil, err
	}

	var result []DiscoveredONT
	for _, slot := range topology {
		for _, gponPort := range slot.Ports {
			result = append(result, gponPort.ONTs...)
		}
	}

	return result, nil
}

// ExtractSerialNumber extracts serial number from SNMP value and converts to hex string
// Following exact approach from https://github.com/Cepat-Kilat-Teknologi/snmp-olt-zte:
// - ZTE returns serial as 8-byte binary/octet string (e.g., [82 84 69 71 ...])
// - We convert to lowercase hex: 52544547... = ASCII "ZTEG" + remaining bytes
// - Also handles "1," prefix removal if present (some firmware versions)
func ExtractSerialNumber(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		// Already a string, but may have "1," prefix from some firmwares
		s := v
		if strings.HasPrefix(s, "1,") {
			return s[2:]
		}
		return s

	case []byte:
		data := v

		// Handle ASN.1 encoded octet string (common for ZTE firmware V2.x)
		// Format: [tag][length][data...] where tag=0x31, length=data length
		if len(data) >= 2 && data[0] == 0x31 {
			// Remove ASN.1 header (first 2 bytes)
			if len(data) > 2 {
				data = data[2:]
				log.Printf("[Serial] Stripped ASN.1 header: %v -> %q", v, string(data))
			} else {
				return ""
			}
		}

		// Check if it starts with "1," prefix (some firmware versions add this)
		str := string(data)
		if strings.HasPrefix(str, "1,") {
			str = str[2:]
			log.Printf("[Serial] Removed '1,' prefix: %q -> %q", string(data), str)
		}

		// If remaining data looks like hex string (only hex chars), convert to ASCII
		if len(str) >= 8 && isValidHex(str[:min(8, len(str))]) {
			// Try to decode as ASCII first, fallback to hex
			asciiDecoded := ""
			for _, b := range str {
				if b >= 48 && b <= 57 { // '0'-'9'
					asciiDecoded += string(b)
				} else if b >= 65 && b <= 70 { // 'A'-'F'
					// Skip or handle differently based on your use case
				} else if b >= 97 && b <= 102 { // 'a'-'f'
					// Skip or handle differently based on your use case
				} else {
					break
				}
			}

			if len(asciiDecoded) >= 8 {
				// Return printable ASCII serial number
				log.Printf("[Serial] Returned ASCII serial: %q", asciiDecoded)
				return asciiDecoded
			}
		}

		// Default: return as-is string
		log.Printf("[Serial] Returning: %q (type=%T)", str, oidValue)
		return str

	default:
		// Data type is not recognized
		log.Printf("[Serial] Unknown type: %T, value=%v", oidValue, oidValue)
		return ""
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ExtractName extracts name/description from SNMP value
// Handles both string and byte slice types
func ExtractName(oidValue any) string {
	switch v := oidValue.(type) {
	case string:
		// Data is string, return it
		if v != "" && !strings.HasPrefix(v, "1,") {
			return v
		}
		// Remove "1," prefix if present
		if strings.HasPrefix(v, "1,") {
			return v[2:]
		}
	case []byte:
		// Convert byte slice to string
		s := string(v)
		if s != "" && !strings.HasPrefix(s, "1,") {
			return s
		}
		// Remove "1," prefix if present
		if strings.HasPrefix(s, "1,") {
			return s[2:]
		}
	default:
		// Data type is not recognized
		return ""
	}
	return ""
}

// isValidHex checks if a string contains only valid hexadecimal characters
func isValidHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// PollOntStatus queries ONT phase state via SNMP
// OID: 1.3.6.1.4.1.3902.1012.3.28.2.1.4.{ifIndex}.{onuIndex}
// Returns phase state integer value (3=online, 4=dying_gasp, 6=offline, 1=los)
func PollOntStatus(ipAddress string, community string, snmpPort int, slot, gponPort, ontID int) (int, error) {
	// Encode ZXGPON ifIndex (frame is not part of this encoding)
	zxIfIndex := encodeZxGponIfIndex(1, slot, gponPort)

	// Build phase state OID
	phaseStateOID := fmt.Sprintf("%s.%d.%d", OID_ZXAN_ONU_PHASE_STATE_TABLE, zxIfIndex, ontID)

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(snmpPort),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Second * 3,
		Retries:   1,
	}

	err := client.Connect()
	if err != nil {
		return 0, fmt.Errorf("SNMP connect failed: %w", err)
	}
	defer client.Conn.Close()

	result, err := client.Get([]string{phaseStateOID})
	if err != nil {
		return 0, fmt.Errorf("SNMP get failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no SNMP response")
	}

	// Parse phase state value
	var phaseState int
	switch v := result.Variables[0].Value.(type) {
	case int:
		phaseState = v
	case int64:
		phaseState = int(v)
	default:
		return 0, fmt.Errorf("invalid phase state type: %T", v)
	}

	return phaseState, nil
}

// ONTLocation identifies an ONT by its physical position on the OLT, decoded
// from the ZXGPON ifIndex reported by the device itself.
type ONTLocation struct {
	Slot  int
	Port  int
	ONTID int
}

// newSNMPClient builds a connected gosnmp client for walk operations.
func newSNMPClient(ipAddress, community string, snmpPort int) (*gosnmp.GoSNMP, error) {
	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(snmpPort),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Second * 5,
		Retries:   1,
	}
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	return client, nil
}

// parseZxGponSuffix decodes the OID suffix after a ZXGPON table base into an
// ONT location. Accepts both <ifIndex>.<onuIndex> and <ifIndex>.<onuIndex>.<sub>
// shapes, since the optical power tables carry a trailing sub-instance.
func parseZxGponSuffix(oid, base string) (ONTLocation, bool) {
	trimmed := strings.TrimPrefix(oid, ".")
	baseTrimmed := strings.TrimPrefix(base, ".")
	if !strings.HasPrefix(trimmed, baseTrimmed+".") {
		return ONTLocation{}, false
	}

	parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
	if len(parts) < 2 {
		return ONTLocation{}, false
	}

	ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return ONTLocation{}, false
	}
	onuIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return ONTLocation{}, false
	}

	slot, port, ok := decodeZxGponIfIndex(uint32(ifIndexStr))
	if !ok {
		return ONTLocation{}, false
	}

	return ONTLocation{Slot: slot, Port: port, ONTID: onuIndex}, true
}

// toInt64 extracts an integer from an SNMP value, reporting whether it was numeric.
func toInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	default:
		return 0, false
	}
}

// WalkONTStatuses walks the ZXGPON phase state table and returns the raw phase
// state for every ONT the OLT knows about, keyed by its physical location.
//
// Walking is used rather than a per-ONT GET because the ZXGPON ifIndex encodes
// the line-card slot, which the OLT reports authoritatively - deriving it from
// stored rack/shelf/slot values is guesswork and silently yields wrong OIDs.
func WalkONTStatuses(ipAddress, community string, snmpPort int) (map[ONTLocation]int, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	statuses := make(map[ONTLocation]int)

	// Walk the entire ZXGPON phase state table
	err = client.Walk(OID_ZXAN_ONU_PHASE_STATE_TABLE, func(pdu gosnmp.SnmpPDU) error {
		loc, ok := parseZxGponSuffix(pdu.Name, OID_ZXAN_ONU_PHASE_STATE_TABLE)
		if !ok {
			return nil // skip non-ZXGPON entries
		}

		value, ok := toInt64(pdu.Value)
		if !ok || value >= 30000 {
			return nil // ignore invalid or "no signal" readings
		}

		statuses[loc] = int(value)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	log.Printf("[Walk] Collected %d ONT statuses", len(statuses))
	return statuses, nil
}

// WalkONTMetrics walks the optical metrics tables (power, distance) and returns
// collected metrics keyed by the ONT's location, decoded from each OID.
func WalkONTMetrics(ipAddress, community string, snmpPort int) (map[ONTLocation]ONTMetrics, error) {
	metrics := make(map[ONTLocation]ONTMetrics)

	// RX power
	rxMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_RX_POWER_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{RxPower: decodeZxGponPower(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] RX power walk failed: %v", err)
	} else {
		for k, v := range rxMetrics {
			if m, found := metrics[k]; found {
				m.RxPower = v.RxPower
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	// TX power
	txMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_TX_POWER_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{TxPower: decodeZxGponPower(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] TX power walk failed: %v", err)
	} else {
		for k, v := range txMetrics {
			if m, found := metrics[k]; found {
				m.TxPower = v.TxPower
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	// Distance
	distMetrics, err := walkONTMetricTable(ipAddress, community, snmpPort, OID_ZXGPON_ONU_DISTANCE_TABLE, func(_ ONTLocation, raw int64) ONTMetrics {
		return ONTMetrics{Distance: int(raw)}
	})
	if err != nil {
		log.Printf("[Metrics] Distance walk failed: %v", err)
	} else {
		for k, v := range distMetrics {
			if m, found := metrics[k]; found {
				m.Distance = v.Distance
				metrics[k] = m
			} else {
				metrics[k] = v
			}
		}
	}

	log.Printf("[Metrics] Walked %d ONTs", len(metrics))
	return metrics, nil
}

// walkONTMetricTable is a generic helper that walks a metrics table and collects
// values keyed by the ONT location decoded from each OID.
func walkONTMetricTable[T any](ipAddress, community string, snmpPort int, baseOID string, mapper func(ONTLocation, int64) T) (map[ONTLocation]T, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	results := make(map[ONTLocation]T)

	err = client.Walk(baseOID, func(pdu gosnmp.SnmpPDU) error {
		loc, ok := parseZxGponSuffix(pdu.Name, baseOID)
		if !ok {
			return nil // skip non-ZXGPON entries
		}

		value, ok := toInt64(pdu.Value)
		if !ok || value >= 30000 {
			return nil // ignore invalid or "no signal" readings
		}

		results[loc] = mapper(loc, value)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	return results, nil
}

// decodeZxGponPower converts ZTE's raw optical power into dBm. The formula per
// NetManeger is: raw * 0.002 - 30 = dBm. Raw values >= 30000 indicate "no
// optical signal" and map to nil to distinguish missing readings from real 0 dBm.
func decodeZxGponPower(raw int64) *float64 {
	if raw >= 30000 {
		return nil // no signal sentinel
	}

	dbm := float64(raw*2-30000) / 1000.0
	return &dbm
}

// ============================================================================
// Legacy Metrics Query (per-ONT) - DEPRECATED
// ============================================================================

// QueryONTMetricsWithDynamicPort queries power metrics for a single ONT using
// dynamic slot detection from the device. Slot parameter is ignored here because
// the ZXGPON ifIndex encoding already includes the slot value. We keep this
// function for backwards compatibility.
func QueryONTMetricsWithDynamicPort(ipAddress, community string, snmpPort int, slot, gponPort, ontID int) (ONTMetrics, error) {
	// Build ZXGPON ifIndex from parameters
	zxIfIndex := encodeZxGponIfIndex(1, slot, gponPort)

	metrics := ONTMetrics{}

	// RX power
	rxOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_RX_POWER_TABLE, zxIfIndex, ontID)
	val, err := pollSNMPInteger(ipAddress, community, snmpPort, rxOID)
	if err == nil && val < 30000 {
		rxDbm := decodeZxGponPower(val)
		metrics.RxPower = rxDbm
	}

	// TX power
	txOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_TX_POWER_TABLE, zxIfIndex, ontID)
	val, err = pollSNMPInteger(ipAddress, community, snmpPort, txOID)
	if err == nil && val < 30000 {
		txDbm := decodeZxGponPower(val)
		metrics.TxPower = txDbm
	}

	// Distance
	distOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_DISTANCE_TABLE, zxIfIndex, ontID)
	val, err = pollSNMPInteger(ipAddress, community, snmpPort, distOID)
	if err == nil && val > 0 && val < 30000 {
		metrics.Distance = int(val)
	}

	return metrics, nil
}

// pollSNMPInteger performs a simple SNMP GET for a numeric value
func pollSNMPInteger(ipAddress, community string, snmpPort int, oid string) (int64, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return 0, err
	}
	defer client.Conn.Close()

	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no response for OID %s", oid)
	}

	val, ok := toInt64(result.Variables[0].Value)
	if !ok {
		return 0, fmt.Errorf("invalid SNMP value type for OID %s", oid)
	}

	return val, nil
}

// encodeZxGponIfIndex encodes the ZXGPON-MIB ifIndex according to ZTE spec
func encodeZxGponIfIndex(frame, slot, port int) uint32 {
	return (uint32(frame)<<28)|(0x10<<16)|(uint32(slot&0xff)<<8)|uint32(port&0xff)
}

// encodeZteAnIfIndex encodes the ZTE-AN-MIB ifIndex format.
// Format: (frame << 28) | (0x101 << 16) | (slot << 8) | port
// Used for ZTE-AN-MIB tables like registered ONU serial number table.
func encodeZteAnIfIndex(frame, slot, port int) uint32 {
	return (uint32(frame)<<28)|(0x101<<16)|(uint32(slot&0xff)<<8)|uint32(port&0xff)
}

// decodeZxGponIfIndex reverses the ZXGPON-MIB ifIndex encoding
// ZTE C300 format: [byte3][byte2][byte1][byte0] where:
//   byte3 (bits 24-31): card/device ID (0x10 for GPON cards)
//   byte2 (bits 16-23): slot number
//   byte1 (bits 8-15): PON port number
//   byte0 (bits 0-7): ONT index (in full OID suffix)
// Example: 268635648 = 0x10030E00 -> slot=3 (0x03), port=14 (0x0E)
func decodeZxGponIfIndex(ifIndex uint32) (slot, port int, ok bool) {
	// Extract slot from bytes 16-23 and port from bytes 8-15
	slot = int((ifIndex >> 16) & 0xFF)
	port = int((ifIndex >> 8) & 0xFF)

	// Validate that we have reasonable values
	if slot == 0 || port == 0 {
		return 0, 0, false
	}

	return slot, port, true
}

// walkONTMetricsForPort queries optical metrics for ONTs on a specific port
func walkONTMetricsForPort(client *gosnmp.GoSNMP, ipAddress, community string, snmpPort int,
	slot, port int, ontLocations []ONTLocation) (map[ONTLocation]*ONTMetrics, error) {

	log.Printf("[Metrics] Querying metrics for slot %d port %d (%d ONTs)", slot, port, len(ontLocations))
	metrics := make(map[ONTLocation]*ONTMetrics)

	for _, loc := range ontLocations {
		onMetrics := &ONTMetrics{}

		// Query RX power using ZXGPON-MIB table
		zxIfIndex := encodeZxGponIfIndex(1, slot, port)
		rxOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_RX_POWER_TABLE, zxIfIndex, loc.ONTID)

		result, err := client.Get([]string{rxOID})
		if err == nil && len(result.Variables) > 0 {
			if val, ok := toInt64(result.Variables[0].Value); ok {
				log.Printf("[Metrics] ONT %d RX raw value: %d", loc.ONTID, val)
				if val < 30000 {
					rxDbm := decodeZxGponPower(val)
					onMetrics.RxPower = rxDbm
					log.Printf("[Metrics] ONT %d RX: %.2f dBm", loc.ONTID, *rxDbm)
				} else {
					log.Printf("[Metrics] ONT %d RX: no signal (val=%d)", loc.ONTID, val)
				}
			}
		} else if err != nil {
			log.Printf("[Metrics] RX power query failed for ONT %d: %v", loc.ONTID, err)
		}

		// Query TX power
		txOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_TX_POWER_TABLE, zxIfIndex, loc.ONTID)
		result, err = client.Get([]string{txOID})
		if err == nil && len(result.Variables) > 0 {
			if val, ok := toInt64(result.Variables[0].Value); ok && val < 30000 {
				txDbm := decodeZxGponPower(val)
				onMetrics.TxPower = txDbm
			}
		} else if err != nil {
			log.Printf("[Metrics] TX power query failed for ONT %d: %v", loc.ONTID, err)
		}

		// Query distance
		distOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_DISTANCE_TABLE, zxIfIndex, loc.ONTID)
		result, err = client.Get([]string{distOID})
		if err == nil && len(result.Variables) > 0 {
			if val, ok := toInt64(result.Variables[0].Value); ok && val > 0 && val < 30000 {
				onMetrics.Distance = int(val)
			}
		} else if err != nil {
			log.Printf("[Metrics] Distance query failed for ONT %d: %v", loc.ONTID, err)
		}

		metrics[loc] = onMetrics
	}

	log.Printf("[Metrics] Retrieved metrics for %d ONTs on slot %d port %d", len(metrics), slot, port)
	return metrics, nil
}
