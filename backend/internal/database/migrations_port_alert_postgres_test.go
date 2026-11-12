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

// After migration 77 a port is recorded once, the NOC destination is one row, and deleting its group or
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

// Migration 78 opts in every port of a report that had notices on, leaves the
// others' ports silent, and drops the report-level choice.
func TestMigration78MovesThePortNoticeChoiceOntoEachPort(t *testing.T) {
	db := freshPostgresBefore(t, portAlertSchema, "78")
	account := models.WAAccount{Label: "CS", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	insertReport := func(jid string, notify bool) uuid.UUID {
		group := models.WAGroup{WAAccountID: account.ID, JID: jid, Name: jid}
		require.NoError(t, db.Create(&group).Error)
		report := uuid.New()
		require.NoError(t, db.Exec(`INSERT INTO graph_reports
			(id, wa_group_id, customer_name, notify_port_status, created_at, updated_at)
			VALUES (?, ?, 'PT Contoh', ?, now(), now())`, report, group.ID, notify).Error)
		for i, ifName := range []string{"ether1", "ether2"} {
			require.NoError(t, db.Exec(`INSERT INTO graph_report_ports
				(id, report_id, device_id, device_name, if_name, label, position)
				VALUES (?, ?, 7, 'core', ?, ?, ?)`, uuid.New(), report, ifName, ifName, i).Error)
		}
		return report
	}
	loud, quiet := insertReport("1@g.us", true), insertReport("2@g.us", false)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	notified := func(report uuid.UUID) []bool {
		var flags []bool
		require.NoError(t, db.Raw("SELECT notify_status FROM graph_report_ports WHERE report_id = ? ORDER BY position",
			report).Scan(&flags).Error)
		return flags
	}
	assert.Equal(t, []bool{true, true}, notified(loud))
	assert.Equal(t, []bool{false, false}, notified(quiet))
	var left int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'graph_reports'
		AND column_name = 'notify_port_status'`).Scan(&left).Error)
	assert.Zero(t, left)
}
