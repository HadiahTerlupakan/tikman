package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

type OLTService struct {
	db               *gorm.DB
	encryptionKey    string
	validatorService *OLTValidatorService
}

func NewOLTService(db *gorm.DB, encryptionKey string) *OLTService {
	return &OLTService{
		db:               db,
		encryptionKey:    encryptionKey,
		validatorService: NewOLTValidatorService(db),
	}
}

func (s *OLTService) encryptPassword(plaintext string) (string, error) {
	return utils.Encrypt(plaintext, s.encryptionKey)
}

func (s *OLTService) DecryptPassword(ciphertext string) (string, error) {
	return utils.Decrypt(ciphertext, s.encryptionKey)
}

func (s *OLTService) Create(siteID uuid.UUID, name, ipAddress, username, password string,
	sshPort, telnetPort, snmpPort int, snmpCommunity string, preferredProtocol models.OLTProtocol) (*models.OLT, error) {

	// Validate site exists
	var site models.Site
	if err := s.db.First(&site, "id = ?", siteID).Error; err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// Validate IP not duplicate
	if err := s.validatorService.ValidateIPNotDuplicate(ipAddress); err != nil {
		return nil, err
	}

	// Run connection tests
	validationResult, err := s.validatorService.ValidateCreate(
		ipAddress,
		username,
		password,
		sshPort,
		telnetPort,
		snmpPort,
		snmpCommunity,
		preferredProtocol,
	)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	if !validationResult.Success {
		return nil, fmt.Errorf("OLT validation failed - Passed: %v, Failed: %s (%s)",
			validationResult.PassedTests,
			validationResult.FailedTest,
			validationResult.FailedReason,
		)
	}

	// Encrypt password
	encryptedPassword, err := s.encryptPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	// Create OLT with status "online" since validation passed
	olt := &models.OLT{
		SiteID:            siteID,
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
		encryptedPassword, err := s.encryptPassword(password)
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

func (s *OLTService) UpdateStatus(id uuid.UUID, status models.OLTStatus) error {
	now := time.Now().UTC()
	return s.db.Model(&models.OLT{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    status,
		"last_seen": &now,
	}).Error
}
