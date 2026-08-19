//go:build ignore

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
)

func main() {
	target := "113.192.1.98"
	port := 23161
	community := "public"
	timeout := time.Second * 5

	log.Printf("Testing SNMP connection to %s:%d with community '%s' (timeout: %v)", target, port, community, timeout)

	client := &gosnmp.GoSNMP{
		Target:    target,
		Port:      uint16(port),
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   timeout,
		Retries:   1,
	}

	err := client.Connect()
	if err != nil {
		log.Fatalf("UDP connection failed: %v", err)
	}
	defer client.Conn.Close()

	// Test basic OID
	oids := []string{"1.3.6.1.2.1.1.1.0"} // sysDescr
	result, err := client.Get(oids)
	if err != nil {
		log.Fatalf("GET request failed: %v", err)
	}

	fmt.Println("\n=== Basic SNMP Test ===")
	fmt.Printf("System Description:\n%v\n", result.Variables[0].Value)

	// Try to discover ONTs
	log.Printf("\n=== Discovering ONTs ===")
	oidSerial := ".1.3.6.1.4.1.3902.1012.3.28.1.1.1"

	var discoveredOnus []struct {
		portID       int
		ontID        int
		serialNumber string
		rxPower      float64
		txPower      float64
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

		// Get metrics for this ONT
		oidRx := fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.5.%d.%d", ifindex, ontID)
		oidTx := fmt.Sprintf(".1.3.6.1.4.1.3902.1012.3.28.1.1.6.%d.%d", ifindex, ontID)

		resultRx, errRx := client.Get([]string{oidRx})
		resultTx, errTx := client.Get([]string{oidTx})

		rxPower := float64(0)
		txPower := float64(0)

		if errRx == nil && len(resultRx.Variables) > 0 {
			rxPower = float64(gosnmp.ToBigInt(resultRx.Variables[0].Value).Int64()) * 0.01
		}

		if errTx == nil && len(resultTx.Variables) > 0 {
			txPower = float64(gosnmp.ToBigInt(resultTx.Variables[0].Value).Int64()) * 0.01
		}

		discoveredOnus = append(discoveredOnus, struct {
			portID       int
			ontID        int
			serialNumber string
			rxPower      float64
			txPower      float64
		}{portID, ontID, serialNumber, rxPower, txPower})

		fmt.Printf("\nONT Found:\n")
		fmt.Printf("  Port ID: %d\n", portID)
		fmt.Printf("  ONT ID: %d\n", ontID)
		fmt.Printf("  Serial: %s\n", serialNumber)
		fmt.Printf("  Rx Power: %.2f dBm\n", rxPower)
		fmt.Printf("  Tx Power: %.2f dBm\n", txPower)

		return nil
	})

	if err != nil {
		log.Printf("SNMP walk completed with warnings: %v", err)
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total ONTs discovered: %d\n", len(discoveredOnus))

	if len(discoveredOnus) == 0 {
		fmt.Println("\n⚠️ WARNING: No ONTs discovered!")
		fmt.Println("This could mean:")
		fmt.Println("1. Incorrect rack/shelf/slot configuration")
		fmt.Println("2. SNMP community string wrong")
		fmt.Println("3. ONT not online or responding")
		fmt.Println("4. Wrong SNMP port")
	}
}
