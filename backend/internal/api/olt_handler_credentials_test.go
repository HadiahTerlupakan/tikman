package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// getOLTsAs runs List and GetByID for one role and returns both bodies.
func getOLTsAs(t *testing.T, role models.UserRole) (OLTResponse, OLTResponse) {
	t.Helper()

	handler, siteService, db := SetupOLTHandlerTest(t)
	site, err := siteService.Create("Cariu", "Cariu", "")
	require.NoError(t, err)
	olt := CreateTestOLT(t, db, site.ID, "OLT-1", "10.0.0.1", "admin", "rahasia",
		22, 23, 161, "komunitas-rahasia", models.OLTProtocolSSH)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_role", role) })
	router.GET("/api/v1/olts", handler.List)
	router.GET("/api/v1/olts/:id", handler.GetByID)

	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/olts", nil))
	require.Equal(t, http.StatusOK, listRec.Code)
	var list []OLTResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &list))
	require.Len(t, list, 1)

	oneRec := httptest.NewRecorder()
	router.ServeHTTP(oneRec, httptest.NewRequest(http.MethodGet, "/api/v1/olts/"+olt.ID.String(), nil))
	require.Equal(t, http.StatusOK, oneRec.Code)
	var one OLTResponse
	require.NoError(t, json.Unmarshal(oneRec.Body.Bytes(), &one))

	return list[0], one
}

// The community string is what an SNMP agent authenticates on, so handing it
// to every logged-in account hands the chassis to roles that may not even
// edit the OLT row it belongs to.
func TestOLTResponsesHideTheCommunityFromReadOnlyRoles(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS} {
		t.Run(string(role), func(t *testing.T) {
			fromList, fromGet := getOLTsAs(t, role)

			assert.Empty(t, fromList.SNMPCommunity)
			assert.Empty(t, fromGet.SNMPCommunity)
			// Redacting a credential must not blank the row around it.
			assert.Equal(t, "OLT-1", fromGet.Name)
		})
	}
}

// The edit form and the trap setup panel read it back, and both are reached
// only by the roles that may manage an OLT.
func TestOLTResponsesKeepTheCommunityForOLTManagers(t *testing.T) {
	for _, role := range []models.UserRole{models.UserRoleAdmin, models.UserRoleTechnician} {
		t.Run(string(role), func(t *testing.T) {
			fromList, fromGet := getOLTsAs(t, role)

			assert.Equal(t, "komunitas-rahasia", fromList.SNMPCommunity)
			assert.Equal(t, "komunitas-rahasia", fromGet.SNMPCommunity)
		})
	}
}
