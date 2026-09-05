package services

import (
	"encoding/json"
	"fmt"
	"log"
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

// macSeparators strips the punctuation a MAC is written with, so a term and a
// stored address compare on their digits alone.
var macSeparators = strings.NewReplacer(":", "", "-", "", ".", "", " ", "")

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
		lowered := strings.ToLower(filter.Search)
		term := "%" + lowered + "%"

		// A MAC is stored colon separated but arrives typed however the router
		// page, sticker or spreadsheet it was copied from wrote it, so the
		// separators come off both sides before matching. Skipped when nothing
		// survives that: an empty pattern would match every ONT that has a MAC.
		if bare := macSeparators.Replace(lowered); bare != "" {
			return query.Where(
				"LOWER(serial_number) LIKE ? OR LOWER(name) LIKE ? OR REPLACE(LOWER(mac_address), ':', '') LIKE ?",
				term, term, "%"+bare+"%")
		}
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
