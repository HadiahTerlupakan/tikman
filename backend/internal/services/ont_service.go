package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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
// ONTListFilter narrows a page of ONTs.
//
// A struct rather than nine positional parameters, and the filters have to be
// applied here rather than in the browser: the page can only filter the rows it
// was sent, so on a network larger than one page it answered from a slice of
// itself and said nothing about the rest.
type ONTListFilter struct {
	OLTID  *uuid.UUID
	Status *models.ONTStatus
	// Slot and PortID address a position. Port alone is ambiguous on a
	// multi-card chassis, where port 1 exists once per card.
	Slot   *int
	PortID *int
	// Search matches the serial or the subscriber name. The serial is what is
	// printed on the box; the name is what an operator is given on the phone.
	Search             string
	StartTime, EndTime *time.Time
	Limit, Offset      int
}

// List returns a page of one OLT's ONTs, or of all of them.
func (s *ONTService) List(oltID *uuid.UUID, status *models.ONTStatus, limit, offset int) ([]models.ONT, int64, error) {
	return s.ListFiltered(ONTListFilter{OLTID: oltID, Status: status, Limit: limit, Offset: offset})
}

// ListFiltered returns one page of ONTs and how many match the filter.
//
// The count describes the whole match, not the page: it drives the pager, and
// reporting the page's own size there is what let a 930-ONT network claim to
// hold however many rows happened to arrive.
func (s *ONTService) ListFiltered(filter ONTListFilter) ([]models.ONT, int64, error) {
	var onts []models.ONT
	var total int64

	query := s.applyONTFilter(s.db.Model(&models.ONT{}), filter)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count ONTs: %w", err)
	}

	// Order by physical position, not by created_at. Newest-first meant a bulk
	// registration buried every other OLT: adding one 246-ONT OLT pushed a
	// 198-ONT OLT past the client's window, so it vanished from the page
	// entirely. Position is stable across polls and keeps an OLT's ONTs
	// together, which also makes offset paging land where the operator expects.
	query = query.Order("olt_id, slot, port_id, ont_id")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if err := query.Offset(filter.Offset).Find(&onts).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list ONTs: %w", err)
	}

	return onts, total, nil
}

func (s *ONTService) applyONTFilter(query *gorm.DB, filter ONTListFilter) *gorm.DB {
	if filter.OLTID != nil {
		query = query.Where("olt_id = ?", *filter.OLTID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Slot != nil {
		query = query.Where("slot = ?", *filter.Slot)
	}
	if filter.PortID != nil {
		query = query.Where("port_id = ?", *filter.PortID)
	}
	if filter.Search != "" {
		// Lowered on both sides rather than using ILIKE: the tests run on SQLite,
		// which has no ILIKE, and a search that behaved differently there than in
		// production would be worse than no test.
		term := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(serial_number) LIKE ? OR LOWER(name) LIKE ?", term, term)
	}
	if filter.StartTime != nil && filter.EndTime != nil {
		query = query.Where(`EXISTS (
			SELECT 1 FROM ont_metrics
			WHERE ont_metrics.ont_id = onts.id
			AND ont_metrics.time >= ?
			AND ont_metrics.time <= ?
		)`, *filter.StartTime, *filter.EndTime)
	}
	return query
}

func (s *ONTService) checkUnclaimed(ont *models.ONT) error {
	// A serial identifies one physical box, so it may appear once across every
	// OLT. An absent serial is not a value though: the inventory walk does not
	// return one for every ONU it finds, and treating "" as a serial made the
	// first serial-less ONU registered lock out every other serial-less ONU in
	// the database. Those ONTs are identified by their position alone.
	var count int64
	if ont.SerialNumber != "" {
		if err := s.db.Model(&models.ONT{}).Where("serial_number = ?", ont.SerialNumber).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check duplicate serial number: %w", err)
		}
		if count > 0 {
			return fmt.Errorf("ONT with serial number %s already exists", ont.SerialNumber)
		}
	}

	// A position is olt + card + port + ONU. Leaving the card out of this check
	// rejected the same port and ONU number on a second card as a duplicate,
	// which on a multi-card chassis is a different subscriber's box.
	positionQuery := s.db.Model(&models.ONT{}).
		Where("olt_id = ? AND port_id = ? AND ont_id = ?", ont.OLTID, ont.PortID, ont.ONTID)
	if ont.Slot != nil {
		positionQuery = positionQuery.Where("slot = ?", *ont.Slot)
	} else {
		positionQuery = positionQuery.Where("slot IS NULL")
	}
	if err := positionQuery.Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check duplicate ONT position: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("ONT position already exists on this OLT (slot %s, port %d, ont_id %d)", describeSlot(ont.Slot), ont.PortID, ont.ONTID)
	}

	return nil
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

// ONTSummary is the projection the discovery handler renders: enough of an
// ONT row to display discovery results without loading full models.
type ONTSummary struct {
	// Slot is the OLT card. Absent for an ONT the poll has not placed yet.
	Slot         *int   `json:"slot"`
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
		Select("slot, port_id, ont_id, serial_number, status, name, description").
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
// describeSlot renders a slot for an error message an operator has to act on,
// where "unknown" is more useful than an empty gap.
func describeSlot(slot *int) string {
	if slot == nil {
		return "unknown"
	}
	return strconv.Itoa(*slot)
}

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
		existing := s.rowForDiscovered(oltID, ont)

		if existing != nil {
			updates := discoveryUpdates(ont, existing)
			if len(updates) == 0 {
				result.Skipped++
				continue
			}
			updates["updated_at"] = time.Now()
			if err := s.db.Model(existing).Updates(updates).Error; err == nil {
				result.Registered++
			}
			continue
		}

		err := s.Create(newONTFromDiscovery(oltID, ont))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Port %d ONT %d: %v", ont.PortID, ont.ONTID, err))
			continue
		}

		result.Registered++
	}

	return result
}

