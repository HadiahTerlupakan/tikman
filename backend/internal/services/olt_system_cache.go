package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

// refreshSystemCache stores the chassis summary and port inventory so the OLT
// configuration page can render without walking the device. Both reads are
// plain SNMP; a failure keeps whatever was cached before, because an
// unreachable OLT is no reason to blank a page the operator is looking at.
func (s *OLTService) refreshSystemCache(olt *models.OLT) {
	updates := make(map[string]interface{}, 5)

	info, err := connectivity.ReadSystemInfo(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		log.Printf("[AutoDiscovery] System info read failed for OLT %s: %v", olt.Name, err)
	} else {
		cacheJSON(updates, "system_info", info, olt.Name)
	}

	health, err := connectivity.WalkCardHealth(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if keptWalk(err, len(health), "Card health walk", olt.Name) {
		cacheJSON(updates, "card_health", health, olt.Name)
	}

	speeds, err := connectivity.WalkTcontProfiles(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if keptWalk(err, len(speeds), "T-CONT profile walk", olt.Name) {
		names := make([]string, 0, len(speeds))
		for _, profile := range speeds {
			names = append(names, profile.Name)
		}
		addProfileUpdate(updates, "tcont_profiles", names, olt.Name)
		cacheJSON(updates, "tcont_profile_details", speeds, olt.Name)
	}

	ports, err := connectivity.WalkPorts(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if keptWalk(err, len(ports), "Port walk", olt.Name) {
		cacheJSON(updates, "ports", ports, olt.Name)
	}

	if len(updates) == 0 {
		return
	}
	updates["system_updated_at"] = time.Now()

	if err := s.db.Model(&models.OLT{}).Where("id = ?", olt.ID).Updates(updates).Error; err != nil {
		log.Printf("[AutoDiscovery] Cannot store system info for OLT %s: %v", olt.Name, err)
		return
	}

	log.Printf("[AutoDiscovery] Cached %d chassis entities and %d ports for OLT %s", len(info.Entities), len(ports), olt.Name)
}

// keptWalk reports whether a walk returned something worth caching. An empty
// answer leaves the cached list alone: the OLT dropping a table under load
// must not read as the cards or ports having gone away.
func keptWalk(err error, found int, what, oltName string) bool {
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] %s failed for OLT %s: %v", what, oltName, err)
		return false
	case found == 0:
		log.Printf("[AutoDiscovery] %s returned nothing for OLT %s; keeping the cached list", what, oltName)
		return false
	default:
		return true
	}
}

func cacheJSON(updates map[string]interface{}, column string, value interface{}, oltName string) {
	encoded, err := json.Marshal(value)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot encode %s for OLT %s: %v", column, oltName, err)
		return
	}
	updates[column] = datatypes.JSON(encoded)
}

// RefreshSystem re-reads the chassis, ports and card health from the OLT now
// and returns the fresh snapshot. Every read is SNMP, so this costs the device
// three short walks and never opens a CLI session.
func (s *OLTService) RefreshSystem(oltID uuid.UUID) (OLTSystemSnapshot, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return OLTSystemSnapshot{}, fmt.Errorf("OLT not found: %w", err)
	}

	s.refreshSystemCache(&olt)
	s.refreshVLANCache(&olt)

	return s.GetSystemSnapshot(oltID)
}

// OLTSystemSnapshot is everything the configuration page reads in one request:
// the chassis summary, the port inventory, and the fitted cards the CLI poll
// recorded separately.
type OLTSystemSnapshot struct {
	System     connectivity.OLTSystemInfo     `json:"system"`
	Ports      []connectivity.OLTPort         `json:"ports"`
	Cards      []connectivity.ZTECard         `json:"cards"`
	CardHealth []connectivity.CardHealth      `json:"card_health"`
	ONUTypes   []connectivity.ZTEONUType      `json:"onu_types"`
	Speeds     []connectivity.ZTETcontProfile `json:"speed_profiles"`
	UpdatedAt  *time.Time                     `json:"updated_at,omitempty"`
}

// GetSystemSnapshot returns the cached chassis and port inventory. An OLT that
// has not been polled yet returns empty lists rather than an error, so the page
// can say the poll has not run instead of showing a failure.
func (s *OLTService) GetSystemSnapshot(oltID uuid.UUID) (OLTSystemSnapshot, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return OLTSystemSnapshot{}, fmt.Errorf("OLT not found: %w", err)
	}

	snapshot := OLTSystemSnapshot{
		Ports:      make([]connectivity.OLTPort, 0),
		Cards:      make([]connectivity.ZTECard, 0),
		CardHealth: make([]connectivity.CardHealth, 0),
		ONUTypes:   make([]connectivity.ZTEONUType, 0),
		Speeds:     make([]connectivity.ZTETcontProfile, 0),
		UpdatedAt:  olt.SystemUpdatedAt,
	}

	cached := []struct {
		what   string
		stored datatypes.JSON
		into   interface{}
	}{
		{"system info", olt.SystemInfo, &snapshot.System},
		{"port list", olt.Ports, &snapshot.Ports},
		{"card list", olt.Cards, &snapshot.Cards},
		{"card health", olt.CardHealth, &snapshot.CardHealth},
		{"ONU type details", olt.ONUTypeDetails, &snapshot.ONUTypes},
		{"speed profiles", olt.TCONTProfileDetails, &snapshot.Speeds},
	}
	for _, entry := range cached {
		if len(entry.stored) == 0 {
			continue
		}
		if err := json.Unmarshal(entry.stored, entry.into); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached %s is unreadable: %w", entry.what, err)
		}
	}

	// The chassis list is drawn even when nothing was cached, so it must be an
	// array rather than a null.
	if snapshot.System.Entities == nil {
		snapshot.System.Entities = make([]connectivity.ChassisEntity, 0)
	}

	return snapshot, nil
}
