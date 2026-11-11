package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

func (e graphReportEnv) telegramChat(t *testing.T, chatID int64, title string) models.TelegramChat {
	t.Helper()
	chat := models.TelegramChat{ChatID: chatID, Title: title, Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, e.db.Create(&chat).Error)
	return chat
}

func (e graphReportEnv) forumTopic(t *testing.T, chat *models.TelegramChat, threadID int64, name string) {
	t.Helper()
	require.NoError(t, e.db.Model(chat).Update("is_forum", true).Error)
	require.NoError(t, e.db.Create(&models.TelegramTopic{TelegramChatID: chat.ID, ThreadID: threadID, Name: name}).Error)
}

func TestTelegramChatsAreListedByTitle(t *testing.T) {
	env := newGraphReportEnv(t)
	noc := env.telegramChat(t, -100123, "NOC")
	forum := env.telegramChat(t, -100456, "Akses")
	env.forumTopic(t, &forum, 9, "PT Zeta")
	env.forumTopic(t, &forum, 4, "PT Alfa")

	rec := env.do(http.MethodGet, "/api/v1/graph-reports/telegram/chats", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)
	assert.Equal(t, "Akses", body.Data[0]["title"])
	assert.Equal(t, true, body.Data[0]["is_forum"])
	assert.Equal(t, []any{
		map[string]any{"thread_id": float64(4), "name": "PT Alfa"},
		map[string]any{"thread_id": float64(9), "name": "PT Zeta"},
	}, body.Data[0]["topics"])
	assert.Equal(t, map[string]any{
		"id": noc.ID.String(), "chat_id": float64(-100123), "title": "NOC", "type": "supergroup",
		"is_forum": false, "active": true, "topics": []any{},
	}, body.Data[1])
}

func TestGraphReportIntoAForumTopicRoundTrips(t *testing.T) {
	env := newGraphReportEnv(t)
	chat := env.telegramChat(t, -100123, "Pelanggan")
	env.forumTopic(t, &chat, 9, "PT Contoh")
	body := env.reportBody()
	body["wa_group_id"] = nil
	body["telegram_chat_id"] = chat.ID
	body["telegram_thread_id"] = 9

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", body)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var report map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	assert.Equal(t, float64(9), report["telegram_thread_id"])
	assert.Equal(t, "PT Contoh", report["telegram_topic_name"])

	body["telegram_thread_id"] = 10
	rec = env.do(http.MethodPost, "/api/v1/graph-reports", body)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a topic the bot has not seen is refused")
}

func TestGraphReportTelegramOnlyRoundTrips(t *testing.T) {
	env := newGraphReportEnv(t)
	chat := env.telegramChat(t, -100123, "NOC")
	body := env.reportBody()
	body["wa_group_id"] = nil
	body["telegram_chat_id"] = chat.ID

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", body)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var report map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	assert.Equal(t, chat.ID.String(), report["telegram_chat_id"])
	assert.Equal(t, "NOC", report["telegram_chat_title"])
	assert.Nil(t, report["telegram_thread_id"])
	assert.Equal(t, "", report["telegram_topic_name"])
	assert.Nil(t, report["wa_group_id"])
	assert.Nil(t, report["wa_account_id"])
	assert.Equal(t, "", report["group_name"])
	assert.Equal(t, "", report["wa_group_jid"])
	assert.Equal(t, "", report["wa_account_label"])
}

func TestGraphReportWithBothDestinations(t *testing.T) {
	env := newGraphReportEnv(t)
	chat := env.telegramChat(t, -100123, "NOC")
	body := env.reportBody()
	body["telegram_chat_id"] = chat.ID

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", body)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var report map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	assert.Equal(t, env.group.ID.String(), report["wa_group_id"])
	assert.Equal(t, "TEST MRTG", report["group_name"])
	assert.Equal(t, "NOC", report["telegram_chat_title"])
}

func TestGraphReportWithoutADestinationIsRefused(t *testing.T) {
	env := newGraphReportEnv(t)
	body := env.reportBody()
	delete(body, "wa_group_id")

	rec := env.do(http.MethodPost, "/api/v1/graph-reports", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var resp ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "pilih grup WhatsApp atau Telegram", resp.Error)
}

// redirectTelegram sends the process's requests for the public Bot API to
// handler. The sender builds its own client for the public server, so the
// default transport is the one seam a handler test has.
func redirectTelegram(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	fake, err := url.Parse(server.URL)
	require.NoError(t, err)
	public, err := url.Parse(telegram.DefaultBaseURL)
	require.NoError(t, err)
	original := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == public.Host {
			r = r.Clone(r.Context())
			r.URL.Scheme, r.URL.Host = fake.Scheme, fake.Host
		}
		return original.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = original })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGraphReportTelegramSendStatuses(t *testing.T) {
	refusing := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Too Many Requests: retry after 5"}`))
	}
	cases := map[string]struct {
		token    string
		inactive bool
		want     int
		message  string
	}{
		"no token":         {"", false, http.StatusBadRequest, "token bot Telegram belum diatur di Pengaturan"},
		"malformed token":  {"bukan-token", false, http.StatusBadRequest, "token bot Telegram tidak valid"},
		"bot left chat":    {"1:bot", true, http.StatusConflict, "bot sudah tidak ada di grup Telegram ini"},
		"Telegram refuses": {"1:bot", false, http.StatusBadGateway, "Too Many Requests: retry after 5"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := newGraphReportEnv(t)
			var graph bytes.Buffer
			require.NoError(t, png.Encode(&graph, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
			env.useLibreNMS(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(graph.Bytes())
			})
			redirectTelegram(t, refusing)
			if tc.token != "" {
				require.NoError(t, env.settings.Set(models.SettingTelegramBotToken, tc.token, uuid.Nil))
			}
			chat := env.telegramChat(t, -100123, "NOC")
			report := models.GraphReport{TelegramChatID: &chat.ID, CustomerName: "PT Contoh", Enabled: true, SendTime: "06:00"}
			require.NoError(t, env.db.Create(&report).Error)
			require.NoError(t, env.db.Create(&models.GraphReportPort{ReportID: report.ID, DeviceID: 7, IfName: "if-A", Label: "A"}).Error)
			if tc.inactive {
				require.NoError(t, env.db.Model(&chat).Update("active", false).Error)
			}

			rec := env.do(http.MethodPost, fmt.Sprintf("/api/v1/graph-reports/%s/send", report.ID), nil)

			assert.Equal(t, tc.want, rec.Code, rec.Body.String())
			var resp ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			assert.Equal(t, tc.message, resp.Error)
		})
	}
}
