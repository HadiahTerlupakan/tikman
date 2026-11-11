package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func memberUpdate(id int64, chat telegram.Chat, status string) telegram.Update {
	return telegram.Update{UpdateID: id, MyChatMember: &telegram.ChatMemberUpdated{
		Chat: chat, NewChatMember: telegram.ChatMember{Status: status},
	}}
}

func chatRows(t *testing.T, db *gorm.DB) []models.TelegramChat {
	t.Helper()
	var chats []models.TelegramChat
	require.NoError(t, db.Order("chat_id ASC").Find(&chats).Error)
	return chats
}

func TestTelegramChatFollowsTheBotJoiningAndLeaving(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	noc := telegram.Chat{ID: -100, Title: "NOC", Type: telegram.ChatTypeSupergroup}

	require.NoError(t, chats.Apply(memberUpdate(1, noc, telegram.StatusMember)))
	require.NoError(t, chats.Apply(memberUpdate(1, noc, telegram.StatusMember)), "a re-delivered update is harmless")
	rows := chatRows(t, db)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Active)

	require.NoError(t, chats.Apply(memberUpdate(2, telegram.Chat{ID: -100, Title: "NOC Baru", Type: telegram.ChatTypeSupergroup}, telegram.StatusKicked)))
	rows = chatRows(t, db)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].Active)
	assert.Equal(t, "NOC Baru", rows[0].Title)

	require.NoError(t, chats.Apply(memberUpdate(3, noc, telegram.StatusAdministrator)))
	assert.True(t, chatRows(t, db)[0].Active)
}

func TestTelegramChatFirstSeenAsLeftIsInactive(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)

	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: -7, Title: "Lama", Type: telegram.ChatTypeGroup}, telegram.StatusLeft)))

	rows := chatRows(t, db)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].Active)
}

func TestTelegramChatIgnoresPrivateChats(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)

	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: 42, Type: "private"}, telegram.StatusMember)))
	require.NoError(t, chats.Apply(telegram.Update{UpdateID: 2, Message: &telegram.Message{Chat: telegram.Chat{ID: 42, Type: "private"}}}))

	assert.Empty(t, chatRows(t, db))
}

// A group upgraded to a supergroup keeps its row, and with it the report.
func TestTelegramChatMigrationKeepsTheRow(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: -5, Title: "NOC", Type: telegram.ChatTypeGroup}, telegram.StatusMember)))
	before := chatRows(t, db)[0]

	require.NoError(t, chats.Apply(telegram.Update{UpdateID: 2, Message: &telegram.Message{
		Chat: telegram.Chat{ID: -5, Title: "NOC", Type: telegram.ChatTypeGroup}, MigrateToChatID: -1005,
	}}))
	require.NoError(t, chats.Apply(telegram.Update{UpdateID: 3, Message: &telegram.Message{
		Chat: telegram.Chat{ID: -1005, Title: "NOC", Type: telegram.ChatTypeSupergroup}, MigrateFromChatID: -5,
	}}))

	rows := chatRows(t, db)
	require.Len(t, rows, 1, "no second chat for the supergroup")
	assert.Equal(t, before.ID, rows[0].ID)
	assert.Equal(t, int64(-1005), rows[0].ChatID)
	assert.Equal(t, telegram.ChatTypeSupergroup, rows[0].Type)
}

// The supergroup's own message can arrive before the group's last one.
func TestTelegramChatMigrationAbsorbsASupergroupSeenFirst(t *testing.T) {
	db := setupTestDB(t)
	chats := NewTelegramChatService(db)
	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: -5, Title: "NOC", Type: telegram.ChatTypeGroup}, telegram.StatusMember)))
	before := chatRows(t, db)[0]
	require.NoError(t, chats.Apply(telegram.Update{UpdateID: 2, Message: &telegram.Message{
		Chat: telegram.Chat{ID: -1005, Title: "NOC", Type: telegram.ChatTypeSupergroup},
	}}))

	require.NoError(t, chats.Migrate(-5, -1005))

	rows := chatRows(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, before.ID, rows[0].ID)
	assert.Equal(t, int64(-1005), rows[0].ChatID)
}

