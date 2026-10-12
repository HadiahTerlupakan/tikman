package api

import (
	"net/http"
	"net/http/httptest"
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

func setupRouterFlowHandlerTest(t *testing.T) (*RouterFlowHandler, RouterResponse, *gorm.DB) {
	t.Helper()
	routers, db := setupRouterHandlerTest(t)
	created := createRouterVia(t, routers)
	flows := NewRouterFlowHandler(services.NewRouterService(db, testEncryptionKey), services.NewFlowQueryService(db))
	return flows, created, db
}

func flowContext(routerID, query string) (*httptest.ResponseRecorder, *gin.Context) {
	w, c := SetupTestContext(http.MethodGet, "/api/v1/routers/"+routerID+"/flows/read?"+query, nil)
	c.Params = gin.Params{{Key: "id", Value: routerID}}
	return w, c
}

func rangeQuery(span time.Duration) string {
	now := time.Now().UTC()
	return "from=" + now.Add(-span).Format(time.RFC3339) + "&to=" + now.Format(time.RFC3339)
}

func TestTopIPsRefusesARangeBeyondSevenDays(t *testing.T) {
	flows, created, _ := setupRouterFlowHandlerTest(t)
	w, c := flowContext(created.ID.String(), rangeQuery(30*24*time.Hour))

	flows.TopIPs(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "TOP_IPS_RANGE")
}

func TestFlowReadsRefuseMalformedRequests(t *testing.T) {
	flows, created, _ := setupRouterFlowHandlerTest(t)
	hour := rangeQuery(time.Hour)
	cases := map[string]struct{ query, code string }{
		"direction":   {hour + "&direction=sideways", "INVALID_FILTER"},
		"pop":         {hour + "&pop=vlan-pop-a", "INVALID_FILTER"},
		"upstream":    {hour + "&upstream=transit", "INVALID_FILTER"},
		"limit high":  {hour + "&limit=50", "INVALID_FILTER"},
		"limit zero":  {hour + "&limit=0", "INVALID_FILTER"},
		"limit text":  {hour + "&limit=many", "INVALID_FILTER"},
		"six years":   {rangeQuery(6 * 366 * 24 * time.Hour), "INVALID_RANGE"},
		"no range":    {"", "INVALID_RANGE"},
		"reversed":    {"from=2026-10-02T00:00:00Z&to=2026-10-01T00:00:00Z", "INVALID_RANGE"},
		"not rfc3339": {"from=yesterday&to=today", "INVALID_RANGE"},
	}
	for name, tc := range cases {
		w, c := flowContext(created.ID.String(), tc.query)

		flows.Destinations(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, name)
		assert.Contains(t, w.Body.String(), tc.code, name)
	}
}

func TestPeeringNeedsAnASN(t *testing.T) {
	flows, created, _ := setupRouterFlowHandlerTest(t)
	hour := rangeQuery(time.Hour)
	for name, query := range map[string]string{
		"missing":  hour,
		"text":     hour + "&asn=facebook",
		"negative": hour + "&asn=-1",
		"too big":  hour + "&asn=4294967296",
	} {
		w, c := flowContext(created.ID.String(), query)

		flows.Peering(c)

		assert.Equal(t, http.StatusBadRequest, w.Code, name)
		assert.Contains(t, w.Body.String(), "INVALID_FILTER", name)
	}
}

func TestFlowReadsOfAnUnknownRouterAre404(t *testing.T) {
	flows, _, _ := setupRouterFlowHandlerTest(t)
	w, c := flowContext(uuid.NewString(), rangeQuery(time.Hour))

	flows.Summary(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestFlowStatusReportsTheASNDataset(t *testing.T) {
	flows, created, db := setupRouterFlowHandlerTest(t)
	require.NoError(t, db.Create(&models.ASNName{ASN: 64496, Name: "EXAMPLE-NET-A", Country: "ZZ",
		UpdatedAt: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}).Error)
	w, c := flowContext(created.ID.String(), "")

	flows.Status(c)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"asn_count":1`)
	assert.Contains(t, w.Body.String(), `"asn_dataset_updated_at":"2026-10-12T00:00:00`)
}

// Graphs and event lists have always accepted a range reaching past now (a
// browser clock a little ahead of the server); sharing the parser with flows
// must not change that.
func TestRouterTimeRangeAcceptsAFromSlightlyInTheFuture(t *testing.T) {
	from := time.Now().UTC().Add(time.Minute)
	query := "from=" + from.Format(time.RFC3339) + "&to=" + from.Add(time.Hour).Format(time.RFC3339)
	_, c := flowContext(uuid.NewString(), query)

	gotFrom, gotTo, ok := routerTimeRange(c)

	assert.True(t, ok)
	assert.True(t, from.Truncate(time.Second).Equal(gotFrom))
	assert.True(t, from.Add(time.Hour).Truncate(time.Second).Equal(gotTo))
}
