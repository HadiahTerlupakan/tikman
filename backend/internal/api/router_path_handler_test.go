package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"gorm.io/gorm"
)

// stubLookup answers every lookup with out or err and records the target.
type stubLookup struct {
	out     services.RouteLookup
	err     error
	targets []string
	router  *models.MikrotikRouter
}

func (s *stubLookup) LookupRoutes(router *models.MikrotikRouter, target string, _ time.Time) (services.RouteLookup, error) {
	s.targets = append(s.targets, target)
	s.router = router
	return s.out, s.err
}

func setupRouterPathHandlerTest(t *testing.T) (*RouterPathHandler, *stubLookup, RouterResponse, *gorm.DB) {
	t.Helper()
	routers, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, routers)
	lookup := &stubLookup{}
	h := NewRouterPathHandler(services.NewRouterService(db, testEncryptionKey), lookup, services.NewFlowQueryService(db))
	return h, lookup, created, db
}

func pathContext(routerID, path, query string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+routerID+"/bgp/"+path+"?"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: routerID}}
	return w, c
}

func TestLookupAnswersWhatTheRouterHolds(t *testing.T) {
	h, lookup, created, _ := setupRouterPathHandlerTest(t)
	lookup.out = services.RouteLookup{Target: "198.51.100.7", Prefix: "198.51.100.0/23", Queries: 2,
		Routes: []services.StoredRoute{{Session: "peer-ix-1", ASPath: "64500,64496", LocalPref: 700, Active: true, Gateway: "192.0.2.1"}}}
	w, c := pathContext(created.ID.String(), "lookup", "target=198.51.100.7")

	h.Lookup(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, []string{"198.51.100.7"}, lookup.targets)
	assert.Equal(t, created.ID, lookup.router.ID, "the stored router, with its secrets, is handed to the lookup")
	var out services.RouteLookup
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "198.51.100.0/23", out.Prefix)
	assert.Equal(t, "64500,64496", out.Routes[0].ASPath)
}

func TestLookupMapsTheServiceErrorsToStatuses(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		body string
	}{
		{services.ErrLookupTarget, http.StatusBadRequest, "LOOKUP_TARGET"},
		{services.ErrLookupBusy, http.StatusConflict, "LOOKUP_BUSY"},
		{errors.New("dial tcp: timeout"), http.StatusInternalServerError, "ROUTER_FAILED"},
	} {
		h, lookup, created, _ := setupRouterPathHandlerTest(t)
		lookup.err = tc.err
		w, c := pathContext(created.ID.String(), "lookup", "target=x")

		h.Lookup(c)

		assert.Equal(t, tc.code, w.Code, tc.err.Error())
		assert.Contains(t, w.Body.String(), tc.body)
	}
}

func TestLookupOnAnUnknownRouterIs404(t *testing.T) {
	h, lookup, _, _ := setupRouterPathHandlerTest(t)
	w, c := pathContext(uuid.NewString(), "lookup", "target=198.51.100.7")

	h.Lookup(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, lookup.targets)
}

func TestASNamesAnswersTheNamesItHas(t *testing.T) {
	h, _, created, db := setupRouterPathHandlerTest(t)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET", Country: "ID"}).Error)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64500, Name: "IX-RS", Country: "ID"}).Error)
	w, c := pathContext(created.ID.String(), "as-names", "asn=64496,64500,64511")

	h.ASNames(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, map[string]string{"64496": "EXAMPLE-NET", "64500": "IX-RS"}, out, "an AS without a name is left out")
}

func TestASNamesRejectsABadList(t *testing.T) {
	h, _, created, _ := setupRouterPathHandlerTest(t)
	tooMany := strings.Repeat("1,", maxASNamesQuery) + "1"
	for _, query := range []string{"asn=64496,abc", "asn=", "asn=-1", "asn=" + tooMany} {
		w, c := pathContext(created.ID.String(), "as-names", query)

		h.ASNames(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, query)
	}
}

// A lookup loads the router with full-table scans, so like refresh it is for
// the roles that operate it; the names are a read for everyone.
func TestRouteLookupFollowsItsRoles(t *testing.T) {
	router, store := rbacTestRouter(t)
	id := uuid.NewString()

	assert.Equal(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, "/api/v1/routers/"+id+"/bgp/lookup?target=198.51.100.7"))
	assert.NotEqual(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleTechnician, http.MethodGet, "/api/v1/routers/"+id+"/bgp/lookup?target=198.51.100.7"))
	assert.NotEqual(t, http.StatusForbidden, requestAs(t, router, store, models.UserRoleViewer, http.MethodGet, "/api/v1/routers/"+id+"/bgp/as-names?asn=1"))
}
