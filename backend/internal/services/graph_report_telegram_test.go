package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

// telegramPost is one send the fake Bot API received.
type telegramPost struct {
	method   string
	chatID   int64
	captions []string
	files    []string
}

// fakeTelegram records sends and answers them with refusal, under status,
// when it is set. hangUp drops the connection without an answer, as a
// timeout would, after the send may well have landed.
type fakeTelegram struct {
	mu       sync.Mutex
	posts    []telegramPost
	requests int
	status   int
	refusal  string
	hangUp   bool
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	if f.hangUp {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
		return
	}
	if f.refusal != "" {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.refusal))
		return
	}
	_ = r.ParseMultipartForm(32 << 20)
	post := telegramPost{}
	post.chatID, _ = strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
	if media := r.FormValue("media"); media != "" {
		post.method = "sendMediaGroup"
		var items []struct{ Caption, Media string }
		_ = json.Unmarshal([]byte(media), &items)
		for _, item := range items {
			post.captions = append(post.captions, item.Caption)
			post.files = append(post.files, r.MultipartForm.File[item.Media[len("attach://"):]][0].Filename)
		}
	} else {
		post.method = "sendPhoto"
		post.captions = []string{r.FormValue("caption")}
		post.files = []string{r.MultipartForm.File["photo"][0].Filename}
	}
	f.posts = append(f.posts, post)
	_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
}

func (f *fakeTelegram) sent() []telegramPost {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]telegramPost(nil), f.posts...)
}

func (f *fakeTelegram) refuse(status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status, f.refusal = status, body
}

func (f *fakeTelegram) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

// useTelegram configures a bot whose API is a fake, and the chat it is in.
func (f graphReportFixture) useTelegram(t *testing.T) (*fakeTelegram, models.TelegramChat) {
	t.Helper()
	fake := &fakeTelegram{}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	require.NoError(t, f.sender.settings.Set(models.SettingTelegramBotToken, "1:bot", uuid.Nil))
	f.sender.telegram = func(token string) *telegram.Client { return telegram.NewAt(server.URL, token) }
	chat := models.TelegramChat{ChatID: -100123, Title: "NOC PT Contoh", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, f.db.Create(&chat).Error)
	return fake, chat
}

func (f graphReportFixture) createDestinations(t *testing.T, groupID uuid.UUID, chatID *uuid.UUID, labels ...string) *GraphReportView {
	t.Helper()
	in := reportInput(groupID, labels...)
	in.TelegramChatID = chatID
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).
		Update("created_at", time.Date(2026, 11, 1, 0, 0, 0, 0, wib)).Error)
	return report
}

func TestTelegramOnlyReportPostsOneAlbum(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "A", "B", "C")
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	posts := fake.sent()
	require.Len(t, posts, 1, "the images go as one album")
	assert.Equal(t, "sendMediaGroup", posts[0].method)
	assert.Equal(t, int64(-100123), posts[0].chatID)
	assert.Equal(t, []string{
		TelegramImageCaption("", "PT Contoh", []string{"A", "B"}),
		TelegramImageCaption("", "PT Contoh", []string{"C"}),
	}, posts[0].captions)
	assert.Equal(t, []string{"PT Contoh 11-11-2026 (1 dari 2).png", "PT Contoh 11-11-2026 (2 dari 2).png"}, posts[0].files)
	assert.Zero(t, f.queuedCount(t), "nothing goes to WhatsApp")
	assert.Empty(t, f.load(t, report.ID).LastError)
}

func TestTelegramSendsASingleImageAsAPhoto(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	f.sender.now = func() time.Time { return atWIB(11, 6, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	posts := fake.sent()
	require.Len(t, posts, 1)
	assert.Equal(t, "sendPhoto", posts[0].method, "Telegram refuses an album of one")
	assert.Equal(t, []string{"PT Contoh 11-11-2026.png"}, posts[0].files)
}

func TestDailySendReachesBothDestinations(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	f.scheduler.now = func() time.Time { return atWIB(11, 6, 0) }

	f.scheduler.Tick(context.Background())

	assert.Equal(t, int64(1), f.queuedCount(t))
	assert.Len(t, fake.sent(), 1)
	stored := f.load(t, report.ID)
	today := wibDate(atWIB(11, 6, 0))
	for name, day := range map[string]*time.Time{
		"last": stored.LastSentOn, "wa": stored.WASentOn, "telegram": stored.TelegramSentOn,
	} {
		require.NotNil(t, day, name)
		assert.True(t, sameDate(day, today), name)
	}
}

// Telegram fails the first try; the retry must not post WhatsApp again.
func TestDailyRetrySkipsTheDestinationAlreadySent(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	fake.refuse(http.StatusTooManyRequests, `{"ok":false,"description":"Too Many Requests: retry after 5"}`)
	tick := func(hour, minute int) {
		f.scheduler.now = func() time.Time { return atWIB(11, hour, minute) }
		f.scheduler.Tick(context.Background())
	}

	tick(6, 0)
	stored := f.load(t, report.ID)
	assert.Equal(t, int64(1), f.queuedCount(t), "WhatsApp is not held up by Telegram")
	assert.Nil(t, stored.LastSentOn, "the day is not done while Telegram is owed it")
	assert.NotNil(t, stored.WASentOn)
	assert.Equal(t, "Terkirim ke WhatsApp; Telegram gagal: Too Many Requests: retry after 5", stored.LastError)

	fake.refuse(0, "")
	tick(6, 10)
	stored = f.load(t, report.ID)
	assert.Equal(t, int64(1), f.queuedCount(t), "the group already has the day's report")
	assert.Len(t, fake.sent(), 1)
	assert.NotNil(t, stored.LastSentOn)
	assert.Empty(t, stored.LastError)
}

func TestManualSendGoesEverywhereAndLeavesTheDayAlone(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	today := wibDate(atWIB(11, 6, 0))
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).
		Updates(map[string]any{"wa_sent_on": today, "telegram_sent_on": today, "last_sent_on": today}).Error)
	f.sender.now = func() time.Time { return atWIB(11, 7, 0) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))

	assert.Equal(t, int64(1), f.queuedCount(t), "a manual send ignores what the day already had")
	assert.Len(t, fake.sent(), 1)
}

