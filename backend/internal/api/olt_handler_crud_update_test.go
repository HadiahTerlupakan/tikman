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
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

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
