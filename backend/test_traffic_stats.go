package main

import (
	"log"
	"os"

	"github.com/tikman/olt-provisioning/internal/connectivity"
)

func main() {
	oltIP := os.Getenv("OLT_IP")
	oltCommunity := os.Getenv("OLT_COMMUNITY")
	oltPort := 161

	if oltIP == "" {
		oltIP = "172.20.1.251"
	}
	if oltCommunity == "" {
		oltCommunity = "public"
	}

	log.Printf("Testing traffic statistics collection from ZTE OLT: %s", oltIP)
	log.Printf("Community: %s, Port: %d", oltCommunity, oltPort)
	log.Println()

	log.Println("Step 1: Discovering ONTs...")
	topology, err := connectivity.DiscoverOLTTopology(oltIP, oltCommunity, oltPort)
	if err != nil {
		log.Fatalf("Failed to discover topology: %v", err)
	}

	log.Printf("Found %d slots\n", len(topology))
	log.Println()

	totalONTs := 0
	ontsWithTraffic := 0

	for _, slot := range topology {
		for _, port := range slot.Ports {
			for _, ont := range port.ONTs {
				totalONTs++
				
				log.Printf("ONT: Slot=%d Port=%d ONTID=%d Serial=%s",
					slot.Slot, port.PortID, ont.ONTID, ont.SerialNumber)

				if ont.RxBytes > 0 || ont.TxBytes > 0 {
					ontsWithTraffic++
					log.Printf("  ✅ Traffic Statistics:")
					log.Printf("     RX: %d bytes, %d packets, %d errors",
						ont.RxBytes, ont.RxPackets, ont.RxErrors)
					log.Printf("     TX: %d bytes, %d packets, %d errors",
						ont.TxBytes, ont.TxPackets, ont.TxErrors)
				} else {
					log.Printf("  ⚠️  No traffic data (ONT might be offline or data not available)")
				}

				if ont.RxPower != nil {
					log.Printf("  Optical: RX=%.2f dBm TX=%.2f dBm Distance=%dm",
						*ont.RxPower,
						func() float64 {
							if ont.TxPower != nil {
								return *ont.TxPower
							}
							return 0.0
						}(),
						ont.Distance)
				}
				log.Println()
			}
		}
	}

	log.Println("=====================================")
	log.Printf("SUMMARY:")
	log.Printf("  Total ONTs discovered: %d", totalONTs)
	log.Printf("  ONTs with traffic data: %d", ontsWithTraffic)
	log.Printf("  Coverage: %.1f%%", float64(ontsWithTraffic)/float64(totalONTs)*100)
	log.Println("=====================================")

	if ontsWithTraffic == 0 {
		log.Println()
		log.Println("⚠️  WARNING: No ONTs have traffic statistics!")
		log.Println("Possible causes:")
		log.Println("  1. All ONTs are offline")
		log.Println("  2. OLT doesn't support traffic statistics OIDs")
		log.Println("  3. SNMP walk timeout or community string incorrect")
		os.Exit(1)
	}

	if ont := findONT(topology, "RTEGC609833D"); ont != nil {
		log.Println()
		log.Println("=====================================")
		log.Printf("TARGET ONT FOUND: RTEGC609833D")
		log.Println("=====================================")
		log.Printf("Serial: %s", ont.SerialNumber)
		log.Printf("Status: %s", ont.Status)
		log.Printf("RX Bytes: %d", ont.RxBytes)
		log.Printf("TX Bytes: %d", ont.TxBytes)
		log.Printf("RX Packets: %d", ont.RxPackets)
		log.Printf("TX Packets: %d", ont.TxPackets)
		log.Printf("RX Errors: %d", ont.RxErrors)
		log.Printf("TX Errors: %d", ont.TxErrors)

		if ont.RxPackets > 0 || ont.TxPackets > 0 {
			log.Println()
			log.Println("✅ SUCCESS: Traffic statistics are being collected!")
		} else {
			log.Println()
			log.Println("⚠️  WARNING: Target ONT has no traffic data")
		}
	} else {
		log.Println()
		log.Println("⚠️  Target ONT RTEGC609833D not found in discovery")
	}
}

func findONT(topology []connectivity.GPONSlot, serial string) *connectivity.DiscoveredONT {
	for _, slot := range topology {
		for _, port := range slot.Ports {
			for _, ont := range port.ONTs {
				if ont.SerialNumber == serial {
					return &ont
				}
			}
		}
	}
	return nil
}
