package main

import (
	"fmt"
	"time"
	"github.com/gosnmp/gosnmp"
)

func main() {
	ifindex := 268632320
	ontID := 18
	
	client := &gosnmp.GoSNMP{
		Target:    "192.0.2.10",
		Port:      161,
		Community: "<community-anda>",
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
	
	// Try different serial number OIDs
	oids := map[string]string{
		"zxAnPonOnuSerialNo":     ".1.3.6.1.4.1.3902.1012.3.28.1.1.1",  // Current one
		"zxAnPonOnuEquipmentId":  ".1.3.6.1.4.1.3902.1012.3.28.1.1.3",  // Equipment ID
		"zxAnPonOnuName":         ".1.3.6.1.4.1.3902.1012.3.28.1.1.5",  // ONT Name
		"zxAnPonOnuType":         ".1.3.6.1.4.1.3902.1012.3.28.1.1.8",  // ONT Type
	}
	
	for name, baseOID := range oids {
		oid := fmt.Sprintf("%s.%d.%d", baseOID, ifindex, ontID)
		fmt.Printf("\n%s: %s\n", name, oid)
		
		result, err := client.Get([]string{oid})
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		
		if len(result.Variables) > 0 {
			v := result.Variables[0]
			fmt.Printf("  Type: %v\n", v.Type)
			fmt.Printf("  Value: %v\n", v.Value)
		}
	}
}
