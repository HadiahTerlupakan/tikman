package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
)

const portAlertSchema = "port_alert_check"

// After migration 77 a report made before it stays silent, a port is
// recorded once, the NOC destination is one row, and deleting its group or
// chat unsets that destination instead of being refused or taking the row.
func TestMigration77GuardsThePortAlertTables(t *testing.T) {
	db := freshPostgresBefore(t, portAlertSchema, "77")
	account := models.WAAccount{Label: "CS", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	group := models.WAGroup{WAAccountID: account.ID, JID: "9@g.us", Name: "NOC"}
	require.NoError(t, db.Create(&group).Error)
	report := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO graph_reports (id, wa_group_id, customer_name, created_at, updated_at)
		VALUES (?, ?, 'PT Contoh', now(), now())`, report, group.ID).Error)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var notify bool
	require.NoError(t, db.Raw("SELECT notify_port_status FROM graph_reports WHERE id = ?", report).Scan(&notify).Error)
	assert.False(t, notify)

	insertState := func(status string) error {
		return db.Exec(`INSERT INTO port_link_states (id, device_id, if_name, status, changed_at)
			VALUES (?, 7, 'ether1', ?, now())`, uuid.New(), status).Error
	}
	assert.ErrorContains(t, insertState("unknown"), "chk_port_link_states_status")
	require.NoError(t, insertState("down"))
	assert.ErrorContains(t, insertState("up"), "idx_port_link_states_port")

	chat := models.TelegramChat{ChatID: -100123, Title: "NOC", Type: telegram.ChatTypeSupergroup, Active: true}
	require.NoError(t, db.Create(&chat).Error)
	noc := models.PortAlertDestination{WAGroupID: &group.ID, TelegramChatID: &chat.ID, Enabled: true}
	require.NoError(t, db.Create(&noc).Error)
	assert.ErrorContains(t, db.Create(&models.PortAlertDestination{Enabled: true}).Error,
		"idx_port_alert_destination_single")

	require.NoError(t, db.Exec("DELETE FROM graph_reports").Error)
	require.NoError(t, db.Delete(&group).Error)
	require.NoError(t, db.Delete(&chat).Error)
	var stored models.PortAlertDestination
	require.NoError(t, db.First(&stored, "id = ?", noc.ID).Error)
	assert.Nil(t, stored.WAGroupID)
	assert.Nil(t, stored.TelegramChatID)
}
