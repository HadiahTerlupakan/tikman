package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// ONTService handles ONT business logic
type ONTService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewONTService creates a new ONT service
func NewONTService(db *gorm.DB) *ONTService {
	return &ONTService{db: db}
}

// NewONTServiceWithEncryption adds the key needed to read back a subscriber's
// stored PPPoE password. Without it the service still works and simply never
// returns one.
func NewONTServiceWithEncryption(db *gorm.DB, encryptionKey string) *ONTService {
	return &ONTService{db: db, encryptionKey: encryptionKey}
}

// GetServiceConfig returns the ONU's provisioned service as the last poll read
// it, with the PPPoE password decrypted so a reconfigure can resend it.
func (s *ONTService) GetServiceConfig(ontID uuid.UUID) (*connectivity.ZTEONUService, *time.Time, error) {
	var ont models.ONT
	if err := s.db.First(&ont, "id = ?", ontID).Error; err != nil {
		return nil, nil, fmt.Errorf("ONT not found: %w", err)
	}
	if len(ont.ServiceConfig) == 0 {
		return nil, ont.ServiceConfigAt, nil
	}

	var service connectivity.ZTEONUService
	if err := json.Unmarshal(ont.ServiceConfig, &service); err != nil {
		return nil, nil, fmt.Errorf("stored service config is unreadable: %w", err)
	}

	if ont.PPPoEPassword != "" && s.encryptionKey != "" {
		password, err := utils.Decrypt(ont.PPPoEPassword, s.encryptionKey)
		if err != nil {
			// A password that cannot be decrypted is left out rather than failing
			// the whole read: everything else still pre-fills the form.
			log.Printf("[ONT] Cannot decrypt stored PPPoE password for %s: %v", ontID, err)
		} else {
			service.PPPoEPassword = password
		}
	}

	return &service, ont.ServiceConfigAt, nil
}

// GetDB returns the database instance
func (s *ONTService) GetDB() *gorm.DB {
	return s.db
}

// GetONTOlts returns distinct OLTs that have ONTs in the given ID list.
// Query logic belongs in services, not handlers.
func (s *ONTService) GetONTOlts(oltIDs []uuid.UUID) ([]models.OLT, error) {
	var olts []models.OLT
	err := s.db.Select("id, name").Where("id IN ?", oltIDs).Find(&olts).Error
	return olts, err
}

// List returns paginated list of ONTs with filters
func (s *ONTService) List(oltID *uuid.UUID, status *models.ONTStatus, limit, offset int) ([]models.ONT, int64, error) {
	return s.ListWithMetricsFilter(oltID, status, nil, nil, limit, offset)
}

