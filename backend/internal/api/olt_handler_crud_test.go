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
)

func TestOLTHandler_Create(t *testing.T) {
	handler, siteService, _ := SetupOLTHandlerTest(t)

	// NOTE: These tests now include validation integration.
	// Attempts to create OLTs with unreachable IPs will fail at validation stage.
	// This is EXPECTED behavior - validation is working correctly.

	t.Run("validation failure - unreachable IP", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		reqBody := CreateOLTRequest{
			SiteID:            site.ID,
			Name:              "Test OLT",
			IPAddress:         "192.168.1.1",
			Model:             models.OLTModelZTEC300,
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

		// An unreachable OLT is bad input, not a server fault, and the reason has
		// to reach the operator: this used to answer 500 "Failed to create OLT",
		// which gave no hint that SNMP was what failed.
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "SNMP_TEST_FAILED", response.Code)
		assert.Contains(t, response.Error, "SNMP connection test failed")
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
			Model:             models.OLTModelZTEC300,
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
			Model:             models.OLTModelZTEC300,
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

		// The site check now runs before the SNMP probe, so this finally fails for
		// the reason the test is named after. Previously it never reached the site
		// lookup - and no site lookup existed.
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_SITE_ID", response.Code)
	})
}

func TestOLTHandler_List(t *testing.T) {
	handler, siteService, db := SetupOLTHandlerTest(t)

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

		// Create OLTs directly in DB, bypassing validation
		CreateTestOLT(t, db, site.ID, "OLT 1", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
		CreateTestOLT(t, db, site.ID, "OLT 2", "192.168.1.2", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolTelnet)

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
	handler, siteService, db := SetupOLTHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		// Create OLT directly in DB, bypassing validation
		olt := CreateTestOLT(t, db, site.ID, "Test OLT", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

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
		assert.Equal(t, site.ID, response.SiteID)
		// Note: SiteName is empty because we removed foreign key relationships
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
	handler, siteService, db := SetupOLTHandlerTest(t)

	t.Run("success - update name", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		olt := CreateTestOLT(t, db, site.ID, "Old Name", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

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

		olt := CreateTestOLT(t, db, site.ID, "Test OLT", "192.168.1.1", "admin", "oldpass", 22, 23, 161, "public", models.OLTProtocolSSH)

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

		oltService := services.NewOLTService(db, testEncryptionKey)
		updatedOlt, _ := oltService.GetByID(olt.ID)
		assert.NotEmpty(t, updatedOlt.Password)
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

		olt := CreateTestOLT(t, db, site.ID, "Test OLT", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

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
	handler, siteService, db := SetupOLTHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		site, err := siteService.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		olt := CreateTestOLT(t, db, site.ID, "To Delete", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/olts/"+olt.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: olt.ID.String()}}

		handler.Delete(c)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify deletion
		oltService := services.NewOLTService(db, testEncryptionKey)
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
