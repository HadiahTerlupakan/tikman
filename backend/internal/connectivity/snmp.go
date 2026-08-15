package connectivity

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// SNMPTest performs SNMP connectivity test
func SNMPTest(ipAddress string, port int, community string, timeout time.Duration) error {
	client := &gosnmp.GoSNMP{
		Target:    ipAddress,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   timeout,
		Retries:   1,
	}

	err := client.Connect()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer client.Conn.Close()

	// Perform a simple GET request to verify connectivity
	oids := []string{"1.3.6.1.2.1.1.1.0"} // sysDescr OID
	result, err := client.Get(oids)
	if err != nil {
		if err.Error() == "request timeout" || err.Error() == "timeout" {
			return fmt.Errorf("timeout after %s", timeout)
		}
		return fmt.Errorf("SNMP request failed: %w", err)
	}

	if len(result.Variables) == 0 {
		return fmt.Errorf("no response from SNMP agent")
	}

	return nil
}
