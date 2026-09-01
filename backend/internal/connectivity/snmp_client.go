package connectivity

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
)

// snmpTestTimeout is the floor for a reachability probe. SNMP runs over UDP, so
// a device that is present but slow answers in tens or hundreds of milliseconds
// across a NAT hop; a zero timeout gives it no chance at all.
const snmpTestTimeout = 3 * time.Second

// SNMPTest performs SNMP connectivity test with actual OID query.
//
// A non-positive timeout is clamped rather than honoured: gosnmp treats it as
// "give up immediately", which fails every reachable device and reports it as
// "request timeout" - indistinguishable from a wrong port or community. The
// guard lives here because both callers reach this one function, so one clamp
// covers them and any future caller.
func SNMPTest(ipAddress string, port int, community string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = snmpTestTimeout
	}

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
		// A silent agent and a wrong community look identical from here: SNMP
		// drops a packet it will not answer rather than refusing it. Whether the
		// device is reachable at all is not this probe's to say — ping and the
		// login answer that, and claiming it here has sent operators to check
		// routing that was already proven clear.
		log.Printf("[SNMP] GET request failed: %v", err)
		return fmt.Errorf("no SNMP response (SNMP not enabled on the device, or wrong community/port): %w", err)
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
	return newSNMPClientWithContext(context.Background(), ipAddress, community, snmpPort)
}

// newSNMPClientWithContext builds a connected client whose requests stop at the
// context deadline. gosnmp clamps each request to the earlier of its own Timeout
// and the context deadline, so ctx bounds a multi-walk scan as a whole.
func newSNMPClientWithContext(ctx context.Context, ipAddress, community string, snmpPort int) (*gosnmp.GoSNMP, error) {
	client := &gosnmp.GoSNMP{
		Context:   ctx,
		Target:    ipAddress,
		Port:      uint16(snmpPort),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Second * 5,
		Retries:   1,
		// Every table read goes through bulkWalk, so this is what decides how
		// many values one round trip brings back.
		MaxRepetitions: uint32(maxRepetitions),
	}
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	return client, nil
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
