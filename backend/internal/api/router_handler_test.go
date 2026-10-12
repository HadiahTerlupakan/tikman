package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

var routerTestFingerprint = strings.Repeat("ab", 32)

func setupRouterHandlerTest(t *testing.T) (*RouterHandler, *gorm.DB) {
	t.Helper()
	db := TestDB(t)
	handler := NewRouterHandler(services.NewRouterService(db, testEncryptionKey),
		services.NewRouterPollJobService(db), services.NewRouterQueryService(db), nil)
	return handler, db
}

func createRouterRequest() CreateRouterRequest {
	return CreateRouterRequest{Name: "Border", Host: "192.0.2.1", SNMPCommunity: "c-secret",
		APIUsername: "tikman", APIPassword: "p-secret", TLSFingerprint: routerTestFingerprint}
}

func createRouterVia(t *testing.T, handler *RouterHandler) RouterResponse {
	t.Helper()
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", createRouterRequest())
	handler.Create(c)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var out RouterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestCreateRouterNeverEchoesItsSecrets(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", createRouterRequest())

	handler.Create(c)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.NotContains(t, w.Body.String(), "c-secret")
	assert.NotContains(t, w.Body.String(), "p-secret")
	assert.NotContains(t, w.Body.String(), "snmp_community")
	assert.Contains(t, w.Body.String(), `"api_port":8443`)
}

func TestCreateRouterRejectsAFingerprintThatIsNotOne(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	req := createRouterRequest()
	req.TLSFingerprint = "not-a-fingerprint"
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTestRouterConnectionBindsItsBody(t *testing.T) {
	full := TestRouterConnectionRequest{Host: "192.0.2.1", SNMPCommunity: "c", APIUsername: "u", APIPassword: "p"}
	badFingerprint := full
	badFingerprint.TLSFingerprint = "xyz"
	cases := map[string]interface{}{
		"no api fields":       map[string]string{"host": "192.0.2.1", "snmp_community": "c"},
		"invalid fingerprint": badFingerprint,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			handler, _ := setupRouterHandlerTest(t)
			w, c := SetupTestContext(http.MethodPost, "/api/v1/routers/test-connection", body)

			handler.TestConnection(c)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestListRoutersCarriesTheBGPTally(t *testing.T) {
	handler, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	for _, s := range []models.RouterBGPSession{
		{RouterID: created.ID, Name: "a", State: models.BGPStateEstablished},
		{RouterID: created.ID, Name: "b", State: models.BGPStateDown},
	} {
		require.NoError(t, db.Create(&s).Error)
	}
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers", nil)

	handler.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	var out []RouterResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
	assert.Equal(t, 1, out[0].BGPEstablished)
	assert.Equal(t, 2, out[0].BGPTotal)
}

func TestDeleteUnknownRouterIs404(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	id := uuid.New().String()
	w, c := SetupTestContext(http.MethodDelete, "/api/v1/routers/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	handler.Delete(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTrustCertificateRefusesAFingerprintTheRouterNeverPresented(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers/"+created.ID.String()+"/trust-certificate",
		TrustCertificateRequest{Fingerprint: strings.Repeat("cd", 32)})
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

	handler.TrustCertificate(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestTrafficRejectsABackwardsRange(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	path := "/api/v1/routers/" + created.ID.String() + "/interfaces/" + uuid.NewString() +
		"/traffic?from=2026-10-12T10:00:00Z&to=2026-10-12T09:00:00Z"
	w, c := SetupTestContext(http.MethodGet, path, nil)
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}, {Key: "ifId", Value: uuid.NewString()}}

	handler.Traffic(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInterfacesOfUnknownRouterIs404(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	id := uuid.NewString()
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+id+"/interfaces", nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	handler.Interfaces(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRefreshMakesDiscoveryDue(t *testing.T) {
	handler, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	require.NoError(t, db.Model(&models.RouterPollJob{}).Where("router_id = ?", created.ID).
		Update("due_at", "2099-01-01").Error)
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers/"+created.ID.String()+"/refresh", nil)
	c.Params = gin.Params{{Key: "id", Value: created.ID.String()}}

	handler.Refresh(c)

	require.Equal(t, http.StatusAccepted, w.Code)
	var job models.RouterPollJob
	require.NoError(t, db.First(&job, "router_id = ? AND kind = ?", created.ID, models.RouterPollKindDiscovery).Error)
	assert.Less(t, job.DueAt.Year(), 2099)
}
