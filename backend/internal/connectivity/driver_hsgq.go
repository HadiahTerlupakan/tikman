package connectivity

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/models"
)

// HSGQ SNMP dialect, enterprise 50224.
//
// SOURCE: verified against a live HSGQ-XE08ID EPON OLT on 2026-08-22 by walking
// the whole enterprise subtree (10,899 OIDs, 246 ONUs across 6 PON ports) and
// cross-checking every decoded value against the device's own ONU naming.
//
// This replaces an earlier set of OIDs taken from github.com/fajriyandi/Go-SNMP-HSGQ,
// which placed the ONU table at .3.12.2.1. That subtree does not exist on this
// firmware - it returned zero rows - so the reference was wrong for this model.
// The column numbering inside the optical table did carry over (4=rx, 5=tx,
// 6=bias, 7=voltage), only the table bases differ.
//
// Everything below is observed, not inferred. Where the device exposes nothing
// (per-ONU traffic rates, an autofind table) the driver reports ErrUnsupported
// rather than deriving a number from an unrelated column.
const (
	hsgqBase = ".1.3.6.1.4.1.50224"

	// hsgqONUTable holds one row per ONU, keyed by ifIndex.
	hsgqONUTable = hsgqBase + ".3.3.2.1"

	// hsgqOpticalTable holds transceiver readings, keyed by ifIndex plus a
	// two-arc tail identifying which end of the link the reading belongs to.
	hsgqOpticalTable = hsgqBase + ".3.3.3.1"

	hsgqONUName     = hsgqONUTable + ".2"  // "ONU01/01"
	hsgqONUStatus   = hsgqONUTable + ".8"  // 1 = online, 2 = offline
	hsgqONUMAC      = hsgqONUTable + ".7"  // 6 raw bytes
	hsgqONUHardware = hsgqONUTable + ".12" // "V6.0"
	hsgqONUFirmware = hsgqONUTable + ".13" // "V6.0.3P1T1"
	hsgqONUDistance = hsgqONUTable + ".15" // metres
	hsgqONUVendor   = hsgqONUTable + ".25" // "ZTE"
	hsgqONUModel    = hsgqONUTable + ".26" // "F460910"

	hsgqRxPower     = hsgqOpticalTable + ".4"
	hsgqTxPower     = hsgqOpticalTable + ".5"
	hsgqBiasCurrent = hsgqOpticalTable + ".6"
	hsgqVoltage     = hsgqOpticalTable + ".7"
	hsgqTemperature = hsgqOpticalTable + ".8"

	// hsgqONUTail selects the ONU end of an optical row. The OLT end of the same
	// row carries .65535.65535 and belongs to the PON port, not to any ONT: on
	// the verified device all 6 of those rows were PON ports and all 208 ONU-side
	// rows used .0.0.
	hsgqONUTail = "0.0"

	// hsgqCentiScale converts the hundredths HSGQ reports into base units: dBm
	// for power, mA for bias current, V for voltage, degrees C for temperature.
	hsgqCentiScale = 100.0

	// Raw status values, confirmed by correlating against optical presence:
	// every one of the 208 ONUs reporting 1 had a plausible optical level, and
	// none of the 38 reporting 2 had any reading at all.
	hsgqStatusOnline  = 1
	hsgqStatusOffline = 2
)

