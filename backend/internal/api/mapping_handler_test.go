package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	g.PUT("/edges/:edge_id", h.UpdateEdge)
	g.DELETE("/edges/:edge_id", h.DeleteEdge)
	return r, svc
}

// doJSON sends a JSON body with the given method; postJSON and putJSON are the
// two shapes the handlers under test actually need.
func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, r, http.MethodPost, path, body)
}

func putJSON(t *testing.T, r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, r, http.MethodPut, path, body)
}

// deleteRequest sends a bodyless DELETE, the shape UpdateNode/UpdateEdge's
// counterparts never need a JSON helper for.
func deleteRequest(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// responseCode reads the machine-readable code a handler answered with, the
// field the frontend actually branches on rather than the free-text message.
func responseCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Code
}

// seedNode places a minimal, valid node directly through the service, for
// tests whose focus is an edge rather than the nodes it connects.
func seedNode(t *testing.T, svc *services.MappingService, nodeID string, typ models.NodeType) {
	t.Helper()
	_, err := svc.CreateNode(models.MappingNode{
		NodeID: nodeID, Type: typ, Name: nodeID, Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)
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
	// A repeated node_id and a full cabinet are both 409s; the frontend tells
	// them apart by this code alone, so it must not collapse to one shared value.
	assert.Equal(t, "NODE_EXISTS", responseCode(t, rec))
}

// The refusal has to reach the technician, not just the log — and it has to be
// distinguishable by code from a duplicate edge, since both answer 409 and the
// frontend branches on the code rather than the free-text message.
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
	assert.Equal(t, "SLOTS_FULL", responseCode(t, rec))
	assert.Contains(t, rec.Body.String(), "1/1")
}

// A duplicate cable and a full cabinet must not read the same to an operator:
// one means "nothing to do", the other means "free a slot first".
func TestADuplicateEdgeAnswers409WithEdgeExists(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODC-01", models.NodeODC)
	seedNode(t, svc, "ODP-01", models.NodeODP)
	first := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01", "fiber_type": "distribution",
	})
	require.Equal(t, http.StatusCreated, first.Code)

	rec := postJSON(t, r, "/api/v1/mapping/edges", gin.H{
		"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01", "fiber_type": "distribution",
	})

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "EDGE_EXISTS", responseCode(t, rec))
}

func TestUpdatingAnEdgeAnswersWithWhatChanged(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODC-01", models.NodeODC)
	seedNode(t, svc, "ODP-01", models.NodeODP)
	_, err := svc.CreateEdge(models.MappingEdge{
		EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01", FiberType: models.FiberDistribution,
	})
	require.NoError(t, err)

	rec := putJSON(t, r, "/api/v1/mapping/edges/E-1", gin.H{
		"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01",
		"fiber_type": "distribution", "notes": "sudah diperbaiki",
	})

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data models.MappingEdge `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "sudah diperbaiki", body.Data.Notes)
}

func TestUpdatingAMissingEdgeAnswers404(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := putJSON(t, r, "/api/v1/mapping/edges/does-not-exist", gin.H{
		"edge_id": "does-not-exist", "source": "ODC-01", "target": "ODP-01",
	})

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "EDGE_NOT_FOUND")
}

func TestDeletingAnEdgeRemovesIt(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODC-01", models.NodeODC)
	seedNode(t, svc, "ODP-01", models.NodeODP)
	_, err := svc.CreateEdge(models.MappingEdge{
		EdgeID: "E-1", Source: "ODC-01", Target: "ODP-01", FiberType: models.FiberDistribution,
	})
	require.NoError(t, err)

	rec := deleteRequest(t, r, "/api/v1/mapping/edges/E-1")

	require.Equal(t, http.StatusNoContent, rec.Code)
	edges, err := svc.ListEdges()
	require.NoError(t, err)
	assert.Empty(t, edges)
}

func TestDeletingAMissingEdgeAnswers404(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := deleteRequest(t, r, "/api/v1/mapping/edges/does-not-exist")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "EDGE_NOT_FOUND")
}

// A server node mirroring an OLT must not be deletable from the map at all -
// the same 409 family as NODE_EXISTS/SLOTS_FULL/NODE_IN_USE, but with its own
// code so the frontend can tell "go to the OLT menu instead" apart from every
// other conflict this endpoint answers.
func TestDeletingAnOLTBackedNodeAnswers409WithItsOwnCode(t *testing.T) {
	r, svc := mappingRouter(t)
	oltID := uuid.New()
	_, err := svc.CreateNode(models.MappingNode{
		NodeID: "SERVER-01", Type: models.NodeServer, Name: "OLT Satu",
		Latitude: -6.2, Longitude: 106.8, OLTID: &oltID,
	})
	require.NoError(t, err)

	rec := deleteRequest(t, r, "/api/v1/mapping/nodes/SERVER-01")

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "NODE_MIRRORS_OLT", responseCode(t, rec))
}

// This is the one case that actually exercises mappingError's not-found
// branch rather than its conflict branch — nothing else in this file did.
func TestUpdatingAMissingNodeAnswers404(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := putJSON(t, r, "/api/v1/mapping/nodes/does-not-exist", gin.H{
		"node_id": "does-not-exist", "type": "odp", "name": "Tidak Ada",
		"latitude": -6.2, "longitude": 106.8,
	})

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "NODE_NOT_FOUND")
}

// A real database failure must not be reported as a missing node — that sends
// someone hunting for a node that was never the problem. Built without
// mappingRouter because this is the one test that needs the *gorm.DB handle,
// to sever it before the request.
func TestCreatingANodeAnswers500OnADatabaseFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := TestDB(t)
	h := NewMappingHandler(services.NewMappingService(db))
	r := gin.New()
	r.POST("/api/v1/mapping/nodes", h.CreateNode)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	rec := postJSON(t, r, "/api/v1/mapping/nodes", gin.H{
		"node_id": "ODP-99", "type": "odp", "name": "Akan Gagal",
		"latitude": -6.2, "longitude": 106.8,
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
