package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func brasContext(routerID, query string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+routerID+"/bras?"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: routerID}}
	return w, c
}

func TestBRASServesTheViewOfABRASRouter(t *testing.T) {
	handler, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	now := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, db.Model(&models.MikrotikRouter{}).Where("id = ?", created.ID).
		Updates(map[string]interface{}{"role": models.RouterRoleBRAS, "bras": []byte(`{"sessions":1200}`), "bras_at": now}).Error)
	for i, sessions := range []int{2000, 1880} {
		require.NoError(t, db.Create(&models.RouterBRASSample{RouterID: created.ID,
			Time: now.Add(time.Duration(i-1) * time.Minute), Sessions: sessions, RadiusRejects: int64(10 * (i + 1))}).Error)
	}
	w, c := brasContext(created.ID.String(), "")

	handler.BRAS(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	for _, key := range []string{"bras", "bras_at", "samples", "sessions_15m_ago", "radius_accepts_1h",
		"radius_rejects_1h", "radius_timeouts_1h", "drops"} {
		assert.Contains(t, body, key)
	}
	assert.JSONEq(t, `{"sessions":1200}`, string(body["bras"]))
	assert.Contains(t, string(body["drops"]), `"lost":120`)
	assert.Contains(t, string(body["samples"]), `"sessions":1880`)
}

func TestBRASIsNotFoundForABorderRouter(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	created := createRouterVia(t, handler)
	w, c := brasContext(created.ID.String(), "")

	handler.BRAS(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestBRASRefusesARangeThatIsNotOne(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	for _, query := range []string{"from=yesterday", "to=soon", "from=2026-11-20T10:00:00Z&to=2026-11-20T09:00:00Z"} {
		w, c := brasContext(uuid.NewString(), query)

		handler.BRAS(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, query)
	}
}

func TestBRASRouteNeedsASession(t *testing.T) {
	router, store := rbacTestRouter(t)
	path := "/api/v1/routers/" + uuid.NewString() + "/bras"

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	assert.Equal(t, http.StatusNotFound, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, path),
		"every role reads it; the router does not exist")
}