// Per-ONU traffic counters live in the standard IF-MIB, not in the enterprise
// tree, and under a completely different ifIndex: the enterprise tables key
// ONU01/01 as 0x01000101 (16777473) while IF-MIB calls the same ONU 17.
//
// That IF-MIB index is assigned dynamically and must be resolved by name on
// every read. Only ONUs the OLT currently has registered appear: on the verified
// device 212 of 246 were listed, and all 34 absent ones were offline. The indexes
// are contiguous, so they shift as ONUs come and go - caching a mapping would
// eventually bill one subscriber's traffic to another.
const (
	ifDescrOID = ".1.3.6.1.2.1.2.2.1.2"

	// 64-bit counters, preferred over the 32-bit ifInOctets/ifOutOctets which
	// wrap within hours on a subscriber link.
	ifHCInOctetsOID     = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCInUcastPktsOID  = ".1.3.6.1.2.1.31.1.1.1.7"
	ifHCOutOctetsOID    = ".1.3.6.1.2.1.31.1.1.1.10"
	ifHCOutUcastPktsOID = ".1.3.6.1.2.1.31.1.1.1.11"

	ifInErrorsOID  = ".1.3.6.1.2.1.2.2.1.14"
	ifOutErrorsOID = ".1.3.6.1.2.1.2.2.1.20"
)

// hsgqONUNamePattern matches the interface names the OLT gives its ONUs, e.g.
// "ONU01/01" for PON port 1, ONU 1. This is the only link between an IF-MIB row
// and a physical position, so it is parsed rather than assumed.
var hsgqONUNamePattern = regexp.MustCompile(`^ONU(\d+)/(\d+)$`)

// toUint64 reads an SNMP counter without the signed round-trip toInt64 performs.
// Counter64 arrives as uint64, and casting it through int64 would turn a value
// above 2^63 into a negative and then into an absurd byte total.
func toUint64(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint32:
		return uint64(v), true
	case uint:
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

// hsgqDriver reads an HSGQ OLT.
type hsgqDriver struct{}

func init() {
	Register(hsgqDriver{})
}

func (hsgqDriver) Model() models.OLTModel { return models.OLTModelHSGQ }

// hsgqLocation decodes an ONU ifIndex into its physical position. The layout is
// 0x0100PPNN, where PP is the PON port and NN the ONU id on that port. This was
// verified against the device's own ONU names for all 246 ONUs with no
// mismatch: ifIndex 0x01000101 is named "ONU01/01", 0x01000102 is "ONU01/02".
//
// Slot stays 0 because the XE08ID is a fixed 8-PON unit with no card slots;
// there is no slot number to report and inventing one would be noise.
func hsgqLocation(ifIndex uint32) ONTLocation {
	return ONTLocation{
		Port:  int((ifIndex >> 8) & 0xFF),
		ONTID: int(ifIndex & 0xFF),
	}
}

// hsgqDecodeStatus maps a raw HSGQ status onto the canonical vocabulary. An
// unrecognised value stays Unknown rather than defaulting to offline, so a
// firmware that adds a third state shows up as unknown instead of silently
// reporting an outage.
func hsgqDecodeStatus(raw int64) int {
	switch raw {
	case hsgqStatusOnline:
		return PhaseStateOnline
	case hsgqStatusOffline:
		return PhaseStateOffline
	default:
		return PhaseStateUnknown
	}
}

// hsgqDecodePower converts hundredths of a dBm into dBm, rejecting readings
// outside what a PON link can physically produce.
//
// The device has no no-signal sentinel: an ONU with no light simply has no row
// in the optical table at all (208 rows for the 208 online ONUs). The range
// check therefore guards against a malformed value rather than a known
// placeholder, and zero is still refused because a fabricated 0.00 dBm would
// render as a perfect signal on a dark fibre.
func hsgqDecodePower(raw int64) *float64 {
	if raw == 0 {
		return nil
	}

	dbm := float64(raw) / hsgqCentiScale
	if dbm < -50 || dbm > 20 {
		return nil
	}

	return &dbm
}

func (hsgqDriver) WalkStatuses(ipAddress, community string, snmpPort int) (map[ONTLocation]int, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	statuses := make(map[ONTLocation]int)
	err = hsgqWalkColumn(client, hsgqONUStatus, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		raw, ok := toInt64(pdu.Value)
		if !ok {
			return
		}
		statuses[hsgqLocation(ifIndex)] = hsgqDecodeStatus(raw)
	})
	if err != nil {
		return nil, fmt.Errorf("HSGQ status walk failed: %w", err)
	}

	log.Printf("[HSGQ] Collected %d ONU entries from the status table", len(statuses))
	return statuses, nil
}

