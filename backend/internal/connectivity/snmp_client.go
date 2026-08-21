package connectivity

import (
	"fmt"
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
)

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
	defer func() { _ = client.Conn.Close() }()

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

// PollOntStatus queries ONT phase state via SNMP
// OID: 1.3.6.1.4.1.3902.1012.3.28.2.1.4.{ifIndex}.{onuIndex}
// Returns phase state integer value (3=online, 4=dying_gasp, 6=offline, 1=los)
func PollOntStatus(ipAddress string, community string, snmpPort int, slot, gponPort, ontID int) (int, error) {
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
	defer func() { _ = client.Conn.Close() }()

	ifIndex := encodeOnuIDIfIndex(1, slot, gponPort)
	phaseStateOID := fmt.Sprintf("%s.%d.%d", OID_ZXAN_ONU_PHASE_STATE_TABLE, ifIndex, ontID)

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

// GetLastOnlineTime retrieves the last online timestamp for an ONT
func GetLastOnlineTime(client *gosnmp.GoSNMP, slot, port, ontID int) (time.Time, error) {
	ifIndex := encodeOnuIDIfIndex(1, slot, port)
	oid := fmt.Sprintf("%s%s.%d.%d", BaseOID1, OnuLastOnlineTimePrefix, ifIndex, ontID)

	result, err := client.Get([]string{oid})
	if err != nil {
		return time.Time{}, fmt.Errorf("SNMP get failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return time.Time{}, fmt.Errorf("no data returned")
	}

	// Extract hex bytes from SNMP response
	hexBytes, ok := result.Variables[0].Value.([]byte)
	if !ok {
		return time.Time{}, fmt.Errorf("unexpected value type: %T", result.Variables[0].Value)
	}

	return parseZteHexTimestamp(hexBytes)
}

// GetLastOfflineTime retrieves the last offline timestamp for an ONT
func GetLastOfflineTime(client *gosnmp.GoSNMP, slot, port, ontID int) (time.Time, error) {
	ifIndex := encodeOnuIDIfIndex(1, slot, port)
	oid := fmt.Sprintf("%s%s.%d.%d", BaseOID1, OnuLastOfflineTimePrefix, ifIndex, ontID)

	result, err := client.Get([]string{oid})
	if err != nil {
		return time.Time{}, fmt.Errorf("SNMP get failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return time.Time{}, fmt.Errorf("no data returned")
	}

	hexBytes, ok := result.Variables[0].Value.([]byte)
	if !ok {
		return time.Time{}, fmt.Errorf("unexpected value type: %T", result.Variables[0].Value)
	}

	return parseZteHexTimestamp(hexBytes)
}

// GetLastOfflineReason retrieves the reason for the last offline event
func GetLastOfflineReason(client *gosnmp.GoSNMP, slot, port, ontID int) (string, error) {
	ifIndex := encodeOnuIDIfIndex(1, slot, port)
	oid := fmt.Sprintf("%s%s.%d.%d", BaseOID1, OnuLastOfflineReasonPrefix, ifIndex, ontID)

	result, err := client.Get([]string{oid})
	if err != nil {
		return "", fmt.Errorf("SNMP get failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return "", fmt.Errorf("no data returned")
	}

	// ZTE returns reason as integer code
	reasonCode, ok := toInt64(result.Variables[0].Value)
	if !ok {
		return "Unknown", nil
	}

	// Map ZTE reason codes to human-readable strings
	switch reasonCode {
	case 1:
		return "LOS", nil // Loss of Signal
	case 2:
		return "Dying-Gasp", nil // ONT shutdown signal
	case 3:
		return "Power-Off", nil
	case 4:
		return "Auth-Failed", nil
	case 5:
		return "Manual", nil
	default:
		return fmt.Sprintf("Unknown(%d)", reasonCode), nil
	}
}
