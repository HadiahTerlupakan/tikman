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

func TestSiteHandler_Create(t *testing.T) {
	handler, _ := SetupSiteHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		reqBody := CreateSiteRequest{
			Name:        "Test Site",
			Location:    "Test Location",
			Description: "Test Description",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response SiteResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Test Site", response.Name)
		assert.Equal(t, "Test Location", response.Location)
		assert.Equal(t, "Test Description", response.Description)
		assert.NotEqual(t, uuid.Nil, response.ID)
	})

	t.Run("invalid request - missing name", func(t *testing.T) {
		reqBody := map[string]string{
			"location": "Test Location",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_REQUEST", response.Code)
	})

	t.Run("invalid request - name too short", func(t *testing.T) {
		reqBody := CreateSiteRequest{
			Name: "A",
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.Create(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestSiteHandler_List(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)

	t.Run("empty list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/sites", nil)

		handler.List(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []SiteResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Empty(t, response)
	})

	t.Run("multiple sites", func(t *testing.T) {
		// Create test sites
		service := services.NewSiteService(db)
		_, err := service.Create("Site 1", "Location 1", "Desc 1")
		require.NoError(t, err)
		_, err = service.Create("Site 2", "Location 2", "Desc 2")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/sites", nil)

		handler.List(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []SiteResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
	})
}

func TestSiteHandler_List_ReportsOLTCount(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)

	service := services.NewSiteService(db)
	withOLTs, err := service.Create("Site With OLTs", "Location 1", "Desc 1")
	require.NoError(t, err)
	empty, err := service.Create("Empty Site", "Location 2", "Desc 2")
	require.NoError(t, err)

	CreateTestOLT(t, db, withOLTs.ID, "olt-1", "192.168.1.1", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)
	CreateTestOLT(t, db, withOLTs.ID, "olt-2", "192.168.1.2", "admin", "pass", 22, 23, 161, "public", models.OLTProtocolSSH)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sites", nil)

	handler.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []SiteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	counts := make(map[uuid.UUID]int, len(response))
	for _, site := range response {
		counts[site.ID] = site.OLTCount
	}
	assert.Equal(t, 2, counts[withOLTs.ID])
	assert.Equal(t, 0, counts[empty.ID])
}

func TestSiteHandler_GetByID(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		service := services.NewSiteService(db)
		site, err := service.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/sites/"+site.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: site.ID.String()}}

		handler.GetByID(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SiteResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, site.ID, response.ID)
		assert.Equal(t, "Test Site", response.Name)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/sites/invalid", nil)
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
		c.Request = httptest.NewRequest(http.MethodGet, "/api/sites/"+id.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: id.String()}}

		handler.GetByID(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "NOT_FOUND", response.Code)
	})
}

func TestSiteHandler_Update(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		service := services.NewSiteService(db)
		site, err := service.Create("Old Name", "Old Location", "Old Desc")
		require.NoError(t, err)

		newName := "New Name"
		reqBody := UpdateSiteRequest{
			Name: &newName,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/sites/"+site.ID.String(), bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: site.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SiteResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "New Name", response.Name)
		assert.Equal(t, "Old Location", response.Location) // Unchanged
	})

	t.Run("invalid UUID", func(t *testing.T) {
		newName := "New Name"
		reqBody := UpdateSiteRequest{
			Name: &newName,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/sites/invalid", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid request - name too short", func(t *testing.T) {
		service := services.NewSiteService(db)
		site, err := service.Create("Test Site", "Test Location", "Test Desc")
		require.NoError(t, err)

		shortName := "A"
		reqBody := UpdateSiteRequest{
			Name: &shortName,
		}
		body, _ := json.Marshal(reqBody)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/sites/"+site.ID.String(), bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Params = gin.Params{{Key: "id", Value: site.ID.String()}}

		handler.Update(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestSiteHandler_Delete(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)

	t.Run("success", func(t *testing.T) {
		service := services.NewSiteService(db)
		site, err := service.Create("To Delete", "Location", "Desc")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/sites/"+site.ID.String(), nil)
		c.Params = gin.Params{{Key: "id", Value: site.ID.String()}}

		handler.Delete(c)

		assert.Equal(t, http.StatusNoContent, w.Code)

		// Verify deletion
		_, err = service.GetByID(site.ID)
		assert.Error(t, err)
	})

	t.Run("invalid UUID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodDelete, "/api/sites/invalid", nil)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}

		handler.Delete(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
