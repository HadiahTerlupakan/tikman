package main

import (
	"log"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func main() {
	log.Println("=== ONT Event History Seeder ===")
	log.Println("Reading last online/offline times from OLT via SNMP...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	oltService := services.NewOLTService(db, cfg.EncryptionKey)
	ontService := services.NewONTService(db)
	eventService := services.NewEventService(db)

	olts, err := oltService.List()
	if err != nil {
		log.Fatalf("Failed to list OLTs: %v", err)
	}

	if len(olts) == 0 {
		log.Println("No OLTs found")
		return
	}

	var totalProcessed int
	var totalEvents int
	var totalErrors int

	for _, olt := range olts {
		log.Printf("\n--- Processing OLT: %s (%s) ---", olt.Name, olt.IPAddress)

		onts, _, err := ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			log.Printf("Failed to list ONTs for OLT %s: %v", olt.Name, err)
			continue
		}

		log.Printf("Found %d ONTs", len(onts))

		// Create SNMP client for this OLT
		client := &gosnmp.GoSNMP{
			Target:    olt.IPAddress,
			Port:      uint16(olt.SNMPPort),
			Community: olt.SNMPCommunity,
			Version:   gosnmp.Version2c,
			Timeout:   time.Duration(5) * time.Second,
			Retries:   3,
		}

		if err := client.Connect(); err != nil {
			log.Printf("Failed to connect to OLT %s: %v", olt.Name, err)
			continue
		}
		defer func() { _ = client.Conn.Close() }()

		for _, ont := range onts {
			totalProcessed++

			// Check if ONT already has events
			existingEvents, _, err := eventService.GetEventsByONTID(ont.ID, 1, 0)
			if err != nil {
				log.Printf("Failed to check existing events for ONT %s: %v", ont.SerialNumber, err)
				totalErrors++
				continue
			}

			if len(existingEvents) > 0 {
				log.Printf("  [SKIP] ONT %s already has %d events", ont.SerialNumber, len(existingEvents))
				continue
			}

			// We don't have slot info in database, use 0 (will be discovered from walk)
			// For seeding, we'll try common slots 1-8
			var lastOnlineTime, lastOfflineTime time.Time
			var lastOfflineReason string
			var foundData bool

			// Try slots 1-8 to find the ONT
			for slot := 1; slot <= 8; slot++ {
				onlineTime, err1 := connectivity.GetLastOnlineTime(client, slot, ont.PortID, ont.ONTID)
				offlineTime, err2 := connectivity.GetLastOfflineTime(client, slot, ont.PortID, ont.ONTID)
				offlineReason, _ := connectivity.GetLastOfflineReason(client, slot, ont.PortID, ont.ONTID)

				if err1 == nil && !onlineTime.IsZero() {
					lastOnlineTime = onlineTime
					lastOfflineTime = offlineTime
					lastOfflineReason = offlineReason
					foundData = true
					break
				}

				if err2 == nil && !offlineTime.IsZero() {
					lastOnlineTime = onlineTime
					lastOfflineTime = offlineTime
					lastOfflineReason = offlineReason
					foundData = true
					break
				}
			}

			if !foundData {
				log.Printf("  [SKIP] ONT %s - No historical data found in OLT", ont.SerialNumber)
				continue
			}

			// If we got valid historical data, create initial events
			eventsCreated := 0

			// Create offline event if we have offline time
			if !lastOfflineTime.IsZero() {
				event := &models.ONTEvent{
					ONTID:     ont.ID,
					EventType: models.EventTypeOffline,
					EventTime: lastOfflineTime,
					Reason:    lastOfflineReason,
				}

				if err := db.Create(event).Error; err != nil {
					log.Printf("  [ERROR] Failed to create offline event for ONT %s: %v", ont.SerialNumber, err)
					totalErrors++
				} else {
					eventsCreated++
				}
			}

			// Create online event if we have online time
			if !lastOnlineTime.IsZero() {
				// Calculate duration if we have both offline and online times
				var durationSeconds *int64
				if !lastOfflineTime.IsZero() && lastOnlineTime.After(lastOfflineTime) {
					duration := int64(lastOnlineTime.Sub(lastOfflineTime).Seconds())
					durationSeconds = &duration
				}

				event := &models.ONTEvent{
					ONTID:           ont.ID,
					EventType:       models.EventTypeOnline,
					EventTime:       lastOnlineTime,
					Reason:          "System startup",
					DurationSeconds: durationSeconds,
				}

				if err := db.Create(event).Error; err != nil {
					log.Printf("  [ERROR] Failed to create online event for ONT %s: %v", ont.SerialNumber, err)
					totalErrors++
				} else {
					eventsCreated++
				}
			}

			if eventsCreated > 0 {
				totalEvents += eventsCreated
				log.Printf("  [OK] ONT %s - Created %d historical events (Last Online: %v, Last Offline: %v - %s)",
					ont.SerialNumber,
					eventsCreated,
					formatTimeValue(lastOnlineTime),
					formatTimeValue(lastOfflineTime),
					lastOfflineReason,
				)
			}
		}
	}

	log.Printf("\n=== Summary ===")
	log.Printf("Total ONTs processed: %d", totalProcessed)
	log.Printf("Total events created: %d", totalEvents)
	log.Printf("Total errors: %d", totalErrors)
	log.Println("\nEvent history seeding completed!")
}

func formatTimeValue(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04:05")
}
