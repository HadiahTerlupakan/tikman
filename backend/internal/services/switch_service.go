package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// Default ports for a switch, by how its CLI is reached.
const (
	DefaultSwitchSNMPPort   = 161
	defaultSwitchTelnetPort = 23
	defaultSwitchSSHPort    = 22
)

// switchPollKinds are the jobs a new switch is polled with.
var switchPollKinds = []models.SwitchPollKind{
	models.SwitchPollKindInterfaces, models.SwitchPollKindInventory, models.SwitchPollKindBackup,
}

// SwitchService stores managed switches and their credentials.
type SwitchService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewSwitchService creates a switch service.
func NewSwitchService(db *gorm.DB, encryptionKey string) *SwitchService {
	return &SwitchService{db: db, encryptionKey: strings.TrimSpace(encryptionKey)}
}

// SwitchInput is a new switch.
type SwitchInput struct {
	Name          string
	Host          string
	SiteID        *uuid.UUID
	Vendor        models.SwitchVendor
	SNMPCommunity string
	SNMPPort      int
	Protocol      models.SwitchProtocol
	Port          int
	Username      string
	Password      string
	// HostKeyFingerprint is the SSH key the admin confirmed in the form's
	// connection test, so a new switch is pinned from its first poll.
	HostKeyFingerprint string
}

// SwitchUpdate changes some of a switch's fields. A nil or empty secret keeps
// the stored one, since the form cannot show what it does not know.
type SwitchUpdate struct {
	Name          *string
	Host          *string
	SiteID        *uuid.UUID
	ClearSite     bool
	SNMPCommunity *string
	SNMPPort      *int
	Protocol      *models.SwitchProtocol
	Port          *int
	Username      *string
	Password      *string
	Enabled       *bool
	RackPosition  *int
	// ClearRackPosition takes the switch out of the rack numbering.
	ClearRackPosition bool
}

// Create stores a switch and makes every poll tier due at once, so the first
// readings arrive within a minute.
func (s *SwitchService) Create(in SwitchInput) (*models.Switch, error) {
	if err := validateSwitchInput(in); err != nil {
		return nil, err
	}
	community, err := utils.Encrypt(in.SNMPCommunity, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt community: %w", err)
	}
	password, err := utils.Encrypt(in.Password, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}
	sw := &models.Switch{
		Name: strings.TrimSpace(in.Name), Host: strings.TrimSpace(in.Host), SiteID: in.SiteID, Vendor: in.Vendor,
		SNMPCommunity: community, SNMPPort: portOr(in.SNMPPort, DefaultSwitchSNMPPort),
		Protocol: in.Protocol, Port: portOr(in.Port, defaultSwitchPort(in.Protocol)),
		Username: in.Username, Password: password, HostKeyFingerprint: in.HostKeyFingerprint,
		Enabled: true, Status: models.SwitchStatusUnknown,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sw).Error; err != nil {
			return err
		}
		return createSwitchJobs(tx, sw.ID)
	})
	if err != nil {
		return nil, err
	}
	return sw, nil
}

func createSwitchJobs(tx *gorm.DB, switchID uuid.UUID) error {
	now := time.Now()
	for _, kind := range switchPollKinds {
		if err := tx.Create(&models.SwitchPollJob{SwitchID: switchID, Kind: kind, DueAt: now}).Error; err != nil {
			return err
		}
	}
	return nil
}

func defaultSwitchPort(protocol models.SwitchProtocol) int {
	if protocol == models.SwitchProtocolSSH {
		return defaultSwitchSSHPort
	}
	return defaultSwitchTelnetPort
}

func validateSwitchInput(in SwitchInput) error {
	if err := validateSwitchVendor(in.Vendor, in.Protocol); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(in.Name) == "":
		return fmt.Errorf("%w: name is required", ErrValidation)
	case strings.TrimSpace(in.Host) == "":
		return fmt.Errorf("%w: host is required", ErrValidation)
	case in.SNMPCommunity == "":
		return fmt.Errorf("%w: SNMP community is required", ErrValidation)
	case in.Username == "" || in.Password == "":
		return fmt.Errorf("%w: username and password are required", ErrValidation)
	case in.HostKeyFingerprint != "" && in.Protocol != models.SwitchProtocolSSH:
		return fmt.Errorf("%w: hanya SSH yang punya host key", ErrValidation)
	case in.HostKeyFingerprint != "":
		return validateFingerprint(in.HostKeyFingerprint)
	}
	return nil
}

