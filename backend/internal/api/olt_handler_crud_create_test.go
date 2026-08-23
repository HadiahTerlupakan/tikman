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