func (hsgqDriver) WalkMetrics(ipAddress, community string, snmpPort int) (map[ONTLocation]ONTMetrics, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	metrics := make(map[ONTLocation]ONTMetrics)
	update := func(ifIndex uint32, apply func(*ONTMetrics)) {
		loc := hsgqLocation(ifIndex)
		m := metrics[loc]
		apply(&m)
		metrics[loc] = m
	}

	numeric := func(table, tail string, apply func(*ONTMetrics, int64)) {
		if err := hsgqWalkColumn(client, table, tail, func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
			raw, ok := toInt64(pdu.Value)
			if !ok {
				return
			}
			update(ifIndex, func(m *ONTMetrics) { apply(m, raw) })
		}); err != nil {
			log.Printf("[HSGQ] Walk of %s failed: %v", table, err)
		}
	}

	numeric(hsgqRxPower, hsgqONUTail, func(m *ONTMetrics, raw int64) { m.RxPower = hsgqDecodePower(raw) })
	numeric(hsgqTxPower, hsgqONUTail, func(m *ONTMetrics, raw int64) { m.TxPower = hsgqDecodePower(raw) })
	numeric(hsgqBiasCurrent, hsgqONUTail, func(m *ONTMetrics, raw int64) {
		m.TxBiasCurrent = float64(raw) / hsgqCentiScale
	})
	numeric(hsgqVoltage, hsgqONUTail, func(m *ONTMetrics, raw int64) {
		m.Voltage = float64(raw) / hsgqCentiScale
	})
	numeric(hsgqTemperature, hsgqONUTail, func(m *ONTMetrics, raw int64) {
		m.Temperature = float64(raw) / hsgqCentiScale
	})

	// Distance lives in the ONU table, not the optical one, so it is keyed by a
	// bare ifIndex with no tail.
	numeric(hsgqONUDistance, "", func(m *ONTMetrics, raw int64) {
		if raw > 0 {
			m.Distance = int(raw)
		}
	})

	// Traffic counters come from the IF-MIB under a different ifIndex, so they
	// are attached separately once the position mapping is resolved.
	hsgqWalkCounters(client, metrics)

	log.Printf("[HSGQ] Walked metrics for %d ONUs", len(metrics))
	return metrics, nil
}

// hsgqResolveIfMibIndexes maps each ONU's physical position to its current
// IF-MIB ifIndex by reading the interface names the OLT publishes. Called on
// every read because the indexes are reassigned as ONUs register and drop.
func hsgqResolveIfMibIndexes(client *gosnmp.GoSNMP) (map[ONTLocation]uint32, error) {
	indexes := make(map[ONTLocation]uint32)

	err := hsgqWalkColumn(client, ifDescrOID, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		// printableText, not ExtractName: the "/" separating PON port from ONU id
		// is what carries the position, and cleanString strips it.
		match := hsgqONUNamePattern.FindStringSubmatch(printableText(pdu.Value))
		if match == nil {
			return // a PON, GE or XGE port, not an ONU
		}
		port, err := strconv.Atoi(match[1])
		if err != nil {
			return
		}
		onuID, err := strconv.Atoi(match[2])
		if err != nil {
			return
		}
		indexes[ONTLocation{Port: port, ONTID: onuID}] = ifIndex
	})
	if err != nil {
		return nil, fmt.Errorf("HSGQ ifDescr walk failed: %w", err)
	}

	return indexes, nil
}

