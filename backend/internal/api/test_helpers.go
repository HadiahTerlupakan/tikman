package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "12345678901234567890123456789012" // 32 bytes

// TestDB creates an in-memory SQLite database for testing
func TestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = models.AutoMigrate(db)
	require.NoError(t, err)
	return db
}

// SetupTestContext creates a test Gin context with the given request body
func SetupTestContext(method, path string, body interface{}) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		c.Request = httptest.NewRequest(method, path, bytes.NewReader(jsonBody))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, path, nil)
	}

	return w, c
}

// SetupOLTHandlerTest creates OLT handler with test dependencies
func SetupOLTHandlerTest(t *testing.T) (*OLTHandler, *services.SiteService, *gorm.DB) {
	db := TestDB(t)
	siteService := services.NewSiteService(db)
	oltService := services.NewOLTService(db, testEncryptionKey)
	handler := NewOLTHandler(oltService, nil)
	return handler, siteService, db
}

// SetupSiteHandlerTest creates Site handler with test dependencies
func SetupSiteHandlerTest(t *testing.T) (*SiteHandler, *gorm.DB) {
	db := TestDB(t)
	service := services.NewSiteService(db)
	handler := NewSiteHandler(service, nil) // nil audit service for tests
	return handler, db
}

// SetupUserHandlerTest creates User handler with test dependencies
func SetupUserHandlerTest(t *testing.T) (*UserHandler, *gorm.DB) {
	db := TestDB(t)
	service := services.NewUserService(db)
	handler := NewUserHandler(service, nil) // nil audit service for tests
	return handler, db
}

// SetupAuthHandlerTest creates Auth handler with test dependencies
func SetupAuthHandlerTest(t *testing.T) (*AuthHandler, *services.UserService, *gorm.DB) {
	db := TestDB(t)
	userService := services.NewUserService(db)
	handler := NewAuthHandler(userService, nil)
	return handler, userService, db
}

// CreateTestOLT creates an OLT directly in the database, bypassing validation.
// Use this for tests that need existing OLTs but don't test validation logic.
func CreateTestOLT(t *testing.T, db *gorm.DB, siteID uuid.UUID, name, ipAddress, username, password string,
	sshPort, telnetPort, snmpPort int, snmpCommunity string, protocol models.OLTProtocol) *models.OLT {

	encryptedPassword, err := utils.Encrypt(password, testEncryptionKey)
	require.NoError(t, err)

	olt := &models.OLT{
		ID:                uuid.New(),
		SiteID:            siteID,
		Name:              name,
		IPAddress:         ipAddress,
		Username:          username,
		Password:          encryptedPassword,
		SSHPort:           sshPort,
		TelnetPort:        telnetPort,
		SNMPPort:          snmpPort,
		SNMPCommunity:     snmpCommunity,
		PreferredProtocol: protocol,
		Status:            models.OLTStatusOffline,
	}

	err = db.Create(olt).Error
	require.NoError(t, err)

	return olt
}
