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
// still needs a destination and a chat a single report.
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