// hsgqWalkCounters adds IF-MIB traffic counters to metrics already keyed by
// physical position.
//
// Direction follows IF-MIB's own convention, which is relative to the OLT: In is
// what the OLT received from the ONU (upstream) and Out is what it sent to the
// ONU (downstream). That lines up with how the rest of the application reads
// Rx/Tx, where Tx is the larger, downstream figure for a residential link.
func hsgqWalkCounters(client *gosnmp.GoSNMP, metrics map[ONTLocation]ONTMetrics) {
	indexes, err := hsgqResolveIfMibIndexes(client)
	if err != nil {
		log.Printf("[HSGQ] Traffic counters unavailable: %v", err)
		return
	}
	if len(indexes) == 0 {
		log.Printf("[HSGQ] No ONU interfaces in IF-MIB, so no traffic counters")
		return
	}

	// Reverse the mapping once so each counter row can be attributed in one step.
	byIfIndex := make(map[uint32]ONTLocation, len(indexes))
	for loc, ifIndex := range indexes {
		byIfIndex[ifIndex] = loc
	}

	counter := func(oid string, apply func(*ONTMetrics, uint64)) {
		if err := hsgqWalkColumn(client, oid, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
			loc, ok := byIfIndex[ifIndex]
			if !ok {
				return // a physical port, or an ONU that dropped mid-walk
			}
			raw, ok := toUint64(pdu.Value)
			if !ok {
				return
			}
			m := metrics[loc]
			apply(&m, raw)
			metrics[loc] = m
		}); err != nil {
			log.Printf("[HSGQ] Walk of %s failed: %v", oid, err)
		}
	}

	counter(ifHCInOctetsOID, func(m *ONTMetrics, v uint64) { m.RxBytes = v })
	counter(ifHCOutOctetsOID, func(m *ONTMetrics, v uint64) { m.TxBytes = v })
	counter(ifHCInUcastPktsOID, func(m *ONTMetrics, v uint64) { m.RxPackets = v })
	counter(ifHCOutUcastPktsOID, func(m *ONTMetrics, v uint64) { m.TxPackets = v })
	counter(ifInErrorsOID, func(m *ONTMetrics, v uint64) { m.RxErrors = v })
	counter(ifOutErrorsOID, func(m *ONTMetrics, v uint64) { m.TxErrors = v })

	log.Printf("[HSGQ] Attached traffic counters for %d of %d ONUs", len(indexes), len(metrics))
}

func (hsgqDriver) Inventory(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTInventory, error) {
	inventory := make(map[ONTLocation]ONTInventory, len(locations))
	if len(locations) == 0 {
		return inventory, nil
	}

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return inventory, err
	}
	defer func() { _ = client.Conn.Close() }()

	wanted := make(map[ONTLocation]bool, len(locations))
	for _, loc := range locations {
		wanted[loc] = true
	}

	update := func(ifIndex uint32, apply func(*ONTInventory)) {
		loc := hsgqLocation(ifIndex)
		if !wanted[loc] {
			return
		}
		inv := inventory[loc]
		apply(&inv)
		inventory[loc] = inv
	}

	// printableText, not ExtractName: these values carry punctuation that matters.
	// ExtractName would render firmware "V6.0.3P1T1" as "V603P1T1" and the ONU
	// name "ONU01/01" as "ONU0101".
	text := func(table string, apply func(*ONTInventory, string)) {
		if err := hsgqWalkColumn(client, table, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
			if value := printableText(pdu.Value); value != "" {
				update(ifIndex, func(inv *ONTInventory) { apply(inv, value) })
			}
		}); err != nil {
			log.Printf("[HSGQ] Walk of %s failed: %v", table, err)
		}
	}

	// This is EPON: an ONU is identified by its MAC, not by a GPON-style serial
	// number, and the device exposes no SN column. The MAC therefore fills both
	// fields - SerialNumber because that is the identity the rest of the
	// application keys ONTs on, MACAddress because that is what the value is.
	if err := hsgqWalkColumn(client, hsgqONUMAC, "", func(ifIndex uint32, pdu gosnmp.SnmpPDU) {
		if mac := hsgqFormatMAC(pdu.Value); mac != "" {
			update(ifIndex, func(inv *ONTInventory) {
				inv.MACAddress = mac
				inv.SerialNumber = strings.ReplaceAll(mac, ":", "")
			})
		}
	}); err != nil {
		log.Printf("[HSGQ] MAC walk failed: %v", err)
	}

	text(hsgqONUName, func(inv *ONTInventory, v string) { inv.Name = v })
	text(hsgqONUModel, func(inv *ONTInventory, v string) { inv.DeviceType = v })
	text(hsgqONUFirmware, func(inv *ONTInventory, v string) { inv.SoftwareVersion = v })
	text(hsgqONUHardware, func(inv *ONTInventory, v string) { inv.HardwareVersion = v })

	// The three IpAddress columns in the ONU table read 0.0.0.0 for every ONU on
	// the verified device, so management IP is left empty rather than recorded as
	// an unroutable address.
	return inventory, nil
}

