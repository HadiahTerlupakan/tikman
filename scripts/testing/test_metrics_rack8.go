package main

import (
	"fmt"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	port := 1
	ontID := 18
	
	// Use correct rack=8 from discovery
	rack := 8
	shelf := 0
	slot := 24
	ifindex := (rack << 25) | (shelf << 19) | (slot << 13) | (port << 8)
	
	fmt.Printf("Testing Port %d, ONT %d\n", port, ontID)
	fmt.Printf("Rack=%d, Shelf=%d, Slot=%d, Port=%d\n", rack, shelf, slot, port)
	fmt.Printf("Calculated ifindex: %d (0x%X)\n\n", ifindex, ifindex)
	
	client := &gosnmp.GoSNMP{
		Target:    "113.192.1.98",
		Port:      161,
		Community: "ufiber2",
		Version:   gosnmp.Version2c,
		Timeout:   10 * time.Second,
		Retries:   2,
	}
	
	err := client.Connect()
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		return
	}
	defer client.Conn.Close()
	
	// Test rx_power
	oid := fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.5.%d.%d", ifindex, ontID)
	fmt.Printf("Testing rx_power OID: %s\n", oid)
	
	result, err := client.Get([]string{oid})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if len(result.Variables) > 0 {
		v := result.Variables[0]
		fmt.Printf("Type: %v\n", v.Type)
		fmt.Printf("Value: %v\n", v.Value)
	}
}
