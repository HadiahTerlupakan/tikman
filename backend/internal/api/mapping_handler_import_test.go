package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

const importTestKML = `<kml><Document><Folder><name>ODP</name>
<Placemark><name>ODP-01</name><Point><coordinates>106.81,-6.21,0</coordinates></Point></Placemark>
</Folder></Document></kml>`

// buildImportKMZBytes zips one doc.kml entry - the shape a real upload's
// body carries, independent of anything this backend itself ever wrote.
func buildImportKMZBytes(t *testing.T, kml string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("doc.kml")
	require.NoError(t, err)
	_, err = w.Write([]byte(kml))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// importUploadRequest builds a multipart POST carrying one .kmz file -
// mirrors uploadRequest in cs_handler_upload_test.go, the established shape
// for this codebase's other size-capped upload.
func importUploadRequest(t *testing.T, path string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="peta.kmz"`)
	header.Set("Content-Type", "application/vnd.google-earth.kmz")
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestPreviewImportAnswersWhatTheFileHeld(t *testing.T) {
	r, _ := mappingRouter(t)
	req := importUploadRequest(t, "/api/v1/mapping/import/preview", buildImportKMZBytes(t, importTestKML))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data services.ImportPreview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Nodes, 1)
	assert.Equal(t, "ODP-01", body.Data.Nodes[0].NodeID)
}

// The exact failure this measured through the real handler before the fix:
// a NaN distance reached ImportedEdge.Distance, and encoding/json's own
// Marshal refuses to encode NaN - failing *after* c.JSON had already
// written the 200 status, so the response body came back empty and the
// modal had nothing to show.
func TestPreviewImportAnswersAUsableBodyWhenExtendedDataDistanceIsNaN(t *testing.T) {
	r, _ := mappingRouter(t)
	kml := `<kml><Document><Folder><name>Kabel</name>
<Placemark><name>E-1</name><ExtendedData>
<Data name="edge_id"><value>E-1</value></Data>
<Data name="source"><value>ODC-01</value></Data>
<Data name="target"><value>ODP-01</value></Data>
<Data name="distance"><value>NaN</value></Data>
</ExtendedData><LineString><coordinates>106.8,-6.2,0 106.81,-6.21,0</coordinates></LineString></Placemark>
</Folder></Document></kml>`
	req := importUploadRequest(t, "/api/v1/mapping/import/preview", buildImportKMZBytes(t, kml))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, rec.Body.Bytes(), "a 200 with an empty body is exactly the failure this test exists to catch")
	var body struct {
		Data services.ImportPreview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Edges, 1)
	assert.False(t, math.IsNaN(body.Data.Edges[0].Distance))
}

func TestPreviewImportRefusesAnUploadPastTheSizeCap(t *testing.T) {
	r, _ := mappingRouter(t)
	oversized := bytes.Repeat([]byte("x"), maxImportUploadBytes+1)
	req := importUploadRequest(t, "/api/v1/mapping/import/preview", oversized)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "IMPORT_FILE_TOO_LARGE", responseCode(t, rec))
}

func TestPreviewImportRefusesWhenNoFileIsAttached(t *testing.T) {
	r, _ := mappingRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mapping/import/preview", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "IMPORT_FILE_REQUIRED", responseCode(t, rec))
}

func TestPreviewImportRefusesAFileThatIsNotAValidKMZ(t *testing.T) {
	r, _ := mappingRouter(t)
	req := importUploadRequest(t, "/api/v1/mapping/import/preview", []byte("bukan kmz"))
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "IMPORT_PARSE_FAILED", responseCode(t, rec))
}

func TestCommitImportWritesAndAnswersWhatWasCreated(t *testing.T) {
	r, _ := mappingRouter(t)
	body := gin.H{
		"nodes": []gin.H{{"node_id": "ODP-01", "type": "odp", "name": "ODP Satu", "latitude": -6.2, "longitude": 106.8, "include": true}},
		"edges": []gin.H{},
	}

	rec := postJSON(t, r, "/api/v1/mapping/import/commit", body)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data services.ImportResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.NodesCreated)
}

func TestCommitImportAnswers409WhenARowStillCollides(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODP-01", models.NodeODP)
	body := gin.H{
		"nodes": []gin.H{{"node_id": "ODP-01", "type": "odp", "name": "ODP Satu", "latitude": -6.2, "longitude": 106.8, "include": true}},
	}

	rec := postJSON(t, r, "/api/v1/mapping/import/commit", body)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "IMPORT_REFUSED", responseCode(t, rec))
}

func TestCommitImportAnswers409WhenCapacityWouldOverflow(t *testing.T) {
	r, svc := mappingRouter(t)
	seedNode(t, svc, "ODP-01", models.NodeODP)
	require.NoError(t, svc.DeleteNode("ODP-01"))
	_, err := svc.CreateNode(models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP", Latitude: -6.2, Longitude: 106.8, Capacity: 1,
	})
	require.NoError(t, err)
	seedNode(t, svc, "ONT-01", models.NodeONT)
	seedNode(t, svc, "ONT-02", models.NodeONT)
	_, err = svc.CreateEdge(models.MappingEdge{
		EdgeID: "E-1", Source: "ODP-01", Target: "ONT-01", FiberType: models.FiberDrop,
	})
	require.NoError(t, err)

	body := gin.H{
		"edges": []gin.H{{"edge_id": "E-2", "source": "ODP-01", "target": "ONT-02", "fiber_type": "drop", "include": true}},
	}

	rec := postJSON(t, r, "/api/v1/mapping/import/commit", body)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "SLOTS_FULL", responseCode(t, rec))
}

// PreviewImport has carried a MaxBytesReader since it was written; commit
// never did, despite writing to the database while preview only reads it.
func TestCommitImportRefusesABodyPastTheSizeCap(t *testing.T) {
	r, _ := mappingRouter(t)
	longNotes := strings.Repeat("x", maxImportUploadBytes+1)
	body, err := json.Marshal(gin.H{
		"edges": []gin.H{{"edge_id": "E-1", "source": "ODC-01", "target": "ODP-01", "notes": longNotes, "include": true}},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mapping/import/commit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "IMPORT_FILE_TOO_LARGE", responseCode(t, rec))
}
