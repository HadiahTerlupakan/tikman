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
	updates := make(map[string]interface{}, 3)

	info, err := connectivity.ReadSystemInfo(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] System info read failed for OLT %s: %v", olt.Name, err)
	default:
		if encoded, err := json.Marshal(info); err == nil {
			updates["system_info"] = datatypes.JSON(encoded)
		} else {
			log.Printf("[AutoDiscovery] Cannot encode system info for OLT %s: %v", olt.Name, err)
		}
	}

	health, err := connectivity.WalkCardHealth(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] Card health walk failed for OLT %s: %v", olt.Name, err)
	case len(health) == 0:
		log.Printf("[AutoDiscovery] Card health walk returned nothing for OLT %s; keeping the cached list", olt.Name)
	default:
		if encoded, err := json.Marshal(health); err == nil {
			updates["card_health"] = datatypes.JSON(encoded)
		} else {
			log.Printf("[AutoDiscovery] Cannot encode card health for OLT %s: %v", olt.Name, err)
		}
	}

	speeds, err := connectivity.WalkTcontProfiles(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] T-CONT profile walk failed for OLT %s: %v", olt.Name, err)
	case len(speeds) == 0:
		log.Printf("[AutoDiscovery] T-CONT profile walk returned nothing for OLT %s; keeping the cached list", olt.Name)
	default:
		names := make([]string, 0, len(speeds))
		for _, profile := range speeds {
			names = append(names, profile.Name)
		}
		addProfileUpdate(updates, "tcont_profiles", names, olt.Name)
		if encoded, err := json.Marshal(speeds); err == nil {
			updates["tcont_profile_details"] = datatypes.JSON(encoded)
		}
	}

	ports, err := connectivity.WalkPorts(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] Port walk failed for OLT %s: %v", olt.Name, err)
	case len(ports) == 0:
		log.Printf("[AutoDiscovery] Port walk returned nothing for OLT %s; keeping the cached list", olt.Name)
	default:
		if encoded, err := json.Marshal(ports); err == nil {
			updates["ports"] = datatypes.JSON(encoded)
		} else {
			log.Printf("[AutoDiscovery] Cannot encode ports for OLT %s: %v", olt.Name, err)
		}
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

	if len(olt.SystemInfo) > 0 {
		if err := json.Unmarshal(olt.SystemInfo, &snapshot.System); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached system info is unreadable: %w", err)
		}
	}
	if snapshot.System.Entities == nil {
		snapshot.System.Entities = make([]connectivity.ChassisEntity, 0)
	}
	if len(olt.Ports) > 0 {
		if err := json.Unmarshal(olt.Ports, &snapshot.Ports); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached port list is unreadable: %w", err)
		}
	}
	if len(olt.Cards) > 0 {
		if err := json.Unmarshal(olt.Cards, &snapshot.Cards); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached card list is unreadable: %w", err)
		}
	}
	if len(olt.CardHealth) > 0 {
		if err := json.Unmarshal(olt.CardHealth, &snapshot.CardHealth); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached card health is unreadable: %w", err)
		}
	}
	if len(olt.ONUTypeDetails) > 0 {
		if err := json.Unmarshal(olt.ONUTypeDetails, &snapshot.ONUTypes); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached ONU type details are unreadable: %w", err)
		}
	}
	if len(olt.TCONTProfileDetails) > 0 {
		if err := json.Unmarshal(olt.TCONTProfileDetails, &snapshot.Speeds); err != nil {
			return OLTSystemSnapshot{}, fmt.Errorf("cached speed profiles are unreadable: %w", err)
		}
	}

	return snapshot, nil
}
