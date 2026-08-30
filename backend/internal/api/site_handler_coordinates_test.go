package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/services"
)

func updateSite(t *testing.T, handler *SiteHandler, id string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/sites/"+id, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}

	handler.Update(c)
	return w
}

func TestSiteHandler_Update_ClearCoordinates(t *testing.T) {
	handler, db := SetupSiteHandlerTest(t)
	service := services.NewSiteService(db)

	latitude, longitude := -6.4025, 106.7942

	t.Run("clear_coordinates removes a wrongly placed pin", func(t *testing.T) {
		site, err := service.CreateWithCoordinates("Gudang", "Jl. Margonda", "", &latitude, &longitude)
		require.NoError(t, err)

		body, err := json.Marshal(UpdateSiteRequest{ClearCoordinates: true})
		require.NoError(t, err)

		w := updateSite(t, handler, site.ID.String(), body)
		require.Equal(t, http.StatusOK, w.Code)

		var response SiteResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Nil(t, response.Latitude)
		assert.Nil(t, response.Longitude)

		stored, err := service.GetByID(site.ID)
		require.NoError(t, err)
		assert.Nil(t, stored.Latitude)
		assert.Nil(t, stored.Longitude)
	})

	t.Run("omitting the coordinates leaves the pin where it was", func(t *testing.T) {
		site, err := service.CreateWithCoordinates("Kantor", "Jl. Juanda", "", &latitude, &longitude)
		require.NoError(t, err)

		name := "Kantor Pusat"
		body, err := json.Marshal(UpdateSiteRequest{Name: &name})
		require.NoError(t, err)

		w := updateSite(t, handler, site.ID.String(), body)
		require.Equal(t, http.StatusOK, w.Code)

		stored, err := service.GetByID(site.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.Latitude)
		require.NotNil(t, stored.Longitude)
		assert.InDelta(t, latitude, *stored.Latitude, 1e-9)
		assert.InDelta(t, longitude, *stored.Longitude, 1e-9)
	})
}
