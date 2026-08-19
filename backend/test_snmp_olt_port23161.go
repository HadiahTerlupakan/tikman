//go:build ignore

package main

import (
	"fmt"
	"log"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	fmt.Println("=== Testing ZTE OLT SNMP Port 23161 ===\n")
	
	target := "113.192.1.98"
	port := uint16(23161)  // User confirmed this is correct for OLT
	community := "ufiber2" // From existing tests
	
	log.Printf("Target: %s:%d", target, port)
	log.Printf("Community: %s", community)
	log.Printf("Timeout: 15s, Retries: 3\n")
	
	client := &gosnmp.GoSNMP{
		Target:    target,
		Port:      port,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
	
	err := client.Connect()
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer client.Conn.Close()
	
	fmt.Println("\n✅ Connected to OLT successfully!\n")
	
	// Test basic OID first
	sysOid := "1.3.6.1.2.1.1.1.0"
	result, err := client.Get([]string{sysOid})
	if err != nil {
		log.Printf("Note: sysDescr not available (%v)", err)
		fmt.Println("Continuing with ONT discovery...\n")
	} else {
		fmt.Printf("System: %v\n\n", result.Variables[0].Value)
	}
	
	// Discover ONTs
	onuSerialOID := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"
	
	fmt.Println("=== Discovering ONTs via SNMP Walk ===\n")
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
		
		fmt.Printf("%2d. %-15s | Port: %d | ONT ID: %d | R%d/S%d/S%d | ifindex: %d\n", 
			count, 
			truncate(serialNumber, 15), 
			portID, 
			ontID, 
			rack, 
			shelf, 
			slot, 
			ifindex)
		
		return nil
	})
	
	if err != nil {
		log.Printf("Walk warning: %v", err)
	}
	
	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Total ONTs discovered: %d\n\n", len(discovered))
	
	// Check for duplicates
	seen := make(map[string][]int)
	for i, ont := range discovered {
		seen[ont.serialNumber] = append(seen[ont.serialNumber], i+1)
	}
	
	hasDuplicates := false
	for serial, indices := range seen {
		if len(indices) > 1 {
			fmt.Printf("⚠️  DUPLICATE SERIAL '%s' found at indices: %v\n", serial, indices)
			hasDuplicates = true
		}
	}
	
	if !hasDuplicates {
		fmt.Println("✅ No duplicate serial numbers - good!")
	}
	
	fmt.Println("\n=== Physical Location Analysis ===\n")
	if len(discovered) > 0 {
		// Analyze unique locations
		locations := make(map[[3]int]int)
		for _, ont := range discovered {
			loc := [3]int{ont.rack, ont.shelf, ont.slot}
			locations[loc]++
		}
		
		fmt.Printf("ONTs distributed across %d unique location(s):\n", len(locations))
		for loc, count := range locations {
			fmt.Printf("  Rack %d/Shelf %d/Slot %d : %d ONT(s)\n", 
				loc[0], loc[1], loc[2], count)
		}
		
		// Recommend configuration based on most common location
		fmt.Println("\n🔍 RECOMMENDATION:")
		fmt.Println("Based on discovery, your OLT likely has GPON card in one of these slots.")
		fmt.Println("Configure the OLT entry with the location that matches your hardware.")
		fmt.Println("\nCommon options:")
		for loc := range locations {
			fmt.Printf("  Option: Rack=%d, Shelf=%d, Slot=%d\n", loc[0], loc[1], loc[2])
		}
	} else {
		fmt.Println("❌ No ONTs discovered!")
		fmt.Println("\nPossible causes:")
		fmt.Println("1. SNMP community string wrong (try 'public')")
		fmt.Println("2. Wrong SNMP port (confirmed: 23161?)")
		fmt.Println("3. OLT doesn't support this MIB or walk is blocked")
		fmt.Println("4. All ONTs disconnected")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