// recordingBot is a Bot API answering getUpdates with updates once, and
// noting each offset it was asked for.
type recordingBot struct {
	mu      sync.Mutex
	offsets []string
	tokens  []string
	updates string
}

func (b *recordingBot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = r.ParseForm()
	b.offsets = append(b.offsets, r.PostForm.Get("offset"))
	b.tokens = append(b.tokens, strings.Split(r.URL.Path, "/")[1])
	_, _ = w.Write([]byte(`{"ok":true,"result":` + b.updates + `}`))
	b.updates = "[]"
}

func TestTelegramChatWatcherPollsWithTheTokenAndAdvances(t *testing.T) {
	db := setupTestDB(t)
	settings := NewSettingService(db, settingServiceTestEncryptionKey)
	bot := &recordingBot{updates: `[{"update_id":41,"my_chat_member":{"chat":{"id":-100,"title":"NOC","type":"supergroup"},"new_chat_member":{"status":"member"}}}]`}
	server := httptest.NewServer(bot)
	t.Cleanup(server.Close)
	watcher := NewTelegramChatWatcher(NewTelegramChatService(db), settings, zap.NewNop())
	watcher.newClient = func(token string) *telegram.Client { return telegram.NewAt(server.URL, token) }
	ctx := context.Background()

	assert.Equal(t, telegramTokenRecheck, watcher.Poll(ctx), "no token, no request")
	assert.Empty(t, bot.offsets)

	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "1:a", uuid.Nil))
	assert.Zero(t, watcher.Poll(ctx))
	assert.Zero(t, watcher.Poll(ctx))
	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "2:b", uuid.Nil))
	assert.Zero(t, watcher.Poll(ctx))

	assert.Equal(t, []string{"0", "42", "0"}, bot.offsets, "a new bot starts from its own first update")
	assert.Equal(t, []string{"bot1:a", "bot1:a", "bot2:b"}, bot.tokens)
	require.Len(t, chatRows(t, db), 1)
}

func TestTelegramChatWatcherBacksOffWhenTelegramFails(t *testing.T) {
	db := setupTestDB(t)
	settings := NewSettingService(db, settingServiceTestEncryptionKey)
	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "1:a", uuid.Nil))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Conflict: terminated by other getUpdates request"}`))
	}))
	t.Cleanup(server.Close)
	watcher := NewTelegramChatWatcher(NewTelegramChatService(db), settings, zap.NewNop())
	watcher.newClient = func(token string) *telegram.Client { return telegram.NewAt(server.URL, token) }

	assert.Equal(t, telegramBackoff, watcher.Poll(context.Background()))
}

func TestTelegramImageCaptionFitsTheLimit(t *testing.T) {
	ports := []string{"Uplink", "Backup"}
	assert.Equal(t, GraphImageCaption("Halo", "PT", ports), TelegramImageCaption("Halo", "PT", ports),
		"a caption that fits is left alone")

	long := TelegramImageCaption(strings.Repeat("a", 2000), "PT", ports)
	assert.Equal(t, telegramCaptionLimit, utf16Len(long))
	assert.True(t, strings.HasSuffix(long, "…\n\nPort: Uplink, Backup"), "the ports line stays whole")

	// Each emoji is two UTF-16 units; a cut through one would leave half a
	// character Telegram rejects.
	emoji := TelegramImageCaption(strings.Repeat("😀", 600), "PT", ports)
	assert.LessOrEqual(t, utf16Len(emoji), telegramCaptionLimit)
	assert.True(t, strings.HasSuffix(emoji, "😀…\n\nPort: Uplink, Backup"))
	assert.True(t, utf8.ValidString(emoji), "no character is split")
}

func TestTelegramImageCaptionWithPortsPastTheLimit(t *testing.T) {
	label := strings.Repeat("😀", 255)

	caption := TelegramImageCaption("Halo", "PT", []string{label, label})

	assert.LessOrEqual(t, utf16Len(caption), telegramCaptionLimit)
	assert.True(t, strings.HasPrefix(caption, "Halo\n\nPort: "))
	assert.True(t, strings.HasSuffix(caption, "…"))
}

