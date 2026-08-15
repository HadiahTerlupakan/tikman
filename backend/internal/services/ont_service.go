package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ONTService handles ONT business logic
type ONTService struct {
	db *gorm.DB
}

// NewONTService creates a new ONT service
func NewONTService(db *gorm.DB) *ONTService {
	return &ONTService{db: db}
}

// List returns paginated list of ONTs with filters
func (s *ONTService) List(oltID *uuid.UUID, status *models.ONTStatus, limit, offset int) ([]models.ONT, int64, error) {
	var onts []models.ONT
	var total int64

	query := s.db.Model(&models.ONT{})

	// Apply filters
	if oltID != nil {
		query = query.Where("olt_id = ?", *oltID)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count ONTs: %w", err)
	}

	// Get paginated results
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&onts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list ONTs: %w", err)
	}

	return onts, total, nil
}

// GetByID returns ONT by ID
func (s *ONTService) GetByID(id uuid.UUID) (*models.ONT, error) {
	var ont models.ONT
	if err := s.db.Where("id = ?", id).First(&ont).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("ONT not found")
		}
		return nil, fmt.Errorf("failed to get ONT: %w", err)
	}
	return &ont, nil
}

// GetBySerialNumber returns ONT by serial number
func (s *ONTService) GetBySerialNumber(serialNumber string) (*models.ONT, error) {
	var ont models.ONT
	if err := s.db.Where("serial_number = ?", serialNumber).First(&ont).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("ONT not found")
		}
		return nil, fmt.Errorf("failed to get ONT: %w", err)
	}
	return &ont, nil
}

// Create creates a new ONT
func (s *ONTService) Create(ont *models.ONT) error {
	// Validate OLT exists
	var oltExists bool
	if err := s.db.Model(&models.OLT{}).Select("count(*) > 0").Where("id = ?", ont.OLTID).Find(&oltExists).Error; err != nil {
		return fmt.Errorf("failed to validate OLT: %w", err)
	}
	if !oltExists {
		return fmt.Errorf("OLT not found")
	}

	// Check for duplicate serial number
	var count int64
	if err := s.db.Model(&models.ONT{}).Where("serial_number = ?", ont.SerialNumber).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check duplicate serial number: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("ONT with serial number %s already exists", ont.SerialNumber)
	}

	// Check for duplicate OLT + PortID + ONTID
	if err := s.db.Model(&models.ONT{}).
		Where("olt_id = ? AND port_id = ? AND ont_id = ?", ont.OLTID, ont.PortID, ont.ONTID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check duplicate ONT position: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("ONT position already exists on this OLT (port %d, ont_id %d)", ont.PortID, ont.ONTID)
	}

	// Set default status if not provided
	if ont.Status == "" {
		ont.Status = models.ONTStatusUnknown
	}

	if err := s.db.Create(ont).Error; err != nil {
		return fmt.Errorf("failed to create ONT: %w", err)
	}

	return nil
}

// Update updates an existing ONT
func (s *ONTService) Update(id uuid.UUID, updates map[string]interface{}) (*models.ONT, error) {
	ont, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Validate serial number uniqueness if changing
	if newSerial, ok := updates["serial_number"].(string); ok && newSerial != ont.SerialNumber {
		var count int64
		if err := s.db.Model(&models.ONT{}).Where("serial_number = ? AND id != ?", newSerial, id).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to check duplicate serial number: %w", err)
		}
		if count > 0 {
			return nil, fmt.Errorf("ONT with serial number %s already exists", newSerial)
		}
	}

	updates["updated_at"] = time.Now()

	if err := s.db.Model(ont).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update ONT: %w", err)
	}

	return ont, nil
}

// Delete deletes an ONT
func (s *ONTService) Delete(id uuid.UUID) error {
	result := s.db.Delete(&models.ONT{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete ONT: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("ONT not found")
	}
	return nil
}

// UpdateStatus updates ONT status and last_seen_at
func (s *ONTService) UpdateStatus(id uuid.UUID, status models.ONTStatus) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       status,
		"last_seen_at": now,
		"updated_at":   now,
	}

	if err := s.db.Model(&models.ONT{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update ONT status: %w", err)
	}

	return nil
}

// GetByOLTAndPosition returns ONT by OLT ID, port, and ONT ID
func (s *ONTService) GetByOLTAndPosition(oltID uuid.UUID, portID, ontID int) (*models.ONT, error) {
	var ont models.ONT
	if err := s.db.Where("olt_id = ? AND port_id = ? AND ont_id = ?", oltID, portID, ontID).First(&ont).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("ONT not found at position")
		}
		return nil, fmt.Errorf("failed to get ONT: %w", err)
	}
	return &ont, nil
}

// BulkRegisterResult holds the result of bulk ONT registration
type BulkRegisterResult struct {
	Registered int
	Skipped    int
	Errors     []string
}

// BulkRegisterFromDiscovery registers multiple ONTs from discovery results
func (s *ONTService) BulkRegisterFromDiscovery(oltID uuid.UUID, discovered []connectivity.DiscoveredONT) *BulkRegisterResult {
	result := &BulkRegisterResult{
		Errors: make([]string, 0),
	}

	for _, ont := range discovered {
		// Check if already exists
		existing, _ := s.GetByOLTAndPosition(oltID, ont.PortID, ont.ONTID)
		if existing != nil {
			result.Skipped++
			continue
		}

		// Create new ONT model
		newONT := &models.ONT{
			OLTID:        oltID,
			PortID:       ont.PortID,
			ONTID:        ont.ONTID,
			SerialNumber: ont.SerialNumber,
			Description:  "",
			Status:       models.ONTStatusUnknown,
		}

		// Register new ONT
		err := s.Create(newONT)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Port %d ONT %d: %v", ont.PortID, ont.ONTID, err))
			continue
		}

		result.Registered++
	}

	return result
}