func TestAWhatsAppGroupThatCannotTakeItDoesNotStopTelegram(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	require.NoError(t, f.db.Model(&models.WAGroup{}).Where("id = ?", f.group.ID).
		Updates(map[string]any{"announce": true, "is_admin": false}).Error)

	err := f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrGroupAnnounceOnly))
	assert.EqualError(t, err, "Terkirim ke Telegram; WhatsApp gagal: hanya admin grup yang boleh mengirim pesan di grup ini")
	assert.Len(t, fake.sent(), 1)
	assert.Zero(t, f.queuedCount(t))
}

func TestAGraphFailureSendsNowhere(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	f.librenms.fail("if-Uplink", true)

	err := f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrGraphFetch))
	assert.Empty(t, fake.sent())
	assert.Zero(t, f.queuedCount(t))
}

func TestAnUpgradedGroupMovesTheChatToItsSupergroup(t *testing.T) {
	f := newGraphReportFixture(t)
	fake, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	fake.refuse(http.StatusBadRequest, `{"ok":false,"description":"Bad Request: group chat was upgraded to a supergroup chat","parameters":{"migrate_to_chat_id":-100999}}`)

	err := f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrTelegramSend))
	var moved models.TelegramChat
	require.NoError(t, f.db.First(&moved, "id = ?", chat.ID).Error)
	assert.Equal(t, int64(-100999), moved.ChatID)

	fake.refuse(0, "")
	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))
	assert.Equal(t, int64(-100999), fake.sent()[0].chatID, "the retry goes to the supergroup")
}

func TestTelegramWithoutATokenNamesTheSetting(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	report := f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	require.NoError(t, f.sender.settings.Delete(models.SettingTelegramBotToken))

	err := f.sender.SendNow(context.Background(), report.ID)

	assert.True(t, errors.Is(err, ErrTelegramNotConfigured))
	assert.Equal(t, "token bot Telegram belum diatur di Pengaturan", f.load(t, report.ID).LastError)
}

func TestGraphReportDestinationRules(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	left := models.TelegramChat{ChatID: -100456, Title: "Lama", Type: telegram.ChatTypeGroup, Active: true}
	require.NoError(t, f.db.Create(&left).Error)
	require.NoError(t, f.db.Model(&left).Update("active", false).Error)
	f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")
	unknown := uuid.New()

	cases := map[string]struct {
		chatID *uuid.UUID
		want   string
	}{
		"none":         {nil, "pilih grup WhatsApp atau Telegram"},
		"unknown chat": {&unknown, "grup Telegram tidak ditemukan"},
		"left chat":    {&left.ID, "bot sudah tidak ada di grup Telegram ini"},
		"taken chat":   {&chat.ID, "grup Telegram ini sudah memiliki laporan grafik"},
	}
	for name, tc := range cases {
		in := reportInput(uuid.Nil, "Uplink")
		in.TelegramChatID = tc.chatID
		_, err := f.reports.Create(in, uuid.New())
		assert.True(t, errors.Is(err, ErrValidation), name)
		assert.ErrorContains(t, err, tc.want, name)
	}
}

func TestGraphReportViewNamesTheTelegramChat(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	f.createDestinations(t, uuid.Nil, &chat.ID, "Uplink")

	views, err := f.reports.List()

	require.NoError(t, err)
	require.Len(t, views, 1)
	assert.Equal(t, &chat.ID, views[0].TelegramChatID)
	assert.Equal(t, "NOC PT Contoh", views[0].TelegramChatTitle)
	assert.Nil(t, views[0].WAGroupID)
	assert.Nil(t, views[0].WAAccountID)
	assert.Empty(t, views[0].GroupName)
}

func TestDeletingTheWhatsAppGroupLeavesTheTelegramReport(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	both := f.createDestinations(t, f.group.ID, &chat.ID, "Uplink")
	groups := NewWAGroupService(f.db)
	require.NoError(t, groups.MarkLeft(f.group.WAAccountID, f.group.JID))

	require.NoError(t, groups.DeleteLeft(f.group.ID, f.mediaRoot))

	stored := f.load(t, both.ID)
	assert.Nil(t, stored.WAGroupID)
	assert.Equal(t, &chat.ID, stored.TelegramChatID)
}

func TestAReportMayDropItsWhatsAppGroupForTelegram(t *testing.T) {
	f := newGraphReportFixture(t)
	_, chat := f.useTelegram(t)
	report := f.createReport(t, "Uplink")
	in := reportInput(uuid.Nil, "Uplink")
	in.TelegramChatID = &chat.ID

	updated, err := f.reports.Update(report.ID, in)

	require.NoError(t, err)
	assert.Nil(t, updated.WAGroupID)
	assert.Equal(t, "NOC PT Contoh", updated.TelegramChatTitle)
	assert.Nil(t, f.load(t, report.ID).WAGroupID)
}
