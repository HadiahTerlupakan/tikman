package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

const graphReportTelegramSchema = "graph_report_telegram_check"

// A report's last daily send was WhatsApp's, and after migration 75 a report
// still needs a destination and a chat a single report (in General, after 76).
func TestMigration75KeepsTheDayAndGuardsTheDestinations(t *testing.T) {
	db := freshPostgresBefore(t, graphReportTelegramSchema, "75")
	account := models.WAAccount{Label: "CS", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	group := models.WAGroup{WAAccountID: account.ID, JID: "9@g.us", Name: "TEST MRTG"}
	require.NoError(t, db.Create(&group).Error)
	id := uuid.New()
	sent := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`INSERT INTO graph_reports (id, wa_group_id, customer_name, last_sent_on, created_at, updated_at)
		VALUES (?, ?, 'PT Contoh', ?, now(), now())`, id, group.ID, sent).Error)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var waSentOn string
	require.NoError(t, db.Raw("SELECT wa_sent_on::text FROM graph_reports WHERE id = ?", id).Scan(&waSentOn).Error)
	assert.Equal(t, "2026-11-10", waSentOn)

	err := db.Exec(`INSERT INTO graph_reports (id, customer_name, created_at, updated_at)
		VALUES (?, 'Tanpa tujuan', now(), now())`, uuid.New()).Error
	assert.ErrorContains(t, err, "chk_graph_reports_destination")

	chat := models.TelegramChat{ChatID: -100123, Title: "NOC", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&chat).Error)
	insertTelegram := func() error {
		return db.Exec(`INSERT INTO graph_reports (id, telegram_chat_id, customer_name, created_at, updated_at)
			VALUES (?, ?, 'PT Telegram', now(), now())`, uuid.New(), chat.ID).Error
	}
	require.NoError(t, insertTelegram(), "a Telegram-only report needs no WhatsApp group")
	assert.Error(t, insertTelegram(), "one report per chat")
	assert.Error(t, db.Delete(&chat).Error, "a chat holding a report cannot be deleted")
}

const graphReportTopicSchema = "graph_report_topic_check"

// After migration 76 a forum chat takes a report per topic, General counted
// as one, and the next boot's AutoMigrate does not bring back the per-chat
// unique index the migration dropped.
func TestMigration76AllowsAReportPerForumTopic(t *testing.T) {
	db := freshPostgresBefore(t, graphReportTopicSchema, "76")
	chat := models.TelegramChat{ChatID: -100123, Title: "Pelanggan", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&chat).Error)
	require.NoError(t, RunSQLMigrations(db, "../../migrations"))
	require.NoError(t, models.AutoMigrate(db), "the next boot")

	insert := func(thread any) error {
		return db.Exec(`INSERT INTO graph_reports (id, telegram_chat_id, telegram_thread_id, customer_name, created_at, updated_at)
			VALUES (?, ?, ?, 'PT Contoh', now(), now())`, uuid.New(), chat.ID, thread).Error
	}
	require.NoError(t, insert(9))
	require.NoError(t, insert(12), "another topic of the same forum")
	require.NoError(t, insert(nil), "General")
	assert.ErrorContains(t, insert(9), "idx_graph_reports_telegram_destination")
	assert.ErrorContains(t, insert(nil), "idx_graph_reports_telegram_destination", "General twice")

	var oldIndex int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM pg_indexes WHERE schemaname = ? AND indexname = 'idx_graph_reports_telegram_chat_id'`,
		graphReportTopicSchema).Scan(&oldIndex).Error)
	assert.Zero(t, oldIndex)

	idle := models.TelegramChat{ChatID: -100456, Title: "Lain", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&idle).Error)
	require.NoError(t, db.Create(&models.TelegramTopic{TelegramChatID: idle.ID, ThreadID: 3, Name: "PT Lain"}).Error)
	assert.Error(t, db.Create(&models.TelegramTopic{TelegramChatID: idle.ID, ThreadID: 3}).Error, "one row per topic")
	assert.Error(t, db.Create(&models.TelegramTopic{TelegramChatID: uuid.New(), ThreadID: 3}).Error, "a topic needs its chat")
	require.NoError(t, db.Delete(&idle).Error)
	var topics int64
	require.NoError(t, db.Model(&models.TelegramTopic{}).Where("telegram_chat_id = ?", idle.ID).Count(&topics).Error)
	assert.Zero(t, topics, "a deleted chat takes its topics with it")
}
