package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// UnconfiguredONUWalker walks an OLT's autofind table with the driver for that
// OLT's model. It exists so the service can be tested without a reachable OLT.
type UnconfiguredONUWalker func(driver connectivity.Driver, ipAddress, community string, snmpPort int) ([]connectivity.UnconfiguredONU, error)

// UnconfiguredONUService lists ONUs detected by an OLT but not yet provisioned.
type UnconfiguredONUService struct {
	db   *gorm.DB
	walk UnconfiguredONUWalker
}

// NewUnconfiguredONUService creates a service backed by live SNMP walks.
func NewUnconfiguredONUService(db *gorm.DB) *UnconfiguredONUService {
	return NewUnconfiguredONUServiceWithWalker(db, liveUnconfiguredONUWalk)
}

// liveUnconfiguredONUWalk scans a reachable OLT. The driver applies its own
// deadline to the scan, so the context passed here carries none.
func liveUnconfiguredONUWalk(driver connectivity.Driver, ipAddress, community string, snmpPort int) ([]connectivity.UnconfiguredONU, error) {
	return driver.WalkUnconfigured(context.Background(), ipAddress, community, snmpPort)
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

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	onus, err := s.walk(driver, olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if err != nil {
		return nil, fmt.Errorf("unconfigured ONU scan failed: %w", err)
	}

	// An empty walk is an empty list, not a null one: the page renders it.
	if onus == nil {
		onus = []connectivity.UnconfiguredONU{}
	}

	// The autofind table is the authority here: an OLT lists an ONU there only
	// when it holds no configuration for it. Filtering that against TikMan's own
	// ONT rows used to hide exactly the case an operator needs to see — an ONU
	// deleted from the OLT while a stale row survived here — so the serial
	// became invisible and could never be provisioned again.
	return onus, nil
}
