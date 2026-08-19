//go:build ignore

package main

import (
	"fmt"
	"log"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	fmt.Println("=== Testing OLT SNMP Port 161 ===\n")
	
	target := "113.192.1.98"
	snmpPort := 161  // Standard SNMP port
	snmpCommunity := "ufiber2"  
	timeout := 5 * time.Second
	
	log.Printf("Target: %s", target)
	log.Printf("SNMP Port: %d", snmpPort)
	log.Printf("Community: %s\n", snmpCommunity)
	
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
	
	// Test basic OID
	sysOid := "1.3.6.1.2.1.1.1.0"
	result, err := client.Get([]string{sysOid})
	if err != nil {
		log.Fatalf("Failed to get sysDescr: %v", err)
	}
	fmt.Printf("System Description: %v\n\n", result.Variables[0].Value)
	
	// Discover ONTs
	oidSerial := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"
	
	fmt.Println("=== Discovering ONTs ===\n")
	var discovered []struct {
		portID       int
		ontID        int
		serialNumber string
		rack         int
		shelf        int
		slot         int
	}
	
	err = client.Walk(oidSerial, func(pdu gosnmp.SnmpPDU) error {
		var ifindex, ontID int
		n, _ := fmt.Sscanf(pdu.Name, oidSerial+".%d.%d", &ifindex, &ontID)
		
		if n != 2 {
			return nil
		}
		
		portID := (ifindex >> 8) & 0x1F
		rack := (ifindex >> 25) & 0xF
		shelf := (ifindex >> 19) & 0x7
		slot := (ifindex >> 13) & 0x1F
		
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
			rack         int
			shelf        int
			slot         int
		}{portID, ontID, serialNumber, rack, shelf, slot})
		
		fmt.Printf("ONT #%d:\n", len(discovered))
		fmt.Printf("  Serial: %s\n", serialNumber)
		fmt.Printf("  Port: %d | ONT ID: %d\n", portID, ontID)
		fmt.Printf("  Location: Rack %d, Shelf %d, Slot %d\n", rack, shelf, slot)
		fmt.Printf("  ifindex: %d\n\n", ifindex)
		
		return nil
	})
	
	if err != nil {
		log.Printf("Walk warning: %v", err)
	}
	
	fmt.Printf("=== Results ===\n")
	fmt.Printf("Total ONTs: %d\n\n", len(discovered))
	
	if len(discovered) > 0 {
		fmt.Println("Now testing metrics collection for each ONT...\n")
		
		for i, ont := range discovered {
			fmt.Printf("Testing ONT #%d (%s):\n", i+1, ont.serialNumber)
			
			// Test with the calculated ifindex from discovery
			ifindex := (ont.rack << 25) | (ont.shelf << 19) | (ont.slot << 13) | (ont.portID << 8)
			
			oids := []string{
				fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.5.%d.%d", ifindex, ont.ontID),
				fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.6.%d.%d", ifindex, ont.ontID),
			}
			
			for j, oid := range oids {
				result, err := client.Get([]string{oid})
				if err != nil {
					fmt.Printf("  OID%d error: %v\n", j, err)
				} else {
					val := result.Variables[0]
					if j == 0 {
						fmt.Printf("  Rx Power: %.2f dBm\n", float64(gosnmp.ToBigInt(val.Value).Int64())*0.01)
					} else {
						fmt.Printf("  Tx Power: %.2f dBm\n", float64(gosnmp.ToBigInt(val.Value).Int64())*0.01)
					}
				}
			}
			fmt.Println()
		}
	}
}
