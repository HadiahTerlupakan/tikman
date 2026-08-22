package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// UnconfiguredONUWalker walks an OLT's autofind table. It exists so the service
// can be tested without a reachable OLT.
type UnconfiguredONUWalker func(ipAddress, community string, snmpPort int) ([]connectivity.UnconfiguredONU, error)

// UnconfiguredONUService lists ONUs detected by an OLT but not yet provisioned.
type UnconfiguredONUService struct {
	db   *gorm.DB
	walk UnconfiguredONUWalker
}

// NewUnconfiguredONUService creates a service backed by live SNMP walks.
func NewUnconfiguredONUService(db *gorm.DB) *UnconfiguredONUService {
	return NewUnconfiguredONUServiceWithWalker(db, connectivity.WalkUnconfiguredONUs)
}

// NewUnconfiguredONUServiceWithWalker creates a service with a custom walker so
// callers can exercise the scan without a reachable OLT.
func NewUnconfiguredONUServiceWithWalker(db *gorm.DB, walk UnconfiguredONUWalker) *UnconfiguredONUService {
	return &UnconfiguredONUService{db: db, walk: walk}
}

// ListByOLT returns the unconfigured ONUs the given OLT can currently see.
func (s *UnconfiguredONUService) ListByOLT(oltID uuid.UUID) ([]connectivity.UnconfiguredONU, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", oltID).Error; err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}

	if olt.SNMPCommunity == "" {
		return nil, fmt.Errorf("SNMP community not configured for this OLT")
	}

	onus, err := s.walk(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		return nil, fmt.Errorf("unconfigured ONU scan failed: %w", err)
	}

	return s.excludeRegistered(oltID, onus)
}

// excludeRegistered drops ONUs whose serial is already registered in TikMan.
// The OLT clears its autofind entry only after provisioning completes, so an
// ONU registered here but not yet pushed to the OLT would otherwise resurface
// as unconfigured and invite a duplicate registration.
func (s *UnconfiguredONUService) excludeRegistered(oltID uuid.UUID, onus []connectivity.UnconfiguredONU) ([]connectivity.UnconfiguredONU, error) {
	if len(onus) == 0 {
		return []connectivity.UnconfiguredONU{}, nil
	}

	serials := make([]string, len(onus))
	for i, onu := range onus {
		serials[i] = onu.SerialNumber
	}

	var registered []string
	if err := s.db.Model(&models.ONT{}).
		Where("olt_id = ? AND serial_number IN ?", oltID, serials).
		Pluck("serial_number", &registered).Error; err != nil {
		return nil, fmt.Errorf("registered serial lookup failed: %w", err)
	}

	if len(registered) == 0 {
		return onus, nil
	}

	known := make(map[string]bool, len(registered))
	for _, serial := range registered {
		known[serial] = true
	}

	filtered := make([]connectivity.UnconfiguredONU, 0, len(onus))
	for _, onu := range onus {
		if !known[onu.SerialNumber] {
			filtered = append(filtered, onu)
		}
	}

	return filtered, nil
}