// validateSwitchVendor admits only what a driver exists for. The column holds
// all three vendors so adding one later is code, not a migration.
func validateSwitchVendor(vendor models.SwitchVendor, protocol models.SwitchProtocol) error {
	switch vendor {
	case models.SwitchVendorHuaweiCE:
		if protocol != models.SwitchProtocolTelnet && protocol != models.SwitchProtocolSSH {
			return fmt.Errorf("%w: huawei_ce is reached over telnet or ssh", ErrValidation)
		}
		return nil
	case models.SwitchVendorCRS, models.SwitchVendorFiberHome:
		return fmt.Errorf("%w: vendor %s belum didukung", ErrValidation, vendor)
	}
	return fmt.Errorf("%w: vendor must be crs, huawei_ce or fiberhome", ErrValidation)
}

// Update applies the fields that were given.
func (s *SwitchService) Update(id uuid.UUID, upd SwitchUpdate) (*models.Switch, error) {
	current, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	updates, err := s.switchColumns(current, upd)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.db.Model(&models.Switch{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

func (s *SwitchService) switchColumns(current *models.Switch, upd SwitchUpdate) (map[string]interface{}, error) {
	updates := map[string]interface{}{}
	if err := switchTextColumns(updates, upd); err != nil {
		return nil, err
	}
	if upd.Protocol != nil {
		if err := validateSwitchVendor(current.Vendor, *upd.Protocol); err != nil {
			return nil, err
		}
		updates["protocol"] = *upd.Protocol
	}
	setIfGiven(updates, "snmp_port", upd.SNMPPort)
	setIfGiven(updates, "port", upd.Port)
	setIfGiven(updates, "enabled", upd.Enabled)
	setIfGiven(updates, "site_id", upd.SiteID)
	if upd.ClearSite {
		updates["site_id"] = nil
	}
	if upd.RackPosition != nil && (*upd.RackPosition < minRackPosition || *upd.RackPosition > maxRackPosition) {
		return nil, fmt.Errorf("%w: rack_position must be between %d and %d", ErrValidation, minRackPosition, maxRackPosition)
	}
	setIfGiven(updates, "rack_position", upd.RackPosition)
	if upd.ClearRackPosition {
		updates["rack_position"] = nil
	}
	return updates, s.encryptSwitchSecrets(updates, upd)
}

func switchTextColumns(updates map[string]interface{}, upd SwitchUpdate) error {
	for column, value := range map[string]*string{"name": upd.Name, "host": upd.Host, "username": upd.Username} {
		if value == nil {
			continue
		}
		if strings.TrimSpace(*value) == "" {
			return fmt.Errorf("%w: %s is required", ErrValidation, column)
		}
		updates[column] = strings.TrimSpace(*value)
	}
	return nil
}

func (s *SwitchService) encryptSwitchSecrets(updates map[string]interface{}, upd SwitchUpdate) error {
	for column, secret := range map[string]*string{"snmp_community": upd.SNMPCommunity, "password": upd.Password} {
		if secret == nil || *secret == "" {
			continue
		}
		encrypted, err := utils.Encrypt(*secret, s.encryptionKey)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", column, err)
		}
		updates[column] = encrypted
	}
	return nil
}

// switchChildren are the tables holding rows of a switch. PostgreSQL removes
// them by foreign key (migration 84); the SQLite test schema has no foreign
// keys, and deleting them here keeps both databases saying the same thing.
var switchChildren = []interface{}{
	&models.SwitchPollJob{}, &models.SwitchInterface{}, &models.SwitchVLAN{},
	&models.SwitchNeighbor{}, &models.SwitchConfigBackup{},
}

// Delete removes a switch and its rows. Its metric samples age out by
// retention.
func (s *SwitchService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&models.Switch{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		for _, child := range switchChildren {
			if err := tx.Delete(child, "switch_id = ?", id).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Get returns one switch.
func (s *SwitchService) Get(id uuid.UUID) (*models.Switch, error) {
	var sw models.Switch
	if err := s.db.First(&sw, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &sw, nil
}

// List returns every switch by name.
func (s *SwitchService) List() ([]models.Switch, error) {
	var switches []models.Switch
	return switches, s.db.Order("name").Find(&switches).Error
}

// Secrets decrypts a switch's SNMP community and CLI password for a poll.
func (s *SwitchService) Secrets(sw *models.Switch) (community, password string, err error) {
	if community, err = utils.Decrypt(sw.SNMPCommunity, s.encryptionKey); err != nil {
		return "", "", fmt.Errorf("decrypt community: %w", err)
	}
	if password, err = utils.Decrypt(sw.Password, s.encryptionKey); err != nil {
		return "", "", fmt.Errorf("decrypt password: %w", err)
	}
	return community, password, nil
}
