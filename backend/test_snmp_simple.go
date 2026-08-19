//go:build ignore

package main

import (
	"fmt"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
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
	
	// Test dengan OID yang kita tahu works dari discovery sebelumnya
	// Serial number untuk ifindex 268632320, ONT 18
	oid := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1.268632320.18"
	fmt.Printf("Testing OID: %s\n", oid)
	
	result, err := client.Get([]string{oid})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	if len(result.Variables) > 0 {
		v := result.Variables[0]
		fmt.Printf("Success! Type=%v Value=%v\n", v.Type, v.Value)
	}
	
	// Now test rx_power
	oid = ".1.3.6.1.4.1.3902.1012.3.28.1.1.5.268632320.18"
	fmt.Printf("\nTesting rx_power OID: %s\n", oid)
	
	result, err = client.Get([]string{oid})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	if len(result.Variables) > 0 {
		v := result.Variables[0]
		fmt.Printf("Success! Type=%v Value=%v\n", v.Type, v.Value)
	}
}