// ListWithMetricsFilter returns ONTs that match entity filters and optional metrics time range.
func (s *ONTService) ListWithMetricsFilter(oltID *uuid.UUID, status *models.ONTStatus, startTime, endTime *time.Time, limit, offset int) ([]models.ONT, int64, error) {
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
	if startTime != nil && endTime != nil {
		query = query.Where(`EXISTS (
			SELECT 1 FROM ont_metrics
			WHERE ont_metrics.ont_id = onts.id
			AND ont_metrics.time >= ?
			AND ont_metrics.time <= ?
		)`, *startTime, *endTime)
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count ONTs: %w", err)
	}

	// Get paginated results
	// Order by physical position, not by created_at. Newest-first meant a bulk
	// registration buried every other OLT: adding one 246-ONT OLT pushed a
	// 198-ONT OLT past the client's 200-row window, so it vanished from the page
	// entirely. Position is stable across polls and keeps an OLT's ONTs together.
	if err := query.Order("olt_id, port_id, ont_id").Limit(limit).Offset(offset).Find(&onts).Error; err != nil {
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

// ONTSummary is the projection the discovery handler renders: enough of an
// ONT row to display discovery results without loading full models.
type ONTSummary struct {
	PortID       int    `json:"port_id"`
	ONTID        int    `json:"ont_id"`
	SerialNumber string `json:"serial_number"`
	Status       string `json:"status"`
	Name         string `json:"name"`
	Description  string `json:"description"`
}

// ListONTSummariesForOLT returns the discovery projection of every ONT on an
// OLT. The handler used to run this query itself via GetDB(); the query
// belongs here.
func (s *ONTService) ListONTSummariesForOLT(oltID uuid.UUID) ([]ONTSummary, error) {
	var onts []ONTSummary
	err := s.db.Table("onts").
		Select("port_id, ont_id, serial_number, status, name, description").
		Where("olt_id = ?", oltID).
		Scan(&onts).Error
	return onts, err
}

// CountONTsByOLT returns how many ONTs belong to a single OLT.
func (s *ONTService) CountONTsByOLT(oltID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.Model(&models.ONT{}).Where("olt_id = ?", oltID).Count(&count).Error
	return count, err
}

// BulkRegisterResult holds the result of bulk ONT registration
type BulkRegisterResult struct {
	Registered int
	Skipped    int
	Errors     []string
}

// BulkRegisterFromDiscovery registers multiple ONTs from discovery results
// discoveredSlot keeps a slot the OLT did not report out of the row, so an
// unknown slot stays null rather than becoming a real-looking zero.
func discoveredSlot(ont connectivity.DiscoveredONT) *int {
	if ont.Slot <= 0 {
		return nil
	}
	slot := ont.Slot
	return &slot
}

func (s *ONTService) BulkRegisterFromDiscovery(oltID uuid.UUID, discovered []connectivity.DiscoveredONT) *BulkRegisterResult {
	result := &BulkRegisterResult{
		Errors: make([]string, 0),
	}

	for _, ont := range discovered {
		existing, _ := s.GetByOLTAndPosition(oltID, ont.PortID, ont.ONTID)
		if existing != nil {
			updates := map[string]interface{}{}
			needsUpdate := false

			if ont.Name != "" && existing.Name != ont.Name {
				updates["name"] = ont.Name
				needsUpdate = true
			}
			if ont.Description != "" && existing.Description != ont.Description {
				updates["description"] = ont.Description
				needsUpdate = true
			}
			if ont.DeviceType != "" && existing.DeviceType == "" {
				updates["device_type"] = ont.DeviceType
				needsUpdate = true
			}
			if ont.HardwareVersion != "" && existing.HardwareVersion == "" {
				updates["hardware_version"] = ont.HardwareVersion
				needsUpdate = true
			}
			if ont.SoftwareVersion != "" && existing.SoftwareVersion == "" {
				updates["software_version"] = ont.SoftwareVersion
				needsUpdate = true
			}
			if ont.IPAddress != "" && existing.IPAddress == "" {
				updates["ip_address"] = ont.IPAddress
				needsUpdate = true
			}
			if ont.MACAddress != "" && existing.MACAddress == "" {
				updates["mac_address"] = ont.MACAddress
				needsUpdate = true
			}
			// Backfills rows registered before discovery carried a slot. The auto
			// ONU ID allocator matches on it, and a null one hides the ONT from
			// that lookup, which can hand out an ID already in use.
			if ont.Slot > 0 && existing.Slot == nil {
				updates["slot"] = ont.Slot
				needsUpdate = true
			}

			if needsUpdate {
				updates["updated_at"] = time.Now()
				if err := s.db.Model(existing).Updates(updates).Error; err == nil {
					result.Registered++
				}
			} else {
				result.Skipped++
			}
			continue
		}

		newONT := &models.ONT{
			OLTID:           oltID,
			Slot:            discoveredSlot(ont),
			PortID:          ont.PortID,
			ONTID:           ont.ONTID,
			SerialNumber:    ont.SerialNumber,
			Name:            ont.Name,
			Description:     ont.Description,
			DeviceType:      ont.DeviceType,
			HardwareVersion: ont.HardwareVersion,
			SoftwareVersion: ont.SoftwareVersion,
			IPAddress:       ont.IPAddress,
			MACAddress:      ont.MACAddress,
			// The discovery walk already read this ONT's phase state, so storing
			// "unknown" here threw away a fact we had and left the ONT list showing
			// UNKNOWN until the next status poll happened to run. Newly registered
			// ONTs sort first by created_at, so those placeholders were exactly the
			// rows an operator saw first after adding an OLT.
			Status: models.ONTStatus(utils.StatusMap(ont.RunState)),
		}

		err := s.Create(newONT)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Port %d ONT %d: %v", ont.PortID, ont.ONTID, err))
			continue
		}

		result.Registered++
	}

	return result
}
