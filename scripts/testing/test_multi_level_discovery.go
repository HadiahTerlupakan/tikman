package main

import (
	"fmt"
	"strings"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
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
		fmt.Printf("Connect error: %v\n", err)
		return
	}
	defer client.Conn.Close()
	
	fmt.Println("=== STEP 1: Detecting GPON Cards/Ports ===")
	gponBaseOID := ".1.3.6.1.4.1.3902.1012.3.1.1.2.1.2"
	
	err = client.Walk(gponBaseOID, func(pdu gosnmp.SnmpPDU) error {
		if len(pdu.Name) == 0 {
			return nil
		}
		
		parts := strings.Split(pdu.Name, ".")
		if len(parts) < 12 {
			return nil
		}
		
		ifname := string(pdu.Value.([]byte))
		fmt.Printf("GPON Port found: IFName=%s\n", ifname)
		
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error walking gpon interfaces: %v\n", err)
	}
	
	fmt.Println("\n=== STEP 2: Query ALL registered ONTs via SNMP Walk ===")
	onuSerialOID := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"
	
	count := 0
	err = client.Walk(onuSerialOID, func(pdu gosnmp.SnmpPDU) error {
		count++
		if count <= 20 {
			if name := pdu.Name; len(name) > 0 {
				value, ok := pdu.Value.([]byte)
				if ok && len(value) > 0 && string(value) != "ALL" {
					fmt.Printf("ONT #%d: Serial=%s\nOID=%s\n", count, string(value), name)
				}
			}
		}
		return nil
	})
	
	if err != nil {
		fmt.Printf("Error walking ONU serials: %v\n", err)
	}
	
	fmt.Printf("\nTotal ONTs found: %d\n", count)
}
