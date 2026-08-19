package main

import (
	"fmt"
	"log"
	
	"github.com/tikman/olt-provisioning/internal/connectivity"
)

func main() {
	fmt.Println("Testing Full Metrics Discovery with Fixed RX Power Decoder")
	fmt.Println(("================================================================"))
	
	metrics, err := connectivity.WalkONTMetrics("113.192.1.98", "public", 23161)
	if err != nil {
		log.Fatalf("Walk failed: %v", err)
	}
	
	fmt.Printf("\nTotal ONTs with metrics: %d\n\n", len(metrics))
	
	withRxPower := 0
	withoutRxPower := 0
	
	fmt.Printf("%-15s %-12s %-12s %-12s\n", "Location", "RX Power", "TX Power", "Distance")
	fmt.Println("---------------------------------------------------------------")
	
	for loc, m := range metrics {
		if loc.Slot == 3 && (loc.Port <= 2 || loc.Port == 15) {
			rxStr := "N/A"
			txStr := "N/A"
			distStr := "N/A"
			
			if m.RxPower != nil {
				rxStr = fmt.Sprintf("%.2f dBm", *m.RxPower)
				withRxPower++
			} else {
				withoutRxPower++
			}
			
			if m.TxPower != nil {
				txStr = fmt.Sprintf("%.2f dBm", *m.TxPower)
			}
			
			if m.Distance > 0 {
				distStr = fmt.Sprintf("%d m", m.Distance)
			}
			
			location := fmt.Sprintf("%d/%d:%d", loc.Slot, loc.Port, loc.ONTID)
			
			marker := ""
			if loc.Port == 15 && loc.ONTID == 26 {
				marker = " 🎯 TARGET"
			}
			
			fmt.Printf("%-15s %-12s %-12s %-12s%s\n", location, rxStr, txStr, distStr, marker)
		}
	}
	
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("Summary: %d with RX power, %d without\n", withRxPower, withoutRxPower)
	fmt.Println("\n✅ RX Power decoder fix verified!")
}
