package services

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// Router ports used when none is given. REST sits on 8443 because RouterOS 7's
// reverse-proxy commonly holds 443.
const (
	DefaultRouterSNMPPort = 161
	DefaultRouterAPIPort  = 8443
)

// fingerprintLength is a hex SHA-256.
const fingerprintLength = 64

// RouterService stores MikroTik routers and their credentials.
type RouterService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewRouterService creates a router service.
func NewRouterService(db *gorm.DB, encryptionKey string) *RouterService {
	return &RouterService{db: db, encryptionKey: strings.TrimSpace(encryptionKey)}
}

// RouterInput is a new router. TLSFingerprint is the one the connection test
// returned and the operator confirmed.
type RouterInput struct {
	Name           string
	Host           string
	SiteID         *uuid.UUID
	SNMPCommunity  string
	SNMPPort       int
	APIUsername    string
	APIPassword    string
	APIPort        int
	TLSFingerprint string
}

// RouterUpdate changes some of a router's fields. A nil or empty secret keeps
// the stored one, since the form cannot show what it does not know.
type RouterUpdate struct {
	Name          *string
	Host          *string
	SNMPCommunity *string
	APIUsername   *string
	APIPassword   *string
	SNMPPort      *int
	APIPort       *int
	SiteID        *uuid.UUID
	ClearSite     bool
	Enabled       *bool
}

// Create stores a router and makes all its poll tiers due at once, so the
// first readings arrive within a minute rather than after the next sweep.
func (s *RouterService) Create(in RouterInput) (*models.MikrotikRouter, error) {
	if err := validateRouterInput(in); err != nil {
		return nil, err
	}
	community, err := utils.Encrypt(in.SNMPCommunity, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt community: %w", err)
	}
	password, err := utils.Encrypt(in.APIPassword, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}

	router := &models.MikrotikRouter{
		Name: strings.TrimSpace(in.Name), Host: strings.TrimSpace(in.Host), SiteID: in.SiteID,
		SNMPCommunity: community, SNMPPort: portOr(in.SNMPPort, DefaultRouterSNMPPort),
		APIUsername: in.APIUsername, APIPassword: password, APIPort: portOr(in.APIPort, DefaultRouterAPIPort),
		TLSFingerprint: strings.ToLower(in.TLSFingerprint), Enabled: true,
		Status: models.RouterStatusUnknown, APIStatus: models.RouterAPIStatusUnknown,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(router).Error; err != nil {
			return err
		}
		return createRouterJobs(tx, router.ID)
	})
	if err != nil {
		return nil, err
	}
	return router, nil
}

func createRouterJobs(tx *gorm.DB, routerID uuid.UUID) error {
	now := time.Now()
	for _, kind := range AllRouterPollKinds {
		if err := tx.Create(&models.RouterPollJob{RouterID: routerID, Kind: kind, DueAt: now}).Error; err != nil {
			return err
		}
	}
	return nil
}

func validateRouterInput(in RouterInput) error {
	switch {
	case strings.TrimSpace(in.Name) == "":
		return fmt.Errorf("%w: name is required", ErrValidation)
	case strings.TrimSpace(in.Host) == "":
		return fmt.Errorf("%w: host is required", ErrValidation)
	case in.SNMPCommunity == "":
		return fmt.Errorf("%w: SNMP community is required", ErrValidation)
	case in.APIUsername == "" || in.APIPassword == "":
		return fmt.Errorf("%w: API username and password are required", ErrValidation)
	case !validFingerprint(in.TLSFingerprint):
		return fmt.Errorf("%w: tls_fingerprint must be the SHA-256 the connection test returned", ErrValidation)
	}
	return nil
}

func validFingerprint(fingerprint string) bool {
	if len(fingerprint) != fingerprintLength {
		return false
	}
	_, err := hex.DecodeString(fingerprint)
	return err == nil
}

func portOr(port, fallback int) int {
	if port == 0 {
		return fallback
	}
	return port
}

// Update applies the fields that were given.
func (s *RouterService) Update(id uuid.UUID, upd RouterUpdate) (*models.MikrotikRouter, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	updates, err := s.updateColumns(upd)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.db.Model(&models.MikrotikRouter{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

func (s *RouterService) updateColumns(upd RouterUpdate) (map[string]interface{}, error) {
	updates := map[string]interface{}{}
	if upd.Name != nil {
		if strings.TrimSpace(*upd.Name) == "" {
			return nil, fmt.Errorf("%w: name is required", ErrValidation)
		}
		updates["name"] = strings.TrimSpace(*upd.Name)
	}
	if upd.Host != nil {
		if strings.TrimSpace(*upd.Host) == "" {
			return nil, fmt.Errorf("%w: host is required", ErrValidation)
		}
		updates["host"] = strings.TrimSpace(*upd.Host)
	}
	setIfGiven(updates, "api_username", upd.APIUsername)
	setIfGiven(updates, "snmp_port", upd.SNMPPort)
	setIfGiven(updates, "api_port", upd.APIPort)
	setIfGiven(updates, "enabled", upd.Enabled)
	setIfGiven(updates, "site_id", upd.SiteID)
	if upd.ClearSite {
		updates["site_id"] = nil
	}
	return updates, s.encryptGivenSecrets(updates, upd)
}

func setIfGiven[T any](updates map[string]interface{}, column string, value *T) {
	if value != nil {
		updates[column] = *value
	}
}

func (s *RouterService) encryptGivenSecrets(updates map[string]interface{}, upd RouterUpdate) error {
	for column, secret := range map[string]*string{"snmp_community": upd.SNMPCommunity, "api_password": upd.APIPassword} {
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

// Delete removes a router. Its interfaces, sessions, events and jobs go with
// it by foreign key; its metric samples age out by retention.
func (s *RouterService) Delete(id uuid.UUID) error {
	result := s.db.Delete(&models.MikrotikRouter{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Get returns one router.
func (s *RouterService) Get(id uuid.UUID) (*models.MikrotikRouter, error) {
	var router models.MikrotikRouter
	if err := s.db.First(&router, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &router, nil
}

// List returns every router by name.
func (s *RouterService) List() ([]models.MikrotikRouter, error) {
	var routers []models.MikrotikRouter
	return routers, s.db.Order("name").Find(&routers).Error
}
