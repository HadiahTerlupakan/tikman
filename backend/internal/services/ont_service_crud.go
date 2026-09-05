package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// The row-level operations on a single ONT.

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

	if err := s.checkUnclaimed(ont); err != nil {
		return err
	}

	phone, err := s.resolvePhone(ont.Phone, uuid.Nil)
	if err != nil {
		return err
	}
	ont.Phone = phone

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
	if newSerial, ok := updates["serial_number"].(string); ok && newSerial != "" && newSerial != ont.SerialNumber {
		var count int64
		if err := s.db.Model(&models.ONT{}).Where("serial_number = ? AND id != ?", newSerial, id).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to check duplicate serial number: %w", err)
		}
		if count > 0 {
			return nil, fmt.Errorf("ONT with serial number %s already exists", newSerial)
		}
	}

	// A missing key leaves the stored number untouched; a present one — even ""
	// — is normalized (or, for "", cleared) the same way Create validates it.
	if rawPhone, ok := updates["phone"].(string); ok {
		phone, err := s.resolvePhone(rawPhone, id)
		if err != nil {
			return nil, err
		}
		updates["phone"] = phone
	}

	updates["updated_at"] = time.Now()

	if err := s.db.Model(ont).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update ONT: %w", err)
	}

	return ont, nil
}

// Delete deletes an ONT and its collected data.
func (s *ONTService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&models.ONT{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete ONT: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("ONT not found")
		}

		if err := tx.Exec("DELETE FROM ont_metrics WHERE ont_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete ONT metrics: %w", err)
		}
		if err := tx.Delete(&models.ONTEvent{}, "ont_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete ONT events: %w", err)
		}

		return nil
	})
}

// UpdateStatus updates ONT status. last_seen_at is only refreshed when the ONT
// is actually reachable (online), so it keeps meaning "last time seen up".
func (s *ONTService) UpdateStatus(id uuid.UUID, status models.ONTStatus) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}
	if status == models.ONTStatusOnline {
		updates["last_seen_at"] = now
		updates["last_online"] = now
	} else {
		updates["last_offline"] = now
		updates["last_offline_reason"] = string(status)
	}

	if err := s.db.Model(&models.ONT{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update ONT status: %w", err)
	}

	return nil
}

func (s *ONTService) UpdateUptimeMetrics(id uuid.UUID) error {
	ont, err := s.GetByID(id)
	if err != nil {
		return err
	}

	now := time.Now()
	updates := make(map[string]interface{})

	if ont.Status == models.ONTStatusOnline && ont.LastOnline != nil {
		uptime := int64(now.Sub(*ont.LastOnline).Seconds())
		updates["uptime"] = uptime
	}

	if ont.Status != models.ONTStatusOnline && ont.LastOffline != nil {
		downtime := int64(now.Sub(*ont.LastOffline).Seconds())
		updates["last_down_time_duration"] = downtime
	}

	if len(updates) > 0 {
		updates["updated_at"] = now
		if err := s.db.Model(&models.ONT{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update uptime metrics: %w", err)
		}
	}

	return nil
}

// GetByOLTAndPosition returns the ONT at a position on one OLT.
//
// Slot is part of the address, not decoration: a chassis carries several cards,
// so port 1 / ONU 5 on card 8 and the same position on card 9 are two different
// subscribers' boxes. Matching on port and ONU alone made the second one look
// like the first, and discovery then overwrote a live ONT instead of adding it.
//
// A slot of 0 matches rows whose slot is unknown, which is how ONTs registered
// before the OLT reported card numbers are still found.
func (s *ONTService) GetByOLTAndPosition(oltID uuid.UUID, slot, portID, ontID int) (*models.ONT, error) {
	var ont models.ONT
	query := s.db.Where("olt_id = ? AND port_id = ? AND ont_id = ?", oltID, portID, ontID)
	if slot > 0 {
		query = query.Where("slot = ?", slot)
	} else {
		query = query.Where("slot IS NULL")
	}
	if err := query.First(&ont).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("ONT not found at position")
		}
		return nil, fmt.Errorf("failed to get ONT: %w", err)
	}
	return &ont, nil
}
