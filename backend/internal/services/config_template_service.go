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

// CreateTemplateRequest is the payload accepted by the config template handler.
type CreateTemplateRequest struct {
	Name         string                 `json:"name" binding:"required"`
	Description  string                 `json:"description"`
	Vendor       string                 `json:"vendor" binding:"required"`
	ConfigFields map[string]interface{} `json:"config_fields"`
	IsDefault    bool                   `json:"is_default"`
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
// templateNameBounds is what a template name may weigh. Short enough to be
// unhelpful and long enough to break the picker are both refused.
const (
	minTemplateName = 3
	maxTemplateName = 100
)

// validateTemplate checks what create and update both require, and returns the
// trimmed name alongside the encoded fields.
func validateTemplate(name, vendor string, configFields map[string]interface{}) (string, []byte, error) {
	if err := validateVendor(vendor); err != nil {
		return "", nil, err
	}

	name = strings.TrimSpace(name)
	if len(name) < minTemplateName || len(name) > maxTemplateName {
		return "", nil, fmt.Errorf("config template name must be between %d and %d characters, got %d",
			minTemplateName, maxTemplateName, len(name))
	}

	configJSON, err := json.Marshal(configFields)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config fields: %w", err)
	}
	return name, configJSON, nil
}

// storeTemplateError names a duplicate for what it is. The constraint reads
// differently on SQLite and Postgres, and the caller needs one answer.
func storeTemplateError(verb string, err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "duplicate key") {
		return fmt.Errorf("config template name must be unique: %w", err)
	}
	return fmt.Errorf("failed to %s config template: %w", verb, err)
}

// templateAuditValue is the part of a template worth recording in the audit
// log: the fields another admin would want to see changed.
func templateAuditValue(name, vendor string, isDefault bool) map[string]interface{} {
	return map[string]interface{}{
		"name":       name,
		"vendor":     vendor,
		"is_default": isDefault,
	}
}

func (s *ConfigTemplateService) Create(
	name, description, vendor string,
	configFields map[string]interface{},
	isDefault bool,
	userID uuid.UUID,
) (*models.ConfigTemplate, error) {
	name, configJSON, err := validateTemplate(name, vendor, configFields)
	if err != nil {
		return nil, err
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
		return nil, storeTemplateError("create", err)
	}

	s.logAudit(userID, "create", template.ID, nil, templateAuditValue(name, vendor, isDefault))

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

	// Read before the write, so the audit entry can say what changed.
	oldValue := templateAuditValue(template.Name, template.Vendor, template.IsDefault)

	name, configJSON, err := validateTemplate(name, vendor, configFields)
	if err != nil {
		return nil, err
	}

	if isDefault && !template.IsDefault {
		if err := s.clearDefaultForVendor(vendor); err != nil {
			return nil, fmt.Errorf("failed to clear existing default: %w", err)
		}
	}

	template.Name = name
	template.Description = description
	template.Vendor = vendor
	template.ConfigFields = datatypes.JSON(configJSON)
	template.IsDefault = isDefault

	if err := s.db.Save(template).Error; err != nil {
		return nil, storeTemplateError("update", err)
	}

	s.logAudit(userID, "update", template.ID, oldValue, templateAuditValue(name, vendor, isDefault))

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
