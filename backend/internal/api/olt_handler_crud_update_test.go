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
		c.Set("user_role", models.UserRoleAdmin)

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

// updateOLTAs runs Update for one role with a raw JSON body, so a test can send
// exactly the fields a client would.
func updateOLTAs(t *testing.T, handler *OLTHandler, role models.UserRole, id string, fields map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(fields)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/olts/"+id, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set("user_role", role)

	handler.Update(c)
	return w
}

// The commander sends the stored CLI password, and the pollers the community,
// to whatever address, port and protocol the row names. A technician who could
// move any of them would collect secrets no role can read back from the API.
func TestOLTHandler_Update_TechnicianCannotRepointTheOLT(t *testing.T) {
	repoints := map[string]interface{}{
		"ip_address":         "203.0.113.9",
		"ssh_port":           2222,
		"telnet_port":        2323,
		"snmp_port":          1161,
		"preferred_protocol": "telnet",
	}

	for field, value := range repoints {
		t.Run(field, func(t *testing.T) {
			handler, siteService, db := SetupOLTHandlerTest(t)
			site, err := siteService.Create("Cariu", "Cariu", "")
			require.NoError(t, err)
			olt := CreateTestOLT(t, db, site.ID, "OLT-1", "10.0.0.1", "admin", "rahasia",
				22, 23, 161, "public", models.OLTProtocolSSH)

			w := updateOLTAs(t, handler, models.UserRoleTechnician, olt.ID.String(),
				map[string]interface{}{"name": "Renamed", field: value})

			assert.Equal(t, http.StatusForbidden, w.Code)
			stored, err := services.NewOLTService(db, testEncryptionKey).GetByID(olt.ID)
			require.NoError(t, err)
			assert.Equal(t, "OLT-1", stored.Name)
			assert.Equal(t, "10.0.0.1", stored.IPAddress)
			assert.Equal(t, 22, stored.SSHPort)
			assert.Equal(t, 23, stored.TelnetPort)
			assert.Equal(t, 161, stored.SNMPPort)
			assert.Equal(t, models.OLTProtocolSSH, stored.PreferredProtocol)
		})
	}
}

// The edit form sends every field back, so a technician renaming an OLT or
// placing it on the map must not be refused for resending the stored address.
func TestOLTHandler_Update_TechnicianMayResendTheStoredDestination(t *testing.T) {
	handler, siteService, db := SetupOLTHandlerTest(t)
	site, err := siteService.Create("Cariu", "Cariu", "")
	require.NoError(t, err)
	olt := CreateTestOLT(t, db, site.ID, "OLT-1", "10.0.0.1", "admin", "rahasia",
		22, 23, 161, "public", models.OLTProtocolSSH)

	w := updateOLTAs(t, handler, models.UserRoleTechnician, olt.ID.String(), map[string]interface{}{
		"name": "Renamed", "ip_address": "10.0.0.1", "ssh_port": 22, "telnet_port": 23,
		"snmp_port": 161, "preferred_protocol": "ssh", "latitude": -6.5, "longitude": 107.1,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var response OLTResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "Renamed", response.Name)
	assert.Equal(t, "10.0.0.1", response.IPAddress)
	require.NotNil(t, response.Latitude)
	assert.Equal(t, -6.5, *response.Latitude)
}