// rowForDiscovered finds the row a discovery record belongs to, by position.
//
// Not by serial: this OLT reports the same serial at two positions in one walk,
// so treating a serial found elsewhere as the same box moved made its row chase
// between the two every cycle. A serial appearing where another row already
// holds it stays an error the log names.
func (s *ONTService) rowForDiscovered(oltID uuid.UUID, ont connectivity.DiscoveredONT) *models.ONT {
	existing, _ := s.GetByOLTAndPosition(oltID, ont.Slot, ont.PortID, ont.ONTID)
	if existing == nil && ont.Slot > 0 {
		// A row registered before the OLT reported card numbers carries a null
		// slot. It is the same ONT, so discovery backfills its card rather than
		// inserting a second row beside it. Only the null case falls back: a row
		// already sitting on a different card is a different subscriber's box.
		existing, _ = s.GetByOLTAndPosition(oltID, 0, ont.PortID, ont.ONTID)
	}
	return existing
}

// newONTFromDiscovery builds the row for an ONU no stored row claims yet.
func newONTFromDiscovery(oltID uuid.UUID, ont connectivity.DiscoveredONT) *models.ONT {
	return &models.ONT{
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
}

// discoveryUpdates returns what the walk knows that the stored row does not.
// An empty map means the row already agrees with the OLT and must not be
// written, which is what keeps updated_at meaningful across repeat scans.
//
// Inventory fields only fill gaps: the walk reads them inconsistently, so an
// operator's correction should outlive the next scan. Name and description are
// the exception, being the OLT's own labels rather than ours.
func discoveryUpdates(ont connectivity.DiscoveredONT, existing *models.ONT) map[string]interface{} {
	updates := map[string]interface{}{}

	if ont.Name != "" && existing.Name != ont.Name {
		updates["name"] = ont.Name
	}
	if ont.Description != "" && existing.Description != ont.Description {
		updates["description"] = ont.Description
	}
	if ont.DeviceType != "" && existing.DeviceType == "" {
		updates["device_type"] = ont.DeviceType
	}
	if ont.HardwareVersion != "" && existing.HardwareVersion == "" {
		updates["hardware_version"] = ont.HardwareVersion
	}
	if ont.SoftwareVersion != "" && existing.SoftwareVersion == "" {
		updates["software_version"] = ont.SoftwareVersion
	}
	if ont.IPAddress != "" && existing.IPAddress == "" {
		updates["ip_address"] = ont.IPAddress
	}
	if ont.MACAddress != "" && existing.MACAddress == "" {
		updates["mac_address"] = ont.MACAddress
	}

	// Backfills rows registered before discovery carried a slot. The auto ONU ID
	// allocator matches on it, and a null one hides the ONT from that lookup,
	// which can hand out an ID already in use.
	if ont.Slot > 0 && existing.Slot == nil {
		updates["slot"] = ont.Slot
	}

	return updates
}
