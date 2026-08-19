package services

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// OLTService handles OLT operations
type OLTService struct {
	db            *gorm.DB
	encryptionKey []byte
}

// NewOLTService creates a new OLT service
func NewOLTService(db *gorm.DB, encryptionKey string) *OLTService {
	return &OLTService{
		db:            db,
		encryptionKey: []byte(encryptionKey),
	}
}

// GetDB returns the database instance
func (s *OLTService) GetDB() *gorm.DB {
	return s.db
}

// GetEncryptionKey returns the encryption key
func (s *OLTService) GetEncryptionKey() []byte {
	return s.encryptionKey
}

// Create creates a new OLT with status validation
func (s *OLTService) Create(
	name,
	ipAddress,
	snmpCommunity,
	username,
	password string,
	rack,
	shelf,
	slot int,
	sshPort,
	telnetPort,
	snmpPort int,
	preferredProtocol models.OLTProtocol,
) (*models.OLT, error) {
	// Validate SNMP port
	if snmpPort < 1 || snmpPort > 65535 {
		return nil, fmt.Errorf("invalid SNMP port: %d", snmpPort)
	}

	// Validate SSH port
	if sshPort < 1 || sshPort > 65535 {
		return nil, fmt.Errorf("invalid SSH port: %d", sshPort)
	}

	// Validate Telnet port
	if telnetPort < 1 || telnetPort > 65535 {
		return nil, fmt.Errorf("invalid Telnet port: %d", telnetPort)
	}

	// Encrypt password before storing using encryptionKey as string
	encryptedPassword, err := utils.Encrypt(password, strings.TrimSpace(string(s.encryptionKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	// Validate SNMP connectivity if community provided
	if snmpCommunity != "" {
		if err := connectivity.SNMPTest(ipAddress, snmpPort, snmpCommunity, 0); err != nil {
			return nil, fmt.Errorf("SNMP connection test failed: %w", err)
		}
	}

	// Create OLT without Rack/Shelf/Slot - discovery happens at ONT level
	olt := &models.OLT{
		SiteID:            uuid.New(),
		Name:              name,
		IPAddress:         ipAddress,
		SSHPort:           sshPort,
		TelnetPort:        telnetPort,
		SNMPPort:          snmpPort,
		SNMPCommunity:     snmpCommunity,
		PreferredProtocol: preferredProtocol,
		Username:          username,
		Password:          encryptedPassword,
		Status:            models.OLTStatusOnline, // Changed from Offline to Online
	}

	if err := s.db.Create(olt).Error; err != nil {
		return nil, fmt.Errorf("failed to create OLT: %w", err)
	}

	return olt, nil
}

func (s *OLTService) GetByID(id uuid.UUID) (*models.OLT, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}
	return &olt, nil
}

func (s *OLTService) List() ([]models.OLT, error) {
	var olts []models.OLT
	if err := s.db.Find(&olts).Error; err != nil {
		return nil, fmt.Errorf("failed to list OLTs: %w", err)
	}
	return olts, nil
}

func (s *OLTService) Update(id uuid.UUID, updates map[string]interface{}) error {
	if password, ok := updates["password"].(string); ok {
		encryptedPassword, err := utils.Encrypt(password, strings.TrimSpace(string(s.encryptionKey)))
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		updates["password"] = encryptedPassword
	}

	if err := s.db.Model(&models.OLT{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update OLT: %w", err)
	}

	return nil
}

func (s *OLTService) Delete(id uuid.UUID) error {
	if err := s.db.Delete(&models.OLT{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete OLT: %w", err)
	}
	return nil
}

// DiscoverONTs discovers all ONTs connected to this OLT via SNMP topology walk
func (s *OLTService) DiscoverONTs(oltID uuid.UUID) ([]connectivity.DiscoveredONT, error) {
	// Get OLT details
	olt, err := s.GetByID(oltID)
	if err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}

	// Decrypt SNMP community
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		return nil, fmt.Errorf("SNMP community not configured for this OLT")
	}

	// Perform discovery using topology-based approach
	topology, err := connectivity.DiscoverOLTTopology(olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Flatten topology to simple list of discovered ONTs
	var result []connectivity.DiscoveredONT
	for _, slot := range topology {
		for _, gponPort := range slot.Ports {
			result = append(result, gponPort.ONTs...)
		}
	}

	return result, nil
}
