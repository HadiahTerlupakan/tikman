package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const settingServiceTestEncryptionKey = "01234567890123456789012345678901"

func newSettingService(t *testing.T) (*SettingService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}))

	return NewSettingService(db, settingServiceTestEncryptionKey), db
}

func TestSettingIsStoredEncryptedAndReadsBack(t *testing.T) {
	service, db := newSettingService(t)

	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	var stored models.AppSetting
	require.NoError(t, db.First(&stored, "name = ?", models.SettingGoogleMapsAPIKey).Error)
	require.NotContains(t, stored.Value, "AIzaSyTESTKEY123", "the key must not sit in the table as plaintext")

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "AIzaSyTESTKEY123", value)
}

func TestSettingRejectsANameNobodyDeclared(t *testing.T) {
	service, _ := newSettingService(t)

	err := service.Set("secret_backdoor", "value", uuid.New())
	require.ErrorIs(t, err, ErrUnknownSetting)
}

func TestSettingRejectsABlankValue(t *testing.T) {
	// Storing "" would read as configured on the settings page while the
	// feature stays broken.
	service, _ := newSettingService(t)

	require.Error(t, service.Set(models.SettingGoogleMapsAPIKey, "   ", uuid.New()))
}

func TestSettingReportsNotConfiguredDistinctlyFromAFailure(t *testing.T) {
	service, _ := newSettingService(t)

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, ErrSettingNotConfigured)
}

func TestSettingReplacesRatherThanDuplicates(t *testing.T) {
	service, db := newSettingService(t)
	actor := uuid.New()

	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "first-value", actor))
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "second-value", actor))

	var count int64
	require.NoError(t, db.Model(&models.AppSetting{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "second-value", value)
}

func TestSettingDeleteRemovesTheValue(t *testing.T) {
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	require.NoError(t, service.Delete(models.SettingGoogleMapsAPIKey))

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, ErrSettingNotConfigured)
}

func TestSettingListDescribesEveryKnownSettingEvenUnset(t *testing.T) {
	// The page has to offer a setting before it is configured, or there is no
	// way to configure it.
	service, _ := newSettingService(t)

	statuses, err := service.List()
	require.NoError(t, err)
	require.Len(t, statuses, len(models.SettingDefinitions()))
	require.False(t, statuses[0].Configured)
	require.Empty(t, statuses[0].Preview)
	require.NotEmpty(t, statuses[0].Label)
}

func TestSettingListNeverCarriesTheValue(t *testing.T) {
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	statuses, err := service.List()
	require.NoError(t, err)
	require.True(t, statuses[0].Configured)
	require.NotContains(t, statuses[0].Preview, "SyTESTKEY")
	require.Equal(t, "AIza••••••••Y123", statuses[0].Preview)
	require.NotNil(t, statuses[0].UpdatedAt)
}

func TestSettingPreviewHidesTheLengthOfAShortValue(t *testing.T) {
	// A dot count that tracks the value's length leaks how long the secret is.
	service, _ := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "short", uuid.New()))

	statuses, err := service.List()
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("•", 8), statuses[0].Preview)
}

func TestBrowserValuesCarryOnlyWhatMustRunClientSide(t *testing.T) {
	service, db := newSettingService(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	// A stored row whose name is not a browser definition must not escape, even
	// though it sits in the same table.
	ciphertext, err := encryptForTest("server-secret")
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.AppSetting{Name: "server_only_probe", Value: ciphertext}).Error)

	values, err := service.BrowserValues()
	require.NoError(t, err)
	require.Equal(t, map[string]string{models.SettingGoogleMapsAPIKey: "AIzaSyTESTKEY123"}, values)
}

func TestBrowserValuesAreEmptyBeforeAnythingIsConfigured(t *testing.T) {
	service, _ := newSettingService(t)

	values, err := service.BrowserValues()
	require.NoError(t, err)
	require.Empty(t, values)
}

func TestValueRejectsAnUnknownName(t *testing.T) {
	service, _ := newSettingService(t)

	_, err := service.Value("server_only_probe")
	require.True(t, errors.Is(err, ErrUnknownSetting))
}

func encryptForTest(plaintext string) (string, error) {
	return utils.Encrypt(plaintext, settingServiceTestEncryptionKey)
}
