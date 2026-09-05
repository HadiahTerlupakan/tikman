package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
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

// seedTally accumulates one seeding run across every OLT it visits.
type seedTally struct {
	processed int
	events    int
	errors    int
	details   []string
}

// maxSeedSlots is how many cards a chassis is searched for an ONT's history.
// The position tables are per-card, and the ONT row does not record which card
// it sits on, so the seeder tries each in turn.
const maxSeedSlots = 8

// seedONTPageSize and seedSNMPTimeout bound one chassis's pass.
const (
	seedONTPageSize = 1000
	seedSNMPTimeout = 5 * time.Second
)

func (h *SeedHandler) SeedEventHistory(c *gin.Context) {
	log.Println("[Seed] Starting event history seeding from OLT historical data...")

	olts, err := h.oltService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list OLTs"})
		return
	}
	if len(olts) == 0 {
		c.JSON(http.StatusOK, SeedEventsResponse{Details: []string{"No OLTs found"}})
		return
	}

	tally := &seedTally{}
	for _, olt := range olts {
		h.seedOLT(olt, tally)
	}

	log.Printf("[Seed] Completed: %d ONTs processed, %d events created, %d errors",
		tally.processed, tally.events, tally.errors)

	c.JSON(http.StatusOK, SeedEventsResponse{
		TotalProcessed: tally.processed,
		TotalEvents:    tally.events,
		TotalErrors:    tally.errors,
		Details:        tally.details,
	})
}

// seedOLT walks one chassis, opening a single SNMP session for all its ONTs.
func (h *SeedHandler) seedOLT(olt models.OLT, tally *seedTally) {
	log.Printf("[Seed] Processing OLT: %s (%s)", olt.Name, olt.IPAddress)

	onts, _, err := h.ontService.List(&olt.ID, nil, seedONTPageSize, 0)
	if err != nil {
		tally.details = append(tally.details, "Failed to list ONTs for OLT "+olt.Name)
		return
	}

	client := &gosnmp.GoSNMP{
		Target:    olt.IPAddress,
		Port:      uint16(olt.SNMPPort),
		Community: olt.SNMPCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   seedSNMPTimeout,
		Retries:   2,
	}
	if err := client.Connect(); err != nil {
		tally.details = append(tally.details, "Failed to connect to OLT "+olt.Name)
		return
	}
	defer func() { _ = client.Conn.Close() }()

	for _, ont := range onts {
		h.seedONT(client, ont, tally)
	}
}

// seedONT writes the history the chassis still remembers for one ONT, and
// leaves an ONT that already has events alone: seeding is a backfill, not a
// re-import.
func (h *SeedHandler) seedONT(client *gosnmp.GoSNMP, ont models.ONT, tally *seedTally) {
	tally.processed++

	existing, _, err := h.eventService.GetEventsByONTID(ont.ID, 1, 0)
	if err != nil {
		tally.errors++
		return
	}
	if len(existing) > 0 {
		return
	}

	history, found := ontSNMPHistory(client, ont)
	if !found {
		return
	}

	created, failed := h.seedService.SeedEventsForONT(ont.ID, history.offlineReason, history.online, history.offline)
	tally.errors += failed
	if created > 0 {
		tally.events += created
		tally.details = append(tally.details, fmt.Sprintf("%s - Created %d events", ont.SerialNumber, created))
	}
}

// ontHistory is what one ONT's position tables still hold.
type ontHistory struct {
	online        time.Time
	offline       time.Time
	offlineReason string
}

// ontSNMPHistory searches the cards for the one carrying this ONT, and stops
// at the first that answers with a timestamp.
func ontSNMPHistory(client *gosnmp.GoSNMP, ont models.ONT) (ontHistory, bool) {
	for slot := 1; slot <= maxSeedSlots; slot++ {
		online, onlineErr := connectivity.GetLastOnlineTime(client, slot, ont.PortID, ont.ONTID)
		offline, offlineErr := connectivity.GetLastOfflineTime(client, slot, ont.PortID, ont.ONTID)
		reason, _ := connectivity.GetLastOfflineReason(client, slot, ont.PortID, ont.ONTID)

		onlineFound := onlineErr == nil && !online.IsZero()
		offlineFound := offlineErr == nil && !offline.IsZero()
		if onlineFound || offlineFound {
			return ontHistory{online: online, offline: offline, offlineReason: reason}, true
		}
	}
	return ontHistory{}, false
}
