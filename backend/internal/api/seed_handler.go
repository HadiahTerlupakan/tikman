package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

type SeedHandler struct {
	db            *gorm.DB
	oltService    *services.OLTService
	ontService    *services.ONTService
	eventService  *services.EventService
	seedService   *services.SeedService
	encryptionKey string
}

func NewSeedHandler(db *gorm.DB, encryptionKey string) *SeedHandler {
	return &SeedHandler{
		db:            db,
		oltService:    services.NewOLTService(db, encryptionKey),
		ontService:    services.NewONTService(db),
		eventService:  services.NewEventService(db),
		seedService:   services.NewSeedService(db),
		encryptionKey: encryptionKey,
	}
}

type SeedEventsResponse struct {
	TotalProcessed int      `json:"total_processed"`
	TotalEvents    int      `json:"total_events"`
	TotalErrors    int      `json:"total_errors"`
	Details        []string `json:"details"`
}

func (h *SeedHandler) SeedEventHistory(c *gin.Context) {
	log.Println("[Seed] Starting event history seeding from OLT historical data...")

	olts, err := h.oltService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list OLTs"})
		return
	}

	if len(olts) == 0 {
		c.JSON(http.StatusOK, SeedEventsResponse{
			Details: []string{"No OLTs found"},
		})
		return
	}

	var totalProcessed int
	var totalEvents int
	var totalErrors int
	var details []string

	for _, olt := range olts {
		log.Printf("[Seed] Processing OLT: %s (%s)", olt.Name, olt.IPAddress)

		onts, _, err := h.ontService.List(&olt.ID, nil, 1000, 0)
		if err != nil {
			details = append(details, "Failed to list ONTs for OLT "+olt.Name)
			continue
		}

		client := &gosnmp.GoSNMP{
			Target:    olt.IPAddress,
			Port:      uint16(olt.SNMPPort),
			Community: olt.SNMPCommunity,
			Version:   gosnmp.Version2c,
			Timeout:   time.Duration(5) * time.Second,
			Retries:   2,
		}

		if err := client.Connect(); err != nil {
			details = append(details, "Failed to connect to OLT "+olt.Name)
			continue
		}

		for _, ont := range onts {
			totalProcessed++

			existingEvents, _, err := h.eventService.GetEventsByONTID(ont.ID, 1, 0)
			if err != nil {
				totalErrors++
				continue
			}

			if len(existingEvents) > 0 {
				continue
			}

			var lastOnlineTime, lastOfflineTime time.Time
			var lastOfflineReason string
			var foundData bool

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
				continue
			}

			eventsCreated, eventErrors := h.seedService.SeedEventsForONT(ont.ID, lastOfflineReason, lastOnlineTime, lastOfflineTime)
			totalErrors += eventErrors

			if eventsCreated > 0 {
				totalEvents += eventsCreated
				details = append(details, ont.SerialNumber+" - Created "+string(rune(eventsCreated))+" events")
			}
		}

		_ = client.Conn.Close()
	}

	log.Printf("[Seed] Completed: %d ONTs processed, %d events created, %d errors", totalProcessed, totalEvents, totalErrors)

	c.JSON(http.StatusOK, SeedEventsResponse{
		TotalProcessed: totalProcessed,
		TotalEvents:    totalEvents,
		TotalErrors:    totalErrors,
		Details:        details,
	})
}
