package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// chatLookupBot is a Bot API whose getChat answers from chats, refusing the
// IDs it does not hold, and whose getUpdates has nothing new.
type chatLookupBot struct {
	mu      sync.Mutex
	chats   map[string]string
	lookups []string
}

func (b *chatLookupBot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = r.ParseForm()
	if !strings.HasSuffix(r.URL.Path, "/getChat") {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
		return
	}
	id := r.PostForm.Get("chat_id")
	b.lookups = append(b.lookups, id)
	chat, ok := b.chats[id]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
		return
	}
	_, _ = w.Write([]byte(`{"ok":true,"result":` + chat + `}`))
}

func newLookupWatcher(t *testing.T, db *gorm.DB, bot *chatLookupBot) *TelegramChatWatcher {
	t.Helper()
	server := httptest.NewServer(bot)
	t.Cleanup(server.Close)
	settings := NewSettingService(db, settingServiceTestEncryptionKey)
	require.NoError(t, settings.Set(models.SettingTelegramBotToken, "1:a", uuid.Nil))
	watcher := NewTelegramChatWatcher(NewTelegramChatService(db), settings, zap.NewNop())
	watcher.newClient = func(token string) *telegram.Client { return telegram.NewAt(server.URL, token) }
	return watcher
}

// A forum recorded before is_forum existed, from which the bot hears nothing,
// is still offered with its topics once the watcher has asked about it.
func TestTelegramChatWatcherLearnsAnExistingChatIsAForum(t *testing.T) {
	db := setupTestDB(t)
	forum := models.TelegramChat{ChatID: -100, Title: "Pelanggan", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&forum).Error)
	bot := &chatLookupBot{chats: map[string]string{
		"-100": `{"id":-100,"title":"Pelanggan Baru","type":"supergroup","is_forum":true}`,
	}}
	watcher := newLookupWatcher(t, db, bot)

	assert.Zero(t, watcher.Poll(context.Background()))

	chats, err := NewTelegramChatService(db).List()
	require.NoError(t, err)
	require.Len(t, chats, 1)
	assert.True(t, chats[0].IsForum)
	assert.Equal(t, "Pelanggan Baru", chats[0].Title)
	assert.True(t, chats[0].Active)
}

func TestTelegramChatWatcherAsksOnlyOnceAndOnlyAboutActiveSupergroups(t *testing.T) {
	db := setupTestDB(t)
	for _, chat := range []models.TelegramChat{
		{ChatID: -100, Title: "Forum", Type: telegram.ChatTypeSupergroup, Active: true},
		{ChatID: -200, Title: "Hilang", Type: telegram.ChatTypeSupergroup, Active: true},
		{ChatID: -300, Title: "Grup", Type: telegram.ChatTypeGroup, Active: true},
		{ChatID: -400, Title: "Keluar", Type: telegram.ChatTypeSupergroup, Active: true},
	} {
		require.NoError(t, db.Create(&chat).Error)
	}
	require.NoError(t, db.Model(&models.TelegramChat{}).Where("chat_id = -400").Update("active", false).Error)
	bot := &chatLookupBot{chats: map[string]string{
		"-100": `{"id":-100,"title":"Forum","type":"supergroup","is_forum":true}`,
	}}
	watcher := newLookupWatcher(t, db, bot)

	assert.Zero(t, watcher.Poll(context.Background()))
	assert.Zero(t, watcher.Poll(context.Background()))

	assert.Equal(t, []string{"-200", "-100"}, bot.lookups,
		"a refused lookup does not stop the rest, and nothing is asked twice")
	rows := chatRows(t, db)
	assert.True(t, rows[3].IsForum, "-100 sorts last")
	assert.True(t, rows[2].Active, "a refused lookup leaves the chat as it was")
}
