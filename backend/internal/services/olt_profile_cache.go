package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
)

// profileCacheTTL is how long a cached profile list is trusted. Reading it
// costs a CLI login, which is far heavier than the SNMP walks around it, and
// profiles change when an engineer edits the OLT config, not by the minute.
const profileCacheTTL = 30 * time.Minute

// refreshTCONTProfileCache stores the OLT's T-CONT profile names for the
// provisioning form. As with the VLAN list, a failed read keeps whatever was
// cached before rather than emptying a dropdown the operator still needs.
func (s *OLTService) refreshTCONTProfileCache(olt *models.OLT) {
	if s.commanderFactory == nil {
		return
	}
	if olt.TCONTProfilesUpdatedAt != nil && time.Since(*olt.TCONTProfilesUpdatedAt) < profileCacheTTL {
		return
	}

	commander, err := createCommanderForOLT(s.commanderFactory, *olt)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot open a CLI session to OLT %s: %v", olt.Name, err)
		return
	}
	if closer, ok := commander.(interface{ Close() error }); ok {
		defer func() { _ = closer.Close() }()
	}

	profiles, err := connectivity.ReadZTETcontProfiles(context.Background(), commander)
	if err != nil {
		log.Printf("[AutoDiscovery] T-CONT profile read failed for OLT %s: %v", olt.Name, err)
		return
	}
	if len(profiles) == 0 {
		log.Printf("[AutoDiscovery] T-CONT profile read returned nothing for OLT %s; keeping the cached list", olt.Name)
		return
	}

	encoded, err := json.Marshal(profiles)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot encode T-CONT profiles for OLT %s: %v", olt.Name, err)
		return
	}

	if err := s.db.Model(&models.OLT{}).Where("id = ?", olt.ID).Updates(map[string]interface{}{
		"tcont_profiles":            datatypes.JSON(encoded),
		"tcont_profiles_updated_at": time.Now(),
	}).Error; err != nil {
		log.Printf("[AutoDiscovery] Cannot store T-CONT profiles for OLT %s: %v", olt.Name, err)
		return
	}

	log.Printf("[AutoDiscovery] Cached %d T-CONT profiles for OLT %s", len(profiles), olt.Name)
}

// ListTCONTProfiles returns the profile names cached by the last poll, with the
// time they were read. An OLT that has not been read yet returns an empty list
// rather than an error, so the form can fall back to a typed profile name.
func (s *OLTService) ListTCONTProfiles(oltID uuid.UUID) ([]string, *time.Time, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, nil, fmt.Errorf("OLT not found: %w", err)
	}

	profiles := make([]string, 0)
	if len(olt.TCONTProfiles) > 0 {
		if err := json.Unmarshal(olt.TCONTProfiles, &profiles); err != nil {
			return nil, nil, fmt.Errorf("cached profile list is unreadable: %w", err)
		}
	}

	return profiles, olt.TCONTProfilesUpdatedAt, nil
}
