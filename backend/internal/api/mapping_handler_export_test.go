package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func getRequest(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestExportingTheMapAnswersAKMZWithItsOwnContentType(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODP-01", models.NodeODP)

	rec := getRequest(t, r, "/api/v1/mapping/export")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/vnd.google-earth.kmz", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".kmz")
}

func TestExportingTheMapAnswersABodyThatOpensAsAZip(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODP-01", models.NodeODP)

	rec := getRequest(t, r, "/api/v1/mapping/export")

	body := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	assert.Equal(t, "doc.kml", zr.File[0].Name)
}

// An empty map is a real, reachable state (a fresh install, or every node
// removed) - the export must still answer 200 with an openable file, not 500.
func TestExportingAnEmptyMapStillAnswers200(t *testing.T) {
	r, _ := mappingRouter(t)

	rec := getRequest(t, r, "/api/v1/mapping/export")

	assert.Equal(t, http.StatusOK, rec.Code)
}
