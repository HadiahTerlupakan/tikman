package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
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
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type graphReportEnv struct {
	router   *gin.Engine
	settings *services.SettingService
	group    models.WAGroup
	db       *gorm.DB
}

func newGraphReportEnv(t *testing.T) graphReportEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	account := models.WAAccount{Label: "CS", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	group := models.WAGroup{WAAccountID: account.ID, JID: "9@g.us", Name: "TEST MRTG"}
	require.NoError(t, db.Create(&group).Error)

	settings := services.NewSettingService(db, testEncryptionKey)
	sender := services.NewGraphReportSender(db, settings, t.TempDir(), nil)
	h := &handlers{
		graphReportHandler: NewGraphReportHandler(services.NewGraphReportService(db), sender, services.NewTelegramChatService(db), nil),
		portAlertHandler: NewPortAlertHandler(services.NewPortAlertService(db, sender, zap.NewNop()),
			services.NewPortAlertDestinationService(db), nil),
	}
	router := gin.New()
	pass := func(c *gin.Context) { c.Set("user_id", uuid.New()); c.Set("user_role", models.UserRoleAdmin) }
	h.registerGraphReportRoutes(router.Group("/api/v1"), pass)
	return graphReportEnv{router: router, settings: settings, group: group, db: db}
}

// useLibreNMS points the settings at a server answering every request with
// status and body.
func (e graphReportEnv) useLibreNMS(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	require.NoError(t, e.settings.Set(models.SettingLibreNMSURL, server.URL, uuid.Nil))
	require.NoError(t, e.settings.Set(models.SettingLibreNMSAPIToken, "token", uuid.Nil))
}

func (e graphReportEnv) do(method, path string, body any) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e graphReportEnv) reportBody() map[string]any {
	return map[string]any{
		"wa_group_id": e.group.ID, "customer_name": "PT Contoh", "enabled": true,
		"ports": []map[string]any{{"device_id": 7, "device_name": "core", "if_name": "ether1", "label": "Uplink"}},
	}
}

func TestGraphReportCreateAnswersTheReport(t *testing.T) {
	env := newGraphReportEnv(t)

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var report map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	assert.Equal(t, "TEST MRTG", report["group_name"])
	assert.Equal(t, env.group.WAAccountID.String(), report["wa_account_id"])
	assert.Equal(t, "CS", report["wa_account_label"])
	assert.Equal(t, env.group.JID, report["wa_group_jid"])
	assert.Contains(t, report, "last_sent_on")
	assert.Nil(t, report["last_sent_on"])
	assert.Len(t, report["ports"], 1)
	assert.Equal(t, "", report["message"], "an unset message is sent with the default greeting")

	again := env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody())
	assert.Equal(t, http.StatusConflict, again.Code)

	list := env.do(http.MethodGet, "/api/v1/graph-reports", nil)
	assert.Equal(t, http.StatusOK, list.Code)
	assert.Contains(t, list.Body.String(), `"data":[`)
}

func TestGraphReportSendStatuses(t *testing.T) {
	env := newGraphReportEnv(t)
	created := env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody())
	var report services.GraphReportView
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &report))
	path := fmt.Sprintf("/api/v1/graph-reports/%s/send", report.ID)

	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, path, nil).Code, "LibreNMS not configured")

	env.useLibreNMS(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	failed := env.do(http.MethodPost, path, nil)
	assert.Equal(t, http.StatusBadGateway, failed.Code)
	assert.Contains(t, failed.Body.String(), "Uplink")
}

