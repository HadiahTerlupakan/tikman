//go:build ignore

package main

import (
	"fmt"
	"log"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	fmt.Println("=== Testing OLT SNMP Configuration ===\n")
	
	target := "113.192.1.98"
	snmpPort := 23161  // From user input
	snmpCommunity := "ufiber2"  // From existing test file
	timeout := 5 * time.Second
	
	log.Printf("Target: %s", target)
	log.Printf("SNMP Port: %d", snmpPort)
	log.Printf("Community: %s", snmpCommunity)
	log.Printf("Timeout: %v\n", timeout)
	
	client := &gosnmp.GoSNMP{
		Target:    target,
		Port:      uint16(snmpPort),
		Community: snmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   timeout,
		Retries:   1,
	}
	
	err := client.Connect()
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer client.Conn.Close()
	
	fmt.Println("\n✅ Connected successfully!\n")
	
	// Test basic system OID
	sysOid := "1.3.6.1.2.1.1.1.0"
	result, err := client.Get([]string{sysOid})
	if err != nil {
		log.Fatalf("Failed to get sysDescr: %v", err)
	}
	fmt.Printf("System Description: %v\n\n", result.Variables[0].Value)
	
	// Now try to discover ONTs
	oidSerial := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"
	
	fmt.Println("=== Discovering ONTs ===\n")
	var discovered []struct {
		portID       int
		ontID        int
		serialNumber string
	}
	
	err = client.Walk(oidSerial, func(pdu gosnmp.SnmpPDU) error {
		var ifindex, ontID int
		n, _ := fmt.Sscanf(pdu.Name, oidSerial+".%d.%d", &ifindex, &ontID)
		
		if n != 2 {
			return nil
		}
		
		portID := (ifindex >> 8) & 0x1F
		
		serialBytes, ok := pdu.Value.([]byte)
		if !ok || len(serialBytes) == 0 {
			return nil
		}
		
		serialNumber := string(serialBytes)
		if serialNumber == "ALL" || serialNumber == "" {
			return nil
		}
		
		discovered = append(discovered, struct {
			portID       int
			ontID        int
			serialNumber string
		}{portID, ontID, serialNumber})
		
		rack := (ifindex >> 25) & 0xF
		shelf := (ifindex >> 19) & 0x7
		slot := (ifindex >> 13) & 0x1F
		
		fmt.Printf("ONT #%d:\n", len(discovered))
		fmt.Printf("  Serial Number: %s\n", serialNumber)
		fmt.Printf("  Port ID: %d\n", portID)
		fmt.Printf("  ONT ID: %d\n", ontID)
		fmt.Printf("  Calculated from ifindex:\n")
		fmt.Printf("    Rack: %d, Shelf: %d, Slot: %d\n", rack, shelf, slot)
		fmt.Printf("    ifindex: %d (0x%X)\n\n", ifindex, ifindex)
		
		return nil
	})
	
	if err != nil {
		log.Printf("Walk completed with warning: %v", err)
	}
	
	fmt.Printf("=== Summary ===\n")
	fmt.Printf("Total ONTs discovered: %d\n", len(discovered))
	
	if len(discovered) == 0 {
		fmt.Println("\n⚠️ No ONTs found!")
		fmt.Println("Check:")
		fmt.Println("1. SNMP community string is correct")
		fmt.Println("2. SNMP port is correct")
		fmt.Println("3. OLT has ONTs connected and online")
		fmt.Println("4. SNMP walk is enabled on OLT")
	}
}
