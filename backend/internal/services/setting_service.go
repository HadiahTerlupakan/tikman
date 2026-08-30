package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrUnknownSetting marks a name that is not in the registry. The store is not
// an arbitrary key-value bag.
var ErrUnknownSetting = errors.New("unknown setting")

// ErrSettingNotConfigured means the setting exists but has no value yet, which
// callers handle by degrading rather than failing.
var ErrSettingNotConfigured = errors.New("setting not configured")

// previewDots is fixed so the mask cannot leak the value's length.
const previewDots = "••••••••"

// previewEdge is how much of a long value stays readable, enough to tell two
// keys apart without revealing either.
const previewEdge = 4

// SettingStatus describes a setting for the management page. It deliberately
// carries no value.
type SettingStatus struct {
	Name        string     `json:"name"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Configured  bool       `json:"configured"`
	Preview     string     `json:"preview"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// SettingService stores credentials for external integrations.
type SettingService struct {
	db            *gorm.DB
	encryptionKey string
}

// NewSettingService creates a setting service.
func NewSettingService(db *gorm.DB, encryptionKey string) *SettingService {
	return &SettingService{db: db, encryptionKey: encryptionKey}
}

// List returns every known setting, configured or not, so the page can offer
// one that has never been set.
func (s *SettingService) List() ([]SettingStatus, error) {
	stored, err := s.storedByName()
	if err != nil {
		return nil, err
	}

	definitions := models.SettingDefinitions()
	statuses := make([]SettingStatus, 0, len(definitions))
	for _, definition := range definitions {
		status := SettingStatus{
			Name:        definition.Name,
			Label:       definition.Label,
			Description: definition.Description,
		}
		if row, ok := stored[definition.Name]; ok {
			plaintext, decryptErr := utils.Decrypt(row.Value, s.encryptionKey)
			if decryptErr != nil {
				return nil, fmt.Errorf("failed to read setting %s: %w", definition.Name, decryptErr)
			}
			updatedAt := row.UpdatedAt
			status.Configured = true
			status.Preview = maskSecret(plaintext)
			status.UpdatedAt = &updatedAt
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// Set stores a value for a known setting, replacing any previous one.
func (s *SettingService) Set(name, value string, actor uuid.UUID) error {
	if _, ok := models.LookupSetting(name); !ok {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		// An empty value would show as configured while the feature stays
		// broken, which is worse than not being set at all.
		return fmt.Errorf("%w: a value is required", ErrValidation)
	}

	ciphertext, err := utils.Encrypt(trimmed, s.encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt setting: %w", err)
	}

	setting := models.AppSetting{
		Name:      name,
		Value:     ciphertext,
		UpdatedAt: time.Now(),
		UpdatedBy: &actor,
	}

	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
	}).Create(&setting).Error
}

// Delete removes a stored value. Deleting one that was never set is not an
// error: the caller's intent is already satisfied.
func (s *SettingService) Delete(name string) error {
	if _, ok := models.LookupSetting(name); !ok {
		return fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}
	return s.db.Where("name = ?", name).Delete(&models.AppSetting{}).Error
}

// Value decrypts a setting for backend use.
func (s *SettingService) Value(name string) (string, error) {
	if _, ok := models.LookupSetting(name); !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownSetting, name)
	}

	var setting models.AppSetting
	err := s.db.First(&setting, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("%w: %s", ErrSettingNotConfigured, name)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read setting: %w", err)
	}

	return utils.Decrypt(setting.Value, s.encryptionKey)
}

// BrowserValues returns the settings whose features can only run client-side.
// It is the only path by which a stored value leaves this server, so it walks
// the registry rather than the table: a row whose name is not a browser
// definition is never read, however it got there.
func (s *SettingService) BrowserValues() (map[string]string, error) {
	stored, err := s.storedByName()
	if err != nil {
		return nil, err
	}

	values := map[string]string{}
	for _, definition := range models.SettingDefinitions() {
		if definition.Visibility != models.VisibilityBrowser {
			continue
		}
		row, ok := stored[definition.Name]
		if !ok {
			continue
		}
		plaintext, decryptErr := utils.Decrypt(row.Value, s.encryptionKey)
		if decryptErr != nil {
			return nil, fmt.Errorf("failed to read setting %s: %w", definition.Name, decryptErr)
		}
		values[definition.Name] = plaintext
	}
	return values, nil
}

func (s *SettingService) storedByName() (map[string]models.AppSetting, error) {
	var rows []models.AppSetting
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	byName := make(map[string]models.AppSetting, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	return byName, nil
}

// maskSecret keeps the ends of a long value readable so two keys can be told
// apart, and hides everything about a short one.
func maskSecret(value string) string {
	if len(value) < previewEdge*2+4 {
		return previewDots
	}
	return value[:previewEdge] + previewDots + value[len(value)-previewEdge:]
}
