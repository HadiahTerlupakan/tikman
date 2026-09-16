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

func mappingRouter(t *testing.T) (*gin.Engine, *services.MappingService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := TestDB(t)
	svc := services.NewMappingService(db)
	h := NewMappingHandler(svc)

	r := gin.New()
	g := r.Group("/api/v1/mapping")
	g.GET("/nodes", h.ListNodes)
	g.POST("/nodes", h.CreateNode)
	g.PUT("/nodes/:node_id", h.UpdateNode)
	g.DELETE("/nodes/:node_id", h.DeleteNode)
	g.GET("/edges", h.ListEdges)
	g.POST("/edges", h.CreateEdge)
	return r, svc
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestPlacingANodeAnswersWithWhatWasStored(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "ODP Depan Masjid",
		"latitude": -6.21, "longitude": 106.81,
	})

	require.Equal(t, http.StatusCreated, rec.Code)
	var body struct {
		Data models.MappingNode `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "ODP-01", body.Data.NodeID)
}

func TestANodeWithoutAPositionIsRefused(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Tanpa Koordinat",
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestARepeatedNodeIDAnswers409(t *testing.T) {
	r, _ := mappingRouter(t)
	first := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Satu",
		"latitude": -6.2, "longitude": 106.8,
	})
	require.Equal(t, http.StatusCreated, first.Code)

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-01", "type": "odp", "name": "Dua",
		"latitude": -6.2, "longitude": 106.8,
	})

	assert.Equal(t, http.StatusConflict, rec.Code)
}

// The refusal has to reach the technician, not just the log.
func TestAFullCabinetAnswers409WithItsNumbers(t *testing.T) {
	r, svc := mappingRouter(t)
	_, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODC-01", Type: models.NodeODC, Name: "ODC", Latitude: -6.2, Longitude: 106.8, Capacity: 1,
	})
	require.NoError(t, err)
	for _, id := range []string{"ODP-01", "ODP-02"} {
		_, err := svc.CreateNode(models.MappingNode{
			NodeID: id, Type: models.NodeODP, Name: id, Latitude: -6.2, Longitude: 106.8,
		})
		require.NoError(t, err)
	}
	first := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01", "fiber_type": "distribution",
	})
	require.Equal(t, http.StatusCreated, first.Code)

	rec := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-2", "source": "ODC-01", "target": "ODP-02", "fiber_type": "distribution",
	})

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "1/1")
}