// The browser giving up (its own timeout, a closed tab) cancels the request;
// that must not abort the graphs still being fetched for the group.
func TestGraphReportSendOutlivesTheRequest(t *testing.T) {
	env := newGraphReportEnv(t)
	created := env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody())
	var report services.GraphReportView
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &report))
	var graph bytes.Buffer
	require.NoError(t, png.Encode(&graph, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
	env.useLibreNMS(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(graph.Bytes())
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/graph-reports/%s/send", report.ID), nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestLibreNMSDevicesNamesTheMissingSettings(t *testing.T) {
	env := newGraphReportEnv(t)

	rec := env.do(http.MethodGet, "/api/v1/graph-reports/librenms/devices", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "LibreNMS belum diatur di Pengaturan")
}

func TestLibreNMSPortsAndPreview(t *testing.T) {
	env := newGraphReportEnv(t)
	var graph bytes.Buffer
	require.NoError(t, png.Encode(&graph, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
	env.useLibreNMS(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("graph_type") == "png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(graph.Bytes())
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","ports":[{"port_id":3,"ifName":"ether1","ifAlias":"Uplink","ifOperStatus":"up"}]}`))
	})

	ports := env.do(http.MethodGet, "/api/v1/graph-reports/librenms/devices/7/ports", nil)
	require.Equal(t, http.StatusOK, ports.Code)
	assert.JSONEq(t, `{"data":[{"port_id":3,"if_name":"ether1","if_alias":"Uplink","oper_status":"up"}]}`, ports.Body.String())

	preview := env.do(http.MethodGet, "/api/v1/graph-reports/librenms/graph?device_id=7&if_name=ether1&label=Uplink", nil)
	require.Equal(t, http.StatusOK, preview.Code)
	assert.Equal(t, "image/png", preview.Header().Get("Content-Type"))
	section, err := png.Decode(preview.Body)
	require.NoError(t, err)
	assert.Greater(t, section.Bounds().Dy(), 2, "the graph comes with its title band")

	long := env.do(http.MethodGet,
		"/api/v1/graph-reports/librenms/graph?device_id=7&if_name=ether1&label="+strings.Repeat("x", 256), nil)
	assert.Equal(t, http.StatusBadRequest, long.Code)
}

func TestGraphReportUpdateStoresTheMessage(t *testing.T) {
	env := newGraphReportEnv(t)
	created := env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody())
	var report services.GraphReportView
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &report))

	body := env.reportBody()
	body["message"] = "Halo {pelanggan}"
	rec := env.do(http.MethodPut, "/api/v1/graph-reports/"+report.ID.String(), body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"message":"Halo {pelanggan}"`)

	body["message"] = strings.Repeat("a", services.MaxGraphReportMessageRunes+1)
	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPut, "/api/v1/graph-reports/"+report.ID.String(), body).Code)
}

func TestGraphReportPreviewStatuses(t *testing.T) {
	env := newGraphReportEnv(t)
	body := env.reportBody()
	delete(body, "wa_group_id")
	path := "/api/v1/graph-reports/preview"

	assert.Equal(t, http.StatusBadRequest, env.do(http.MethodPost, path, body).Code, "LibreNMS not configured")

	failing := true
	var graph bytes.Buffer
	require.NoError(t, png.Encode(&graph, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
	env.useLibreNMS(t, func(w http.ResponseWriter, r *http.Request) {
		if failing {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(graph.Bytes())
	})
	assert.Equal(t, http.StatusBadGateway, env.do(http.MethodPost, path, body).Code)

	failing = false
	ok := env.do(http.MethodPost, path, body)
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	var preview GraphReportPreviewResponse
	require.NoError(t, json.Unmarshal(ok.Body.Bytes(), &preview))
	require.Len(t, preview.Images, 1)
	_, err := png.Decode(bytes.NewReader(preview.Images[0].Image))
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(preview.Images[0].Caption, "\n\nPort: Uplink"), preview.Images[0].Caption)

	body["ports"] = []map[string]any{}
	invalid := env.do(http.MethodPost, path, body)
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Contains(t, invalid.Body.String(), `"code"`)
}

func TestGraphReportsAreAdminOnly(t *testing.T) {
	router, store := rbacTestRouter(t)
	for _, role := range []models.UserRole{models.UserRoleViewer, models.UserRoleCS, models.UserRoleTechnician} {
		assert.Equal(t, http.StatusForbidden,
			requestAs(t, router, store, role, http.MethodGet, "/api/v1/graph-reports"), role)
	}
	assert.Equal(t, http.StatusOK, requestAs(t, router, store, models.UserRoleAdmin, http.MethodGet, "/api/v1/graph-reports"))
}

func TestGraphReportMentionAllRoundTrips(t *testing.T) {
	env := newGraphReportEnv(t)
	body := env.reportBody()
	created := env.do(http.MethodPost, "/api/v1/graph-reports", body)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	assert.Contains(t, created.Body.String(), `"mention_all":false`, "left out, a report tags nobody")
	var report services.GraphReportView
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &report))

	body["mention_all"] = true
	rec := env.do(http.MethodPut, "/api/v1/graph-reports/"+report.ID.String(), body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"mention_all":true`)
	assert.Contains(t, env.do(http.MethodGet, "/api/v1/graph-reports", nil).Body.String(), `"mention_all":true`)
}

func TestGraphReportSendTimeRoundTrips(t *testing.T) {
	env := newGraphReportEnv(t)
	body := env.reportBody()
	created := env.do(http.MethodPost, "/api/v1/graph-reports", body)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	assert.Contains(t, created.Body.String(), `"send_time":"06:00"`, "left out, a report goes at 06:00")
	var report services.GraphReportView
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &report))
	path := "/api/v1/graph-reports/" + report.ID.String()

	body["send_time"] = "14:30"
	rec := env.do(http.MethodPut, path, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"send_time":"14:30"`)
	assert.Contains(t, env.do(http.MethodGet, "/api/v1/graph-reports", nil).Body.String(), `"send_time":"14:30"`)

	body["send_time"] = "7:00"
	rec = env.do(http.MethodPut, path, body)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "jam kirim harus berformat HH:MM")
}

func TestGraphReportSameGroupThroughAnotherNumberIsTaken(t *testing.T) {
	env := newGraphReportEnv(t)
	require.Equal(t, http.StatusCreated, env.do(http.MethodPost, "/api/v1/graph-reports", env.reportBody()).Code)
	account := models.WAAccount{Label: "Kantor", Status: models.WAAccountConnected}
	require.NoError(t, env.db.Create(&account).Error)
	twin := models.WAGroup{WAAccountID: account.ID, JID: env.group.JID, Name: env.group.Name}
	require.NoError(t, env.db.Create(&twin).Error)
	body := env.reportBody()
	body["wa_group_id"] = twin.ID

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", body)

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "GRAPH_REPORT_GROUP_TAKEN", resp.Code)
	assert.Equal(t, "Grup ini sudah punya laporan lewat nomor CS", resp.Error)
}
