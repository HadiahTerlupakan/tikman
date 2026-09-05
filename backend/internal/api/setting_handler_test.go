package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/auth"
	"github.com/tikman/olt-provisioning/internal/config"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testEncryptionKey already exists in internal/api/test_helpers.go; do not
// redeclare it.
func setupSettingHandler(t *testing.T) (*SettingHandler, *services.SettingService) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}, &models.AuditLog{}))

	service := services.NewSettingService(db, testEncryptionKey)
	return NewSettingHandler(service, nil), service
}

func settingContext(t *testing.T, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uuid.New())
	c.Set("user_role", models.UserRoleAdmin)
	return c, recorder
}

func TestSettingListNeverReturnsAValue(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings", "")
	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "AIzaSyTESTKEY123")
	require.Contains(t, recorder.Body.String(), "AIza")
}

func TestSettingSetStoresAndReportsStatus(t *testing.T) {
	handler, service := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"AIzaSyTESTKEY123"}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	value, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.NoError(t, err)
	require.Equal(t, "AIzaSyTESTKEY123", value)
}

func TestSettingSetRefusesAnUnknownName(t *testing.T) {
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/backdoor", `{"value":"x"}`)
	c.Params = gin.Params{{Key: "name", Value: "backdoor"}}
	handler.Set(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "UNKNOWN_SETTING")
}

func TestSettingSetRefusesABlankValue(t *testing.T) {
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"   "}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestSettingBrowserEndpointCarriesOnlyBrowserSettings(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings/browser", "")
	handler.Browser(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"values":[{"name":"`+models.SettingGoogleMapsAPIKey+`","value":"AIzaSyTESTKEY123"}]}`, recorder.Body.String())
}

func TestSettingBrowserEndpointAnswersEmptyBeforeAnythingIsSet(t *testing.T) {
	// "No key configured" is a normal state the frontend renders, not an error.
	handler, _ := setupSettingHandler(t)

	c, recorder := settingContext(t, http.MethodGet, "/api/v1/settings/browser", "")
	handler.Browser(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"values":[]}`, recorder.Body.String())
}

func TestSettingDeleteRemovesTheValue(t *testing.T) {
	handler, service := setupSettingHandler(t)
	require.NoError(t, service.Set(models.SettingGoogleMapsAPIKey, "AIzaSyTESTKEY123", uuid.New()))

	c, recorder := settingContext(t, http.MethodDelete, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, "")
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Delete(c)

	require.Equal(t, http.StatusNoContent, recorder.Code)

	_, err := service.Value(models.SettingGoogleMapsAPIKey)
	require.ErrorIs(t, err, services.ErrSettingNotConfigured)
}

func TestSettingAuditRecordsTheChangeButNotTheValue(t *testing.T) {
	// An audit trail holding the credential defeats the encryption it audits.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.AppSetting{}, &models.AuditLog{}))

	service := services.NewSettingService(db, testEncryptionKey)
	handler := NewSettingHandler(service, services.NewAuditService(db, zap.NewNop()))

	c, recorder := settingContext(t, http.MethodPut, "/api/v1/settings/"+models.SettingGoogleMapsAPIKey, `{"value":"AIzaSyTESTKEY123"}`)
	c.Params = gin.Params{{Key: "name", Value: models.SettingGoogleMapsAPIKey}}
	handler.Set(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var logs []models.AuditLog
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, "setting", logs[0].ResourceType)
	require.NotContains(t, string(logs[0].NewValue), "AIzaSyTESTKEY123")
	require.Contains(t, string(logs[0].NewValue), models.SettingGoogleMapsAPIKey)
}

func TestSettingsAreClosedToNonAdminsButBrowserValuesAreNot(t *testing.T) {
	// Calling a handler directly skips the middleware, so it proves nothing
	// about roles. This goes through the real router.
	logger, _ := zap.NewDevelopment()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	cfg := &config.Config{
		LogLevel:       "debug",
		EncryptionKey:  testEncryptionKey,
		Environment:    "development",
		AllowedOrigins: "http://localhost:3000",
	}

	sessionStore := auth.NewMemoryStore(24 * time.Hour)
	token, err := sessionStore.Create(uuid.New(), models.UserRoleViewer)
	require.NoError(t, err)

	router, _, _, _ := Setup(gin.New(), cfg, db, sessionStore, logger,
		services.NewWireGuardService(db, testEncryptionKey, &connectivity.MemoryTunnelDevice{}), nil)

	call := func(path string) int {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: "session_token", Value: token})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}

	// A viewer must not be able to enumerate stored credentials...
	require.Equal(t, http.StatusForbidden, call("/api/v1/settings"))
	// ...but must receive the values their own browser needs to draw a map.
	require.Equal(t, http.StatusOK, call("/api/v1/settings/browser"))
}
