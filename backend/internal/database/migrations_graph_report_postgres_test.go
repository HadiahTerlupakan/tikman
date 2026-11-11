package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

const graphReportSendTimeSchema = "graph_report_send_time_check"

// A report registered before migration 74 keeps going out at 06:00, the only
// time there was. AutoMigrate has already added the column from the model
// tag, so it is dropped first to stand for the table production holds.
func TestMigration74SendsExistingReportsAtSix(t *testing.T) {
	db := freshPostgresBefore(t, graphReportSendTimeSchema, "74")
	require.NoError(t, db.Exec("ALTER TABLE graph_reports DROP COLUMN send_time").Error)
	account := models.WAAccount{Label: "CS", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	group := models.WAGroup{WAAccountID: account.ID, JID: "9@g.us", Name: "TEST MRTG"}
	require.NoError(t, db.Create(&group).Error)
	id := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO graph_reports (id, wa_group_id, customer_name, created_at, updated_at)
		VALUES (?, ?, 'PT Contoh', now(), now())`, id, group.ID).Error)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var sendTime string
	require.NoError(t, db.Raw("SELECT send_time FROM graph_reports WHERE id = ?", id).Scan(&sendTime).Error)
	assert.Equal(t, "06:00", sendTime)
}
