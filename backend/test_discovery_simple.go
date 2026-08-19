//go:build ignore

package main

import (
	"fmt"
	"log"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	fmt.Println("=== ONT Discovery Test ===")
	fmt.Println("\nOLT Info:")
	fmt.Println("  IP: 113.192.1.98")
	fmt.Println("  SNMP Port: 161")
	fmt.Println("  Community: ufiber2\n")
	
	client := &gosnmp.GoSNMP{
		Target:    "113.192.1.98",
		Port:      161,
		Community: "ufiber2",
		Version:   gosnmp.Version2c,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
	
	err := client.Connect()
	if err != nil {
		log.Fatalf("❌ Connect failed: %v", err)
	}
	defer client.Conn.Close()
	
	fmt.Println("✅ Connected to OLT!\n")
	
	// Discover ONTs
	onuSerialOID := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"
	
	var discovered []struct {
		portID       int
		ontID        int
		serialNumber string
		rack         int
		shelf        int
		slot         int
		ifindex      int
	}
	
	count := 0
	err = client.Walk(onuSerialOID, func(pdu gosnmp.SnmpPDU) error {
		count++
		
		var ifindex, ontID int
		n, _ := fmt.Sscanf(pdu.Name, onuSerialOID+".%d.%d", &ifindex, &ontID)
		
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
			ifindex      int
		}{portID, ontID, serialNumber, rack, shelf, slot, ifindex})
		
		if count <= 10 || count >= len(discovered)-5 {
			fmt.Printf("%d. Serial: %-10s | Port: %d | ONT ID: %d | Loc: R%d/S%d/S%d | ifindex: %d\n", 
				count, serialNumber, portID, ontID, rack, shelf, slot, ifindex)
		}
		
		return nil
	})
	
	if err != nil {
		log.Printf("Walk warning: %v", err)
	}
	
	fmt.Printf("\n=== Discovery Summary ===\n")
	fmt.Printf("Total unique ONTs: %d\n", len(discovered))
	fmt.Println()
	
	// Check for duplicate serial numbers
	seenSerials := make(map[string]int)
	for _, ont := range discovered {
		seenSerials[ont.serialNumber]++
	}
	
	hasDuplicates := false
	for serial, count := range seenSerials {
		if count > 1 {
			fmt.Printf("⚠️ WARNING: Duplicate serial '%s' found %d times!\n", serial, count)
			hasDuplicates = true
		}
	}
	
	if !hasDuplicates {
		fmt.Println("✅ No duplicate serial numbers found")
	}
	
	fmt.Println("\n🔍 Recommendation:")
	if len(discovered) > 0 {
		firstOnt := discovered[0]
		fmt.Printf("Configure OLT with:\n")
		fmt.Printf("  Rack: %d\n", firstOnt.rack)
		fmt.Printf("  Shelf: %d\n", firstOnt.shelf)
		fmt.Printf("  Slot: %d\n", firstOnt.slot)
		fmt.Println("\nThis will ensure SNMP queries use correct ifindex!")
	} else {
		fmt.Println("No ONTs discovered - check OLT connectivity and SNMP config")
	}
}
