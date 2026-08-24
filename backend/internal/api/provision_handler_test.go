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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupProvisionTestContext builds a minimal gin context around a request so
// handler-level guards (UUID parsing, confirm flag) can be tested without the
// full router/auth wiring.
func setupProvisionTestContext(t *testing.T, method, target string, payload interface{}) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	return c, w
}

func TestProvisionHandler_ProvisionOnt_RejectsInvalidUUID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))

	handler := NewProvisionHandler(nil, nil)

	c, w := setupProvisionTestContext(t, "POST", "/provision", map[string]interface{}{
		"template_id":   uuid.New().String(),
		"manual_config": map[string]interface{}{"vlan": "100"},
		"confirm":       true,
	})
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	handler.ProvisionOnt(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "INVALID_ID", response["code"])
}

func TestProvisionHandler_ProvisionOnt_RejectsMissingConfirm(t *testing.T) {
	handler := NewProvisionHandler(nil, nil)

	c, w := setupProvisionTestContext(t, "POST", "/provision", map[string]interface{}{
		"template_id":   uuid.New().String(),
		"manual_config": map[string]interface{}{},
		"confirm":       false,
	})
	c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}

	handler.ProvisionOnt(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "NOT_CONFIRMED", response["code"])
}

func TestProvisionHandler_BatchProvision_RejectsMissingConfirm(t *testing.T) {
	handler := NewProvisionHandler(nil, nil)

	c, w := setupProvisionTestContext(t, "POST", "/batch-provision", map[string]interface{}{
		"template_id": uuid.New().String(),
		"ont_ids":     []string{uuid.New().String()},
		"confirm":     false,
	})

	handler.BatchProvision(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "NOT_CONFIRMED", response["code"])
}

func TestProvisionHandler_BatchProvision_RejectsEmptyONTList(t *testing.T) {
	handler := NewProvisionHandler(nil, nil)

	c, w := setupProvisionTestContext(t, "POST", "/batch-provision", map[string]interface{}{
		"template_id": uuid.New().String(),
		"ont_ids":     []string{},
		"confirm":     true,
	})

	handler.BatchProvision(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// paginationParams defaults are exercised indirectly through every list
// endpoint; a direct unit check pins the clamp behaviour.
func TestPaginationParamsDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	limit, offset := paginationParams(c)
	assert.Equal(t, 20, limit)
	assert.Equal(t, 0, offset)
}