// InventoryByPort reports the whole result in one instalment. HSGQ indexes the
// ONU table by a flat ifIndex with no per-port subtree to scope a walk to, so
// there is nothing to report until every column has been read.
func (d hsgqDriver) InventoryByPort(ipAddress, community string, snmpPort int, locations []ONTLocation, report func([]ONTLocation, map[ONTLocation]ONTInventory)) error {
	inventory, err := d.Inventory(ipAddress, community, snmpPort, locations)
	if err != nil {
		return err
	}
	report(locations, inventory)
	return nil
}

// hsgqFormatMAC renders the 6 raw bytes of an ONU MAC as colon-separated hex.
func hsgqFormatMAC(value any) string {
	raw, ok := value.([]byte)
	if !ok {
		if s, isString := value.(string); isString {
			raw = []byte(s)
		} else {
			return ""
		}
	}

	if len(raw) != 6 {
		return ""
	}

	parts := make([]string, 0, 6)
	for _, b := range raw {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}

	return strings.Join(parts, ":")
}

// QueryONTMetrics reads one ONU's readings on demand. The ifIndex is rebuilt
// from the physical position, which is the inverse of hsgqLocation.
func (hsgqDriver) QueryONTMetrics(ipAddress, community string, snmpPort, _, port, ontID int) (*ONTMetrics, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ifIndex := hsgqIfIndexFor(port, ontID)

	get := func(oid string) (int64, bool) {
		result, err := client.Get([]string{oid})
		if err != nil || len(result.Variables) == 0 {
			return 0, false
		}
		return toInt64(result.Variables[0].Value)
	}

	metrics := &ONTMetrics{}
	optical := fmt.Sprintf(".%d.%s", ifIndex, hsgqONUTail)

	if raw, ok := get(hsgqRxPower + optical); ok {
		metrics.RxPower = hsgqDecodePower(raw)
	}
	if raw, ok := get(hsgqTxPower + optical); ok {
		metrics.TxPower = hsgqDecodePower(raw)
	}
	if raw, ok := get(hsgqBiasCurrent + optical); ok {
		metrics.TxBiasCurrent = float64(raw) / hsgqCentiScale
	}
	if raw, ok := get(hsgqVoltage + optical); ok {
		metrics.Voltage = float64(raw) / hsgqCentiScale
	}
	if raw, ok := get(hsgqTemperature + optical); ok {
		metrics.Temperature = float64(raw) / hsgqCentiScale
	}
	if raw, ok := get(fmt.Sprintf("%s.%d", hsgqONUDistance, ifIndex)); ok && raw > 0 {
		metrics.Distance = int(raw)
	}

	// Traffic counters sit in the IF-MIB under a different, dynamically assigned
	// ifIndex, so the position has to be looked up by interface name rather than
	// computed. An ONU that is offline has no IF-MIB row and simply keeps zero
	// counters.
	if indexes, err := hsgqResolveIfMibIndexes(client); err != nil {
		log.Printf("[HSGQ] Traffic counters unavailable for %d/%d: %v", port, ontID, err)
	} else if ifMibIndex, ok := indexes[ONTLocation{Port: port, ONTID: ontID}]; ok {
		getCounter := func(oid string) (uint64, bool) {
			result, err := client.Get([]string{fmt.Sprintf("%s.%d", oid, ifMibIndex)})
			if err != nil || len(result.Variables) == 0 {
				return 0, false
			}
			return toUint64(result.Variables[0].Value)
		}

		if v, ok := getCounter(ifHCInOctetsOID); ok {
			metrics.RxBytes = v
		}
		if v, ok := getCounter(ifHCOutOctetsOID); ok {
			metrics.TxBytes = v
		}
		if v, ok := getCounter(ifHCInUcastPktsOID); ok {
			metrics.RxPackets = v
		}
		if v, ok := getCounter(ifHCOutUcastPktsOID); ok {
			metrics.TxPackets = v
		}
		if v, ok := getCounter(ifInErrorsOID); ok {
			metrics.RxErrors = v
		}
		if v, ok := getCounter(ifOutErrorsOID); ok {
			metrics.TxErrors = v
		}
	}

	return metrics, nil
}

