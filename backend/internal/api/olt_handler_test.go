package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "12345678901234567890123456789012" // 32 bytes

func setupOLTHandlerTest(t *testing.T) (*OLTHandler, *services.SiteService, *gorm.DB) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = models.AutoMigrate(db)
	require.NoError(t, err)

	siteService := services.NewSiteService(db)
	oltService := services.NewOLTService(db, testEncryptionKey)
	auditService := services.NewAuditService(db, nil)
	handler := NewOLTHandler(oltService, auditService)

	return handler, siteService, db
}

func TestOLTHandler_Create(t *testing.T) {
	handler, siteService, _ := setupOLTHandlerTest(t)

	t.Run("success with default ports", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		reqBody := CreateOLTRequest{
			SiteID:            site.ID,
			Name:              "Test OLT",
			IPAddress:         "192.168.1.1",
			PreferredProtocol: models.OLTProtocolSSH,
			Username:          "admin",
			Password:          "password123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/olts", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Test OLT", response.Name)
		assert.Equal(t, "192.168.1.1", response.IPAddress)
		assert.Equal(t, "admin", response.Username)
		assert.Equal(t, 22, response.SSHPort)
		assert.Equal(t, 23, response.TelnetPort)
		assert.Equal(t, 161, response.SNMPPort)
		assert.Equal(t, "public", response.SNMPCommunity)
		assert.Equal(t, models.OLTProtocolSSH, response.PreferredProtocol)
		assert.Equal(t, models.OLTStatusOffline, response.Status)
		assert.NotEqual(t, uuid.Nil, response.ID)
	})

	t.Run("success with custom ports", func(t *testing.T) {
		site, err := siteService.Create("Test Site 2", "Test Location", "Test Desc")
		require.NoError(t, err)

		reqBody := CreateOLTRequest{
			SiteID:            site.ID,
			Name:              "Test OLT Custom",
			IPAddress:         "192.168.1.2",
			SSHPort:           2222,
			TelnetPort:        2323,
			SNMPPort:          1161,
			SNMPCommunity:     "private",
			PreferredProtocol: models.OLTProtocolTelnet,
			Username:          "admin",
			Password:          "password123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/olts", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, 2222, response.SSHPort)
		assert.Equal(t, 2323, response.TelnetPort)
		assert.Equal(t, 1161, response.SNMPPort)
		assert.Equal(t, "private", response.SNMPCommunity)
		assert.Equal(t, models.OLTProtocolTelnet, response.PreferredProtocol)
	})

	t.Run("invalid request - missing required fields", func(t *testing.T) {
		reqBody := map[string]string{
			"name": "Test OLT",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/olts", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_REQUEST", response.Code)
	})

	t.Run("invalid request - invalid IP address", func(t *testing.T) {
		site, err := siteService.Create("Test Site 3", "Test Location", "Test Desc")
		require.NoError(t, err)

		reqBody := CreateOLTRequest{
			SiteID:            site.ID,
			Name:              "Test OLT",
			IPAddress:         "invalid-ip",
			PreferredProtocol: models.OLTProtocolSSH,
			Username:          "admin",
			Password:          "password123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/olts", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid request - invalid site ID", func(t *testing.T) {
		invalidSiteID := uuid.New()

		reqBody := CreateOLTRequest{
			SiteID:            invalidSiteID,
			Name:              "Test OLT",
			IPAddress:         "192.168.1.1",
			PreferredProtocol: models.OLTProtocolSSH,
			Username:          "admin",
			Password:          "password123",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/olts", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SITE_ID", response.Code)
	})
}

func TestOLTHandler_List(t *testing.T) {
	handler, siteService, db := setupOLTHandlerTest(t)

	t.Run("empty list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/olts", nil)

		handler.List(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []OLTResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Empty(t, response)
	})

	t.Run("multiple olts", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		_, err = oltService.Create(site.ID, "OLT 1", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)
		_, err = oltService.Create(site.ID, "OLT 2", "192.168.1.2", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolTelnet)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/olts", nil)

		handler.List(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
	})
}

func TestOLTHandler_GetByID(t *testing.T) {
	handler, siteService, db := setupOLTHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		olt, err := oltService.Create(site.ID, "Test OLT", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/olts/"+olt.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.GetByID(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, olt.ID, response.ID)
		assert.Equal(t, "Test OLT", response.Name)
		assert.Equal(t, "Test Site", response.SiteName)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/olts/invalid", nil)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.GetByID(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_ID", response.Code)
	})

	t.Run("not found", func(t *testing.T) {
		id := uuid.New()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/olts/"+id.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: id.String()}}

		handler.GetByID(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "NOT_FOUND", response.Code)
	})
}

func TestOLTHandler_Update(t *testing.T) {
	handler, siteService, db := setupOLTHandlerTest(t)

	t.Run("success - update name", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		olt, err := oltService.Create(site.ID, "Old Name", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)

		newName := "New Name"
		reqBody := UpdateOLTRequest{
			Name: &newName,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/olts/"+olt.ID.String(), bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "New Name", response.Name)
		assert.Equal(t, "192.168.1.1", response.IPAddress) // Unchanged
	})

	t.Run("success - update multiple fields", func(t *testing.T) {
		site, err := siteService.Create("Test Site 2", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		olt, err := oltService.Create(site.ID, "Test OLT", "192.168.1.1", "admin", "oldpass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)

		newIP := "192.168.1.100"
		newPort := 2222
		newPassword := "newpass123"
		reqBody := UpdateOLTRequest{
			IPAddress: &newIP,
			SSHPort:   &newPort,
			Password:  &newPassword,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/olts/"+olt.ID.String(), bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response OLTResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.100", response.IPAddress)
		assert.Equal(t, 2222, response.SSHPort)

		// Verify password was encrypted
		updatedOlt, _ := oltService.GetByID(olt.ID)
		decrypted, err := oltService.DecryptPassword(updatedOlt.Password)
		require.NoError(t, err)
		assert.Equal(t, "newpass123", decrypted)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		newName := "New Name"
		reqBody := UpdateOLTRequest{
			Name: &newName,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/olts/invalid", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid request - invalid IP address", func(t *testing.T) {
		site, err := siteService.Create("Test Site 3", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		olt, err := oltService.Create(site.ID, "Test OLT", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)

		invalidIP := "not-an-ip"
		reqBody := UpdateOLTRequest{
			IPAddress: &invalidIP,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/olts/"+olt.ID.String(), bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestOLTHandler_Delete(t *testing.T) {
	handler, siteService, db := setupOLTHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		oltService := services.NewOLTService(db, testEncryptionKey)
		olt, err := oltService.Create(site.ID, "To Delete", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/olts/"+olt.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Delete(c)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify deletion
		_, err = oltService.GetByID(olt.ID)
		assert.Error(t, err)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/olts/invalid", nil)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.Delete(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
