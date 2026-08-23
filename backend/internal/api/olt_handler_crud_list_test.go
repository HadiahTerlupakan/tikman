package api

import (
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