// hsgqIfIndexFor packs a PON port and ONU id back into the 0x0100PPNN ifIndex.
func hsgqIfIndexFor(port, ontID int) uint32 {
	return 0x01000000 | uint32(port&0xFF)<<8 | uint32(ontID&0xFF)
}

// WalkTrafficRates: the counter table at .3.10.1.1 is keyed by physical port
// (8 PON plus uplinks), not by ONU, so there is no per-ONU rate to report.
// Dividing a port's counters among its ONUs would invent numbers.
func (hsgqDriver) WalkTrafficRates(string, string, int) (map[ONTLocation]ONUTrafficRates, error) {
	return nil, fmt.Errorf("HSGQ per-ONU traffic rate gauges: %w", ErrUnsupported)
}

func (hsgqDriver) QueryTrafficRates(string, string, int, int, int, int) (*ONUTrafficRates, error) {
	return nil, fmt.Errorf("HSGQ per-ONU traffic rate gauges: %w", ErrUnsupported)
}

// WalkUnconfigured: no table on the verified device clearly lists ONUs detected
// but not yet provisioned. The MAC-keyed table at .3.13.1.1 is a candidate but
// its columns are unidentified, and guessing would present provisioned ONUs as
// awaiting authorisation.
func (hsgqDriver) WalkUnconfigured(context.Context, string, string, int) ([]UnconfiguredONU, error) {
	return nil, fmt.Errorf("HSGQ unconfigured ONU scan: %w", ErrUnsupported)
}

// hsgqWalkColumn walks one column and hands each row's ifIndex and value to
// visit. tail is the arcs expected after the ifIndex ("0.0" for the ONU end of
// an optical row, empty for a plain ifIndex-keyed column); rows with any other
// tail are skipped, which is what keeps the PON-side .65535.65535 readings out
// of an ONT's metrics.
func hsgqWalkColumn(client *gosnmp.GoSNMP, table, tail string, visit func(uint32, gosnmp.SnmpPDU)) error {
	base := strings.TrimPrefix(table, ".") + "."

	return client.Walk(table, func(pdu gosnmp.SnmpPDU) error {
		suffix := strings.TrimPrefix(pdu.Name, ".")
		if !strings.HasPrefix(suffix, base) {
			return nil
		}

		ifIndex, ok := hsgqParseSuffix(strings.TrimPrefix(suffix, base), tail)
		if !ok {
			return nil
		}

		visit(ifIndex, pdu)
		return nil
	})
}

// hsgqParseSuffix reads the ifIndex from an OID suffix, requiring the remaining
// arcs to equal tail exactly.
func hsgqParseSuffix(suffix, tail string) (uint32, bool) {
	index, rest, found := strings.Cut(suffix, ".")
	if !found {
		rest = ""
	}
	if rest != tail {
		return 0, false
	}

	ifIndex, err := strconv.ParseUint(index, 10, 32)
	if err != nil || ifIndex == 0 {
		return 0, false
	}

	return uint32(ifIndex), true
}
