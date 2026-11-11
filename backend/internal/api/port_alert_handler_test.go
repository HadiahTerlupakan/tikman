package api

import (
	"encoding/json"
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
)

const testWebhookToken = "0123456789abcdef"

// hookEnv is the webhook route with the processing it would start recorded
// instead of run.
type hookEnv struct {
	graphReportEnv
	router    *gin.Engine
	triggered []int
}

func newHookEnv(t *testing.T) *hookEnv {
	t.Helper()
	env := &hookEnv{graphReportEnv: newGraphReportEnv(t)}
	sender := services.NewGraphReportSender(env.db, env.settings, t.TempDir(), nil)
	handler := NewPortAlertHandler(services.NewPortAlertService(env.db, sender, zap.NewNop()),
		services.NewPortAlertDestinationService(env.db), nil)
	handler.trigger = func(deviceID int) { env.triggered = append(env.triggered, deviceID) }
	env.router = gin.New()
	(&handlers{portAlertHandler: handler}).registerGraphReportRoutes(env.router.Group("/api/v1"), func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	return env
}

func (e *hookEnv) hook(token, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/hooks/librenms", strings.NewReader(body))
	if token != "" {
		req.Header.Set(libreNMSTokenHeader, token)
	}
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func TestLibreNMSHookRefusesAMissingOrWrongToken(t *testing.T) {
	env := newHookEnv(t)
	assert.Equal(t, http.StatusForbidden, env.hook(testWebhookToken, "text/plain", `{"device_id":"7"}`).Code,
		"no secret configured refuses even a plausible token")

	require.NoError(t, env.settings.Set(models.SettingLibreNMSWebhookToken, testWebhookToken, uuid.Nil))
	assert.Equal(t, http.StatusForbidden, env.hook("", "text/plain", `{"device_id":"7"}`).Code)
	assert.Equal(t, http.StatusForbidden, env.hook("0123456789abcdeX", "text/plain", `{"device_id":"7"}`).Code)

	assert.Empty(t, env.triggered, "nothing is processed")
}

func TestLibreNMSHookAcceptsTheDeviceAsTextOrNumber(t *testing.T) {
	env := newHookEnv(t)
	require.NoError(t, env.settings.Set(models.SettingLibreNMSWebhookToken, testWebhookToken, uuid.Nil))

	text := env.hook(testWebhookToken, "text/plain", `{"device_id":"7","state":"1","name":"Port status up/down"}`)
	number := env.hook(testWebhookToken, "application/json", `{"device_id": 9}`)

	assert.Equal(t, http.StatusAccepted, text.Code, "LibreNMS's default text/plain is read as JSON")
	assert.Equal(t, http.StatusAccepted, number.Code)
	assert.Equal(t, []int{7, 9}, env.triggered)
}

func TestLibreNMSHookRefusesABodyWithoutADevice(t *testing.T) {
	env := newHookEnv(t)
	require.NoError(t, env.settings.Set(models.SettingLibreNMSWebhookToken, testWebhookToken, uuid.Nil))

	for _, body := range []string{``, `{}`, `{"device_id":""}`, `{"device_id":"abc"}`, `{"device_id":0}`} {
		assert.Equal(t, http.StatusBadRequest, env.hook(testWebhookToken, "text/plain", body).Code, body)
	}
	assert.Empty(t, env.triggered)
}

func TestNOCDestinationRoundTrips(t *testing.T) {
	env := newGraphReportEnv(t)

	put := env.do(http.MethodPut, "/api/v1/graph-reports/port-alert-destination",
		map[string]any{"wa_group_id": env.group.ID, "telegram_chat_id": nil, "telegram_thread_id": nil})
	require.Equal(t, http.StatusOK, put.Code, put.Body.String())

	got := env.do(http.MethodGet, "/api/v1/graph-reports/port-alert-destination", nil)
	require.Equal(t, http.StatusOK, got.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(got.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{
		"wa_account_id": env.group.WAAccountID.String(), "wa_group_id": env.group.ID.String(), "group_name": "TEST MRTG",
		"telegram_chat_id": nil, "telegram_chat_title": "", "telegram_thread_id": nil, "telegram_topic_name": "",
		"enabled": true,
	}, body.Data, "enabled defaults to true when left out")
}

func TestNOCDestinationRefusesAnUnknownGroup(t *testing.T) {
	env := newGraphReportEnv(t)

	rec := env.do(http.MethodPut, "/api/v1/graph-reports/port-alert-destination", map[string]any{"wa_group_id": uuid.New()})

	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