// The supergroup's row may be held by a report while the group's row still
// exists; the two cannot share a chat ID, so the group's row is retired.
func TestTelegramChatMigrationIntoAHeldSupergroupRetiresTheGroup(t *testing.T) {
	f := newGraphReportFixture(t)
	chats := NewTelegramChatService(f.db)
	require.NoError(t, chats.Apply(memberUpdate(1, telegram.Chat{ID: -5, Title: "NOC", Type: telegram.ChatTypeGroup}, telegram.StatusMember)))
	require.NoError(t, chats.Apply(memberUpdate(2, telegram.Chat{ID: -1005, Title: "NOC", Type: telegram.ChatTypeSupergroup}, telegram.StatusMember)))
	super := chatRows(t, f.db)[0]
	f.createDestinations(t, uuid.Nil, &super.ID, "Uplink")

	require.NoError(t, chats.Migrate(-5, -1005))

	rows := chatRows(t, f.db)
	require.Len(t, rows, 2)
	assert.Equal(t, super.ID, rows[0].ID, "the reported supergroup row stays as it was")
	assert.True(t, rows[0].Active)
	assert.Equal(t, int64(-5), rows[1].ChatID)
	assert.False(t, rows[1].Active)
}

func newTestWatcher(t *testing.T, db *gorm.DB, bot *recordingBot) (*TelegramChatWatcher, *SettingService) {
	t.Helper()
	settings := NewSettingService(db, settingServiceTestEncryptionKey)
	server := httptest.NewServer(bot)
	t.Cleanup(server.Close)
	watcher := NewTelegramChatWatcher(NewTelegramChatService(db), settings, zap.NewNop())
	watcher.newClient = func(token string) *telegram.Client { return telegram.NewAt(server.URL, token) }
	return watcher, settings
}

// An update the database refuses outright would be refused on every poll;
// stalling on it would stop discovery for every chat after it.
func TestTelegramChatWatcherSkipsAnUpdateTheDatabaseRefuses(t *testing.T) {
	db := setupTestDB(t)
	// Two chats may not share a title here, so the second update violates a
	// constraint the way a real conflict would.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX test_telegram_chats_title ON telegram_chats(title)").Error)
	bot := &recordingBot{updates: `[
		{"update_id":41,"my_chat_member":{"chat":{"id":-100,"title":"NOC","type":"supergroup"},"new_chat_member":{"status":"member"}}},
		{"update_id":42,"my_chat_member":{"chat":{"id":-200,"title":"NOC","type":"supergroup"},"new_chat_member":{"status":"member"}}},
		{"update_id":43,"my_chat_member":{"chat":{"id":-300,"title":"Akses","type":"supergroup"},"new_chat_member":{"status":"member"}}}]`}
	watcher, settings := newTestWatcher(t, db, bot)
	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "1:a", uuid.Nil))

	assert.Zero(t, watcher.Poll(context.Background()))
	assert.Zero(t, watcher.Poll(context.Background()))

	assert.Equal(t, []string{"0", "44"}, bot.offsets, "the refused update is not asked for again")
	assert.Len(t, chatRows(t, db), 2, "the update after it is recorded")
}

func TestTelegramChatWatcherRetiresTheOldBotsChats(t *testing.T) {
	db := setupTestDB(t)
	chat := models.TelegramChat{ChatID: -100, Title: "NOC", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&chat).Error)
	watcher, settings := newTestWatcher(t, db, &recordingBot{updates: "[]"})
	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "1:a", uuid.Nil))

	assert.Zero(t, watcher.Poll(context.Background()))
	assert.True(t, chatRows(t, db)[0].Active, "the first token a process reads is no switch")

	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "2:b", uuid.Nil))
	assert.Zero(t, watcher.Poll(context.Background()))
	assert.False(t, chatRows(t, db)[0].Active, "the new bot is in none of the old one's chats")
}
