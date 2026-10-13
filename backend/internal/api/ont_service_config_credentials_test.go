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
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// getServiceConfigAs reads one ONT's service config as the given role and
// returns the "data" object of the response.
func getServiceConfigAs(t *testing.T, role models.UserRole) map[string]any {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ONT{}))

	encoded, err := json.Marshal(connectivity.ZTEONUService{VLANID: 214, PPPoEUsername: "100200300400"})
	require.NoError(t, err)
	sealed, err := utils.Encrypt("pppoe-rahasia", testEncryptionKey)
	require.NoError(t, err)
	ont := &models.ONT{
		ID: uuid.New(), OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "HWTCB403E8A0", ServiceConfig: datatypes.JSON(encoded), PPPoEPassword: sealed,
	}
	require.NoError(t, db.Create(ont).Error)

	handler := NewONTHandler(services.NewONTServiceWithEncryption(db, testEncryptionKey), nil, nil, nil)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_role", role) })
	router.GET("/api/v1/onts/:id/service-config", handler.GetServiceConfig)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/onts/"+ont.ID.String()+"/service-config", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Data)
	return body.Data
}

// The PPPoE password logs anyone in to the BRAS as the subscriber, and the
// read-only roles cannot open the configure form that is its only reader.
func TestServiceConfigHidesThePPPoEPasswordFromReadOnlyRoles(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS} {
		t.Run(string(role), func(t *testing.T) {
			data := getServiceConfigAs(t, role)

			assert.Empty(t, data["pppoe_password"])
			// Redacting the credential must not blank the service around it.
			assert.Equal(t, float64(214), data["vlan_id"])
			assert.Equal(t, "100200300400", data["pppoe_username"])
		})
	}
}

// Reconfiguring has to resend the password, so the roles that may do it still
// get it back.
func TestServiceConfigKeepsThePPPoEPasswordForConfigurers(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleAdmin, models.UserRoleTechnician} {
		t.Run(string(role), func(t *testing.T) {
			data := getServiceConfigAs(t, role)

			assert.Equal(t, "pppoe-rahasia", data["pppoe_password"])
		})
	}
}
