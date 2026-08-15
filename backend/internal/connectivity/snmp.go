package connectivity

import (
	"fmt"
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
)

// ZTE C300 OLT SNMP OIDs for ONT metrics
// Format: OID.{rack}.{shelf}.{slot}.{port}.{ont_id}
const (
	// Discovery OIDs
	OID_ONT_SERIAL_NUMBER = ".1.3.6.1.4.1.3902.1012.3.28.1.1.1" // zxAnPonOnuSerialNo
	OID_ONT_RUN_STATE     = ".1.3.6.1.4.1.3902.1012.3.28.1.1.2" // zxAnPonOnuRunState

	// Metrics OIDs
	OID_ONT_RX_POWER    = ".1.3.6.1.4.1.3902.1012.3.28.1.1.5" // × 0.01 dBm
	OID_ONT_TX_POWER    = ".1.3.6.1.4.1.3902.1012.3.28.1.1.6" // × 0.01 dBm
	OID_ONT_TEMPERATURE = ".1.3.6.1.4.1.3902.1012.3.28.1.1.7" // in Celsius
	OID_ONT_VOLTAGE     = ".1.3.6.1.4.1.3902.1012.3.28.1.1.8" // × 0.01 V
	OID_ONT_DISTANCE    = ".1.3.6.1.4.1.3902.1012.3.28.1.1.9" // in meters
	OID_ONT_RX_BYTES    = ".1.3.6.1.4.1.3902.1012.3.50.13.1.4" // counter64
	OID_ONT_TX_BYTES    = ".1.3.6.1.4.1.3902.1012.3.50.13.1.5" // counter64
)

// ONTMetrics represents collected metrics from an ONT
type ONTMetrics struct {
	RxPower     float64 // in dBm
	TxPower     float64 // in dBm
	Temperature float64 // in Celsius
	Voltage     float64 // in Volts
	Distance    int     // in meters
	RxBytes     uint64
	TxBytes     uint64
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
		if variable.Type == gosnmp.NoSuchObject || variable.Type == gosnmp.NoSuchInstance {
			return fmt.Errorf("SNMP agent does not support standard OIDs")
		}
	}

	log.Printf("[SNMP] Test successful!")
	return nil
}

// QueryONTMetrics queries ONT metrics via SNMP
func QueryONTMetrics(ipAddress, community string, port, ontID int) (*ONTMetrics, error) {
	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      161,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Second * 5,
		Retries:   1,
	}

	err := client.Connect()
	if err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	defer client.Conn.Close()

	// Build OIDs with rack=1, shelf=1, slot=1
	oids := []string{
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_RX_POWER, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TX_POWER, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TEMPERATURE, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_VOLTAGE, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_DISTANCE, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_RX_BYTES, port, ontID),
		fmt.Sprintf("%s.1.1.1.%d.%d", OID_ONT_TX_BYTES, port, ontID),
	}

	result, err := client.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("SNMP GET failed: %w", err)
	}

	if len(result.Variables) != 7 {
		return nil, fmt.Errorf("expected 7 values, got %d", len(result.Variables))
	}

	metrics := &ONTMetrics{
		RxPower:     float64(gosnmp.ToBigInt(result.Variables[0].Value).Int64()) * 0.01,
		TxPower:     float64(gosnmp.ToBigInt(result.Variables[1].Value).Int64()) * 0.01,
		Temperature: float64(gosnmp.ToBigInt(result.Variables[2].Value).Int64()),
		Voltage:     float64(gosnmp.ToBigInt(result.Variables[3].Value).Int64()) * 0.01,
		Distance:    int(gosnmp.ToBigInt(result.Variables[4].Value).Int64()),
		RxBytes:     gosnmp.ToBigInt(result.Variables[5].Value).Uint64(),
		TxBytes:     gosnmp.ToBigInt(result.Variables[6].Value).Uint64(),
	}

	return metrics, nil
}

// DiscoveredONT represents an ONT discovered via SNMP
type DiscoveredONT struct {
	PortID       int
	ONTID        int
	SerialNumber string
	RunState     int // 1=online, 2=offline
}

// DiscoverONTs performs SNMP walk to discover all ONTs on an OLT
func DiscoverONTs(ipAddress, community string, port int) ([]DiscoveredONT, error) {
	log.Printf("[Discovery] Starting ONT discovery on %s:%d", ipAddress, port)

	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   10 * time.Second,
		Retries:   2,
		MaxOids:   60,
	}

	err := client.Connect()
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer client.Conn.Close()

	// Walk serial numbers to discover all ONTs
	discovered := make([]DiscoveredONT, 0)
	err = client.Walk(OID_ONT_SERIAL_NUMBER, func(pdu gosnmp.SnmpPDU) error {
		// Parse OID index from ZTE C300 format
		// Format: .1.3.6.1.4.1.3902.1012.3.28.1.1.1.{ifindex}.{ont_id}
		// Example: .1.3.6.1.4.1.3902.1012.3.28.1.1.1.268635648.22
		// ifindex encodes: (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)

		oidStr := pdu.Name

		// Extract last 2 indices (ifindex.ont_id)
		var ifindex, ontID int
		n, _ := fmt.Sscanf(oidStr, OID_ONT_SERIAL_NUMBER+".%d.%d", &ifindex, &ontID)

		if n != 2 {
			log.Printf("[Discovery] Failed to parse OID: %s", oidStr)
			return nil
		}

		// Decode ifindex to get port number
		// port = (ifindex >> 8) & 0x1F  // Extract bits 8-12 (5 bits for port 0-31)
		portID := (ifindex >> 8) & 0x1F

		// Extract serial number
		serialBytes, ok := pdu.Value.([]byte)
		if !ok {
			log.Printf("[Discovery] Invalid serial number type for ifindex %d ONT %d", ifindex, ontID)
			return nil
		}

		serialNumber := string(serialBytes)
		if len(serialNumber) == 0 || serialNumber == "ALL" {
			return nil // Skip empty or placeholder serial numbers
		}

		log.Printf("[Discovery] Found ONT: ifindex=%d, Port %d, ONT ID %d, Serial: %s", ifindex, portID, ontID, serialNumber)

		discovered = append(discovered, DiscoveredONT{
			PortID:       portID,
			ONTID:        ontID,
			SerialNumber: serialNumber,
			RunState:     1, // Assume online if discovered
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("SNMP walk failed: %w", err)
	}

	log.Printf("[Discovery] Completed: found %d ONTs", len(discovered))
	return discovered, nil
}
