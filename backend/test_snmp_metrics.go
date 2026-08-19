//go:build ignore

package main

import (
	"fmt"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	// ONT Port 1, ONT ID 18 dari discovery
	port := 1
	ontID := 18
	
	// Calculate ifindex
	rack := 0
	shelf := 0
	slot := 24
	ifindex := (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)
	
	fmt.Printf("Testing SNMP for Port %d, ONT %d\n", port, ontID)
	fmt.Printf("Calculated ifindex: %d (0x%X)\n", ifindex, ifindex)
	
	client := &gosnmp.GoSNMP{
		Target:    "113.192.1.98",
		Port:      161,
		Community: "ufiber2",
		Version:   gosnmp.Version2c,
		Timeout:   5 * time.Second,
		Retries:   1,
	}
	
	err := client.Connect()
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		return
	}
	defer client.Conn.Close()
	
	// Test OIDs with ifindex
	oids := []string{
		fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.5.%d.%d", ifindex, ontID),  // rx_power
		fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.6.%d.%d", ifindex, ontID),  // tx_power
		fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.9.%d.%d", ifindex, ontID),  // distance
	}
	
	fmt.Println("\nQuerying OIDs:")
	for i, oid := range oids {
		result, err := client.Get([]string{oid})
		if err != nil {
			fmt.Printf("OID %d error: %v\n", i, err)
			continue
		}
		
		if len(result.Variables) > 0 {
			v := result.Variables[0]
			fmt.Printf("OID %d (%s):\n", i, oid)
			fmt.Printf("  Type: %v\n", v.Type)
			fmt.Printf("  Value: %v\n", v.Value)
		}
	}
	
	// Also test with the ifindex we know works from discovery (268632320)
	fmt.Printf("\n\nTesting with known working ifindex from discovery: 268632320\n")
	oid := fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.5.268632320.%d", ontID)
	result, err := client.Get([]string{oid})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if len(result.Variables) > 0 {
		v := result.Variables[0]
		fmt.Printf("OID: %s\n", oid)
		fmt.Printf("Type: %v\n", v.Type)
		fmt.Printf("Value: %v\n", v.Value)
	}
}
