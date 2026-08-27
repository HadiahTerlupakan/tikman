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

// refreshVLANCache stores the OLT's VLAN table so the provisioning form can
// offer a list without walking SNMP itself. A failed or empty walk leaves the
// previous list in place: an unreachable OLT is no reason to empty a dropdown
// the operator still needs.
func (s *OLTService) refreshVLANCache(olt *models.OLT) {
	vlans, err := connectivity.WalkVLANs(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		log.Printf("[AutoDiscovery] VLAN walk failed for OLT %s: %v", olt.Name, err)
		return
	}
	if len(vlans) == 0 {
		log.Printf("[AutoDiscovery] VLAN walk returned nothing for OLT %s; keeping the cached list", olt.Name)
		return
	}

	encoded, err := json.Marshal(vlans)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot encode VLANs for OLT %s: %v", olt.Name, err)
		return
	}

	if err := s.db.Model(&models.OLT{}).Where("id = ?", olt.ID).Updates(map[string]interface{}{
		"vlans":            datatypes.JSON(encoded),
		"vlans_updated_at": time.Now(),
	}).Error; err != nil {
		log.Printf("[AutoDiscovery] Cannot store VLANs for OLT %s: %v", olt.Name, err)
		return
	}

	log.Printf("[AutoDiscovery] Cached %d VLANs for OLT %s", len(vlans), olt.Name)
}

// ListVLANs returns the VLANs cached by the last discovery poll, with the time
// they were read. An OLT that has not been polled yet returns an empty list
// rather than an error, so the form can fall back to a typed VLAN ID.
func (s *OLTService) ListVLANs(oltID uuid.UUID) ([]connectivity.OLTVLAN, *time.Time, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, nil, fmt.Errorf("OLT not found: %w", err)
	}

	vlans := make([]connectivity.OLTVLAN, 0)
	if len(olt.VLANs) > 0 {
		if err := json.Unmarshal(olt.VLANs, &vlans); err != nil {
			return nil, nil, fmt.Errorf("cached VLAN list is unreadable: %w", err)
		}
	}

	return vlans, olt.VLANsUpdatedAt, nil
}
