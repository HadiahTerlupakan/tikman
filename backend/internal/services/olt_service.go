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
	db               *gorm.DB
	encryptionKey    []byte
	commanderFactory CommanderFactory
}

// NewOLTServiceWithCommander adds CLI access, which the discovery poll uses to
// read profile lists the OLT does not publish over SNMP. Without a factory the
// service still works and simply skips them.
func NewOLTServiceWithCommander(db *gorm.DB, encryptionKey string, factory CommanderFactory) *OLTService {
	service := NewOLTService(db, encryptionKey)
	service.commanderFactory = factory
	return service
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
// CreateOLTInput carries everything needed to register an OLT. It is a struct
// rather than a parameter list because the list had reached fourteen, six of
// them consecutive ints — rack, shelf, slot and three ports — where swapping
// two at a call site compiles cleanly and fails only against real hardware.
type CreateOLTInput struct {
	SiteID            uuid.UUID
	Name              string
	IPAddress         string
	SNMPCommunity     string
	Username          string
	Password          string
	Model             models.OLTModel
	SSHPort           int
	TelnetPort        int
	SNMPPort          int
	PreferredProtocol models.OLTProtocol
	// Rack, shelf and slot are deliberately absent: the previous signature took
	// them and never used them, because discovery resolves the physical
	// position at ONT level. Carrying them here would restate that mistake.
	Latitude  *float64
	Longitude *float64
}

// Create registers an OLT.
func (s *OLTService) Create(in CreateOLTInput) (*models.OLT, error) {
	if err := validateOLTInput(in); err != nil {
		return nil, err
	}

	encryptedPassword, err := utils.Encrypt(in.Password, strings.TrimSpace(string(s.encryptionKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	// The site has to exist. This used to be uuid.New(), so every OLT created
	// through the API pointed at a site that was never there: the list showed a
	// blank Site column and any per-site query silently matched nothing. Checked
	// after the free input validation above and before the SNMP probe, so the
	// database is only touched once the request is structurally sound.
	var site models.Site
	if err := s.db.First(&site, "id = ?", in.SiteID).Error; err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	if in.SNMPCommunity != "" {
		if err := connectivity.SNMPTest(in.IPAddress, in.SNMPPort, in.SNMPCommunity, 0); err != nil {
			return nil, fmt.Errorf("SNMP connection test failed: %w", err)
		}
	}

	// No rack, shelf or slot: discovery works those out at ONT level.
	olt := &models.OLT{
		SiteID:            in.SiteID,
		Name:              in.Name,
		IPAddress:         in.IPAddress,
		SSHPort:           in.SSHPort,
		TelnetPort:        in.TelnetPort,
		SNMPPort:          in.SNMPPort,
		SNMPCommunity:     in.SNMPCommunity,
		Latitude:          in.Latitude,
		Longitude:         in.Longitude,
		PreferredProtocol: in.PreferredProtocol,
		Model:             in.Model,
		Username:          in.Username,
		Password:          encryptedPassword,
		Status:            models.OLTStatusOnline,
	}

	if err := s.db.Create(olt).Error; err != nil {
		return nil, fmt.Errorf("failed to create OLT: %w", err)
	}

	return olt, nil
}

// validateOLTInput checks everything that can be judged without touching the
// database or the network.
func validateOLTInput(in CreateOLTInput) error {
	if err := validateCoordinates(in.Latitude, in.Longitude); err != nil {
		return err
	}

	// A model with no driver would leave the OLT unmonitorable, so it is
	// rejected here as well as at the API boundary.
	if _, err := connectivity.DriverFor(in.Model); err != nil {
		return err
	}

	ports := map[string]int{
		"SNMP":   in.SNMPPort,
		"SSH":    in.SSHPort,
		"Telnet": in.TelnetPort,
	}
	for name, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid %s port: %d", name, port)
		}
	}
	return nil
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
	// Shares validateCoordinates with SiteService: the rule is a property of a
	// coordinate pair, not of the thing carrying it, so it lives in one place.
	latitude, hasLatitude := updates["latitude"].(*float64)
	longitude, hasLongitude := updates["longitude"].(*float64)
	if hasLatitude || hasLongitude {
		if err := validateCoordinates(latitude, longitude); err != nil {
			return err
		}
	}

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

// Delete removes an OLT and all dependent data in one transaction: ONTs,
// their metrics, and their events. Without this cleanup, deleting an OLT
// left orphaned ONT rows that still showed up in listings.
func (s *OLTService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var ontIDs []uuid.UUID
		if err := tx.Model(&models.ONT{}).Where("olt_id = ?", id).Pluck("id", &ontIDs).Error; err != nil {
			return fmt.Errorf("failed to list ONTs for OLT: %w", err)
		}

		if len(ontIDs) > 0 {
			if tx.Migrator().HasTable("ont_metrics") {
				for _, ontID := range ontIDs {
					if err := tx.Exec("DELETE FROM ont_metrics WHERE ont_id = ?", ontID).Error; err != nil {
						return fmt.Errorf("failed to delete ONT metrics: %w", err)
					}
				}
			}
		}
		if len(ontIDs) > 0 {
			if err := tx.Where("ont_id IN ?", ontIDs).Delete(&models.ONTEvent{}).Error; err != nil {
				return fmt.Errorf("failed to delete ONT events: %w", err)
			}
		}
		if err := tx.Delete(&models.ONT{}, "olt_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete ONTs: %w", err)
		}

		result := tx.Delete(&models.OLT{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete OLT: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("OLT not found")
		}
		return nil
	})
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
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, snmpCommunity, olt.SNMPPort)
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

// SiteNameForOLT resolves the site name for an OLT row. A missing or unset
// site yields an empty string rather than an error, mirroring the previous
// in-DTO behaviour.
func (s *OLTService) SiteNameForOLT(siteID uuid.UUID) string {
	if siteID == uuid.Nil {
		return ""
	}
	var site models.Site
	if err := s.db.Where("id = ?", siteID).First(&site).Error; err == nil {
		return site.Name
	}
	return ""
}
