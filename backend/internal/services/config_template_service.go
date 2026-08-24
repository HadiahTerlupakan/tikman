package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var allowedVendors = []string{
	models.VendorZTE,
	models.VendorHSGQ,
}

type ConfigTemplateService struct {
	db    *gorm.DB
	audit *AuditService
}

func NewConfigTemplateService(db *gorm.DB, audit *AuditService) *ConfigTemplateService {
	return &ConfigTemplateService{
		db:    db,
		audit: audit,
	}
}

// Create validates and persists a new configuration template. It enforces
// name uniqueness, vendor validity, and JSON marshaling rules. When
// isDefault is true, it clears the default flag on other templates of the
// same vendor before persisting the new default.
func (s *ConfigTemplateService) Create(
	name, description, vendor string,
	configFields map[string]interface{},
	isDefault bool,
	userID uuid.UUID,
) (*models.ConfigTemplate, error) {
	// Validate vendor against allowed list
	if err := validateVendor(vendor); err != nil {
		return nil, err
	}

	// Validate name length
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 100 {
		return nil, fmt.Errorf("config template name must be between 3 and 100 characters, got %d", len(name))
	}

	// Marshal config fields to JSON
	configJSON, err := json.Marshal(configFields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config fields: %w", err)
	}

	template := &models.ConfigTemplate{
		Name:         name,
		Description:  description,
		Vendor:       vendor,
		ConfigFields: datatypes.JSON(configJSON),
		IsDefault:    isDefault,
	}

	// Clear default on other templates of same vendor if setting this as default
	if isDefault {
		if err := s.clearDefaultForVendor(vendor); err != nil {
			return nil, fmt.Errorf("failed to clear existing default: %w", err)
		}
	}

	if err := s.db.Create(template).Error; err != nil {
		// Handle unique constraint violation on name
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("config template name must be unique: %w", err)
		}
		return nil, fmt.Errorf("failed to create config template: %w", err)
	}

	// Log the creation
	s.logAudit(userID, "create", template.ID, nil, map[string]interface{}{
		"name":       name,
		"vendor":     vendor,
		"is_default": isDefault,
	})

	return template, nil
}

// GetByID retrieves a config template by ID.
func (s *ConfigTemplateService) GetByID(id uuid.UUID) (*models.ConfigTemplate, error) {
	var template models.ConfigTemplate
	if err := s.db.First(&template, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("config template not found: %w", err)
		}
		return nil, fmt.Errorf("database error: %w", err)
	}
	return &template, nil
}

// List retrieves all config templates ordered by creation date.
func (s *ConfigTemplateService) List() ([]models.ConfigTemplate, error) {
	var templates []models.ConfigTemplate
	if err := s.db.Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("failed to list config templates: %w", err)
	}
	return templates, nil
}

// Update validates and updates an existing config template. The update must
// maintain name uniqueness and vendor validity.
func (s *ConfigTemplateService) Update(
	id uuid.UUID,
	name, description, vendor string,
	configFields map[string]interface{},
	isDefault bool,
	userID uuid.UUID,
) (*models.ConfigTemplate, error) {
	// Fetch existing template
	template, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Capture old state for audit
	oldValue := map[string]interface{}{
		"name":       template.Name,
		"vendor":     template.Vendor,
		"is_default": template.IsDefault,
	}

	// Validate vendor
	if err := validateVendor(vendor); err != nil {
		return nil, err
	}

	// Validate name
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 100 {
		return nil, fmt.Errorf("config template name must be between 3 and 100 characters, got %d", len(name))
	}

	// Marshal config fields to JSON
	configJSON, err := json.Marshal(configFields)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config fields: %w", err)
	}

	// Clear default on other templates of same vendor if setting this as default
	if isDefault && !template.IsDefault {
		if err := s.clearDefaultForVendor(vendor); err != nil {
			return nil, fmt.Errorf("failed to clear existing default: %w", err)
		}
	}

	// Apply updates
	template.Name = name
	template.Description = description
	template.Vendor = vendor
	template.ConfigFields = datatypes.JSON(configJSON)
	template.IsDefault = isDefault

	if err := s.db.Save(template).Error; err != nil {
		// Handle unique constraint violation on name
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, fmt.Errorf("config template name must be unique: %w", err)
		}
		return nil, fmt.Errorf("failed to update config template: %w", err)
	}

	// Log the update
	newValue := map[string]interface{}{
		"name":       name,
		"vendor":     vendor,
		"is_default": isDefault,
	}
	s.logAudit(userID, "update", template.ID, oldValue, newValue)

	return template, nil
}

// Delete removes a config template if it is not referenced by any
// provisioning or batch jobs.
func (s *ConfigTemplateService) Delete(id uuid.UUID) error {
	// Verify template exists
	template, err := s.GetByID(id)
	if err != nil {
		return err
	}

	// Check for provisioning job references
	var jobCount int64
	err = s.db.Model(&models.ProvisioningJob{}).Where("template_id = ?", id).Count(&jobCount).Error
	if err != nil {
		return fmt.Errorf("failed to check provisioning job references: %w", err)
	}
	if jobCount > 0 {
		return fmt.Errorf("config template referenced by %d provisioning job(s), cannot delete", jobCount)
	}

	// Check for batch job references
	var batchCount int64
	err = s.db.Model(&models.BatchJob{}).Where("template_id = ?", id).Count(&batchCount).Error
	if err != nil {
		return fmt.Errorf("failed to check batch job references: %w", err)
	}
	if batchCount > 0 {
		return fmt.Errorf("config template referenced by %d batch job(s), cannot delete", batchCount)
	}

	// Perform deletion
	if err := s.db.Delete(&models.ConfigTemplate{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete config template: %w", err)
	}

	// Log the deletion
	s.logAudit(uuid.Nil, "delete", template.ID, map[string]interface{}{
		"name":   template.Name,
		"vendor": template.Vendor,
	}, nil)

	return nil
}

// GetDefaultByVendor retrieves the default config template for a vendor.
// Returns nil (not an error) if no default is set.
func (s *ConfigTemplateService) GetDefaultByVendor(vendor string) (*models.ConfigTemplate, error) {
	var template models.ConfigTemplate
	err := s.db.First(&template, "vendor = ? AND is_default = ?", vendor, true).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No default set is not an error
		}
		return nil, fmt.Errorf("failed to get default template: %w", err)
	}

	return &template, nil
}

// validateVendor ensures the vendor is in the allowed list.
func validateVendor(vendor string) error {
	vendor = strings.TrimSpace(vendor)
	for _, v := range allowedVendors {
		if v == vendor {
			return nil
		}
	}
	return fmt.Errorf("config template vendor must be one of %v, got %s", allowedVendors, vendor)
}

// clearDefaultForVendor unsets IsDefault on all templates of the given
// vendor.
func (s *ConfigTemplateService) clearDefaultForVendor(vendor string) error {
	return s.db.Model(&models.ConfigTemplate{}).
		Where("vendor = ? AND is_default = ?", vendor, true).
		Update("is_default", false).Error
}

// logAudit sends an audit log entry if an audit service is configured.
func (s *ConfigTemplateService) logAudit(
	userID uuid.UUID,
	action string,
	resourceID uuid.UUID,
	oldValue, newValue map[string]interface{},
) {
	if s.audit == nil {
		return
	}

	_ = s.audit.Log(
		userID,
		action,
		"config_template",
		resourceID,
		oldValue,
		newValue,
		"", // IPAddress - empty for service layer
		"", // UserAgent - empty for service layer
	)
}
