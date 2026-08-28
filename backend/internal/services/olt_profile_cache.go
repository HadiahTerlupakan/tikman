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
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/datatypes"
)

// profileCacheTTL is how long a cached profile list is trusted. Reading it
// costs a CLI login, which is far heavier than the SNMP walks around it, and
// profiles change when an engineer edits the OLT config, not by the minute.
const profileCacheTTL = 30 * time.Minute

// refreshProfileCache stores the profile names the provisioning form offers.
// Both lists come from one CLI session, and a failed read keeps whatever was
// cached before rather than emptying a dropdown the operator still needs.
func (s *OLTService) refreshProfileCache(olt *models.OLT) {
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

	ctx := context.Background()
	updates := make(map[string]interface{}, 3)

	tcont, err := connectivity.ReadZTETcontProfiles(ctx, commander)
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] T-CONT profile read failed for OLT %s: %v", olt.Name, err)
	case len(tcont) == 0:
		log.Printf("[AutoDiscovery] T-CONT profile read returned nothing for OLT %s", olt.Name)
	default:
		addProfileUpdate(updates, "tcont_profiles", tcont, olt.Name)
	}

	snapshot, err := connectivity.ReadZTEConfigSnapshot(ctx, commander)
	vlan := snapshot.VLANProfiles
	switch {
	case err != nil:
		log.Printf("[AutoDiscovery] Running config read failed for OLT %s: %v", olt.Name, err)
	case len(vlan) == 0:
		log.Printf("[AutoDiscovery] No VLAN profiles in use on OLT %s", olt.Name)
	default:
		addProfileUpdate(updates, "vlan_profiles", vlan, olt.Name)
	}
	if err == nil {
		if len(snapshot.ONUTypes) > 0 {
			addProfileUpdate(updates, "onu_types", snapshot.ONUTypes, olt.Name)
		}
		if len(snapshot.Cards) > 0 {
			if encoded, err := json.Marshal(snapshot.Cards); err == nil {
				updates["cards"] = datatypes.JSON(encoded)
			}
		}
		s.storeONUServices(olt, snapshot.ONUServices)
	}

	if len(updates) == 0 {
		return
	}
	updates["tcont_profiles_updated_at"] = time.Now()

	if err := s.db.Model(&models.OLT{}).Where("id = ?", olt.ID).Updates(updates).Error; err != nil {
		log.Printf("[AutoDiscovery] Cannot store profiles for OLT %s: %v", olt.Name, err)
		return
	}

	log.Printf("[AutoDiscovery] Cached %d T-CONT profiles, %d VLAN profiles and %d ONU types for OLT %s", len(tcont), len(vlan), len(snapshot.ONUTypes), olt.Name)
}

// storeONUServices records each ONU's current service against its ONT row, so
// the configure form can open pre-filled. ONUs the OLT reports but TikMan has
// not registered are skipped: there is no row to attach them to.
func (s *OLTService) storeONUServices(olt *models.OLT, services map[connectivity.ONTLocation]connectivity.ZTEONUService) {
	stored, now := 0, time.Now()
	for location, service := range services {
		encoded, err := json.Marshal(service)
		if err != nil {
			continue
		}
		updates := map[string]interface{}{"service_config": datatypes.JSON(encoded), "service_config_at": now}
		// Encrypted with the same key as the OLT's own credentials. A failure to
		// encrypt drops the password rather than storing it readable.
		if service.PPPoEPassword != "" {
			if sealed, err := utils.Encrypt(service.PPPoEPassword, string(s.encryptionKey)); err == nil {
				updates["pppoe_password"] = sealed
			} else {
				log.Printf("[AutoDiscovery] Cannot encrypt PPPoE password for OLT %s: %v", olt.Name, err)
			}
		}
		result := s.db.Model(&models.ONT{}).
			Where("olt_id = ? AND port_id = ? AND ont_id = ?", olt.ID, location.Port, location.ONTID).
			Updates(updates)
		if result.Error != nil {
			log.Printf("[AutoDiscovery] Cannot store service config for OLT %s: %v", olt.Name, result.Error)
			return
		}
		stored += int(result.RowsAffected)
	}
	log.Printf("[AutoDiscovery] Stored service config for %d ONTs on OLT %s", stored, olt.Name)
}

func addProfileUpdate(updates map[string]interface{}, column string, names []string, oltName string) {
	encoded, err := json.Marshal(names)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot encode %s for OLT %s: %v", column, oltName, err)
		return
	}
	updates[column] = datatypes.JSON(encoded)
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

// ListONUTypes returns the ONU types the OLT accepts, cached by the last poll.
func (s *OLTService) ListONUTypes(oltID uuid.UUID) ([]string, *time.Time, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, nil, fmt.Errorf("OLT not found: %w", err)
	}

	types := make([]string, 0)
	if len(olt.ONUTypes) > 0 {
		if err := json.Unmarshal(olt.ONUTypes, &types); err != nil {
			return nil, nil, fmt.Errorf("cached ONU type list is unreadable: %w", err)
		}
	}

	return types, olt.TCONTProfilesUpdatedAt, nil
}

// ListCards returns the line cards fitted to the OLT, as the last poll read
// them from its running config.
func (s *OLTService) ListCards(oltID uuid.UUID) ([]connectivity.ZTECard, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}

	cards := make([]connectivity.ZTECard, 0)
	if len(olt.Cards) > 0 {
		if err := json.Unmarshal(olt.Cards, &cards); err != nil {
			return nil, fmt.Errorf("cached card list is unreadable: %w", err)
		}
	}

	return cards, nil
}

// ListVLANProfiles returns the VLAN profile names cached by the last poll.
func (s *OLTService) ListVLANProfiles(oltID uuid.UUID) ([]string, *time.Time, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, nil, fmt.Errorf("OLT not found: %w", err)
	}

	profiles := make([]string, 0)
	if len(olt.VLANProfiles) > 0 {
		if err := json.Unmarshal(olt.VLANProfiles, &profiles); err != nil {
			return nil, nil, fmt.Errorf("cached profile list is unreadable: %w", err)
		}
	}

	return profiles, olt.TCONTProfilesUpdatedAt, nil
}
