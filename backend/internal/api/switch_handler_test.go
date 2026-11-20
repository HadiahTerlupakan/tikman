package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

func setupSwitchHandlerTest(t *testing.T) *SwitchHandler {
	t.Helper()
	return NewSwitchHandler(services.NewSwitchService(TestDB(t), testEncryptionKey), nil)
}

func createSwitchRequest() CreateSwitchRequest {
	return CreateSwitchRequest{Name: "SW-TEST", Host: "192.0.2.10", Vendor: "huawei_ce",
		SNMPCommunity: "c-secret", Protocol: "telnet", Username: "tikman", Password: "p-secret"}
}

func TestCreateSwitchNeverEchoesItsSecrets(t *testing.T) {
	handler := setupSwitchHandlerTest(t)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/switches", createSwitchRequest())

	handler.Create(c)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "p-secret")
	assert.NotContains(t, w.Body.String(), "c-secret")
	assert.NotContains(t, w.Body.String(), "password")
	assert.NotContains(t, w.Body.String(), "snmp_community")
	assert.Contains(t, w.Body.String(), `"port":23`)
}

func TestCreateSwitchRefusesAVendorWithoutADriver(t *testing.T) {
	handler := setupSwitchHandlerTest(t)
	req := createSwitchRequest()
	req.Vendor = "crs"
	w, c := SetupTestContext(http.MethodPost, "/api/v1/switches", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "belum didukung")
}

func TestCreateSwitchRefusesAnUnknownProtocol(t *testing.T) {
	handler := setupSwitchHandlerTest(t)
	req := createSwitchRequest()
	req.Protocol = "ftp"
	w, c := SetupTestContext(http.MethodPost, "/api/v1/switches", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListUpdateAndDeleteSwitch(t *testing.T) {
	handler := setupSwitchHandlerTest(t)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/switches", createSwitchRequest())
	handler.Create(c)
	var created models.Switch
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	w, c = SetupTestContext(http.MethodGet, "/api/v1/switches", nil)
	handler.List(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SW-TEST")

	name := "SW-RENAMED"
	w, c = SetupTestContext(http.MethodPut, "/api/v1/switches/"+created.ID.String(), UpdateSwitchRequest{Name: &name})
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}
	handler.Update(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "SW-RENAMED")

	_, c = SetupTestContext(http.MethodDelete, "/api/v1/switches/"+created.ID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}
	handler.Delete(c)
	// Gin holds a bodyless status back until the response is flushed.
	assert.Equal(t, http.StatusNoContent, c.Writer.Status())
}

func TestGetAndDeleteSwitchAnswer404ForAMissingOne(t *testing.T) {
	handler := setupSwitchHandlerTest(t)
	missing := uuid.New()

	w, c := SetupTestContext(http.MethodGet, "/api/v1/switches/"+missing.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: missing.String()}}
	handler.Get(c)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w, c = SetupTestContext(http.MethodDelete, "/api/v1/switches/"+missing.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: missing.String()}}
	handler.Delete(c)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSwitchRoutesFollowTheRoles(t *testing.T) {
	router, store := rbacTestRouter(t)
	id := "/api/v1/switches/" + uuid.New().String()

	assert.Equal(t, http.StatusOK, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, "/api/v1/switches"))
	assert.Equal(t, http.StatusNotFound, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, id))
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS, models.UserRoleTechnician} {
		assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodPost, "/api/v1/switches"), role)
		assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodPut, id), role)
		assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, role, http.MethodDelete, id), role)
	}
}

func TestAdminCreatesASwitchThroughTheRouteTree(t *testing.T) {
	router, store := rbacTestRouter(t)
	body, err := json.Marshal(createSwitchRequest())
	require.NoError(t, err)

	token, err := store.Create(uuid.New(), models.UserRoleAdmin)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/switches", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "password")
}
