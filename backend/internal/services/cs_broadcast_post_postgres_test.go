package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
)

// migrationsDirForTest resolves backend/migrations from this package's test
// working directory, the same relative path setupPostgresTestDB already uses.
func migrationsDirForTest(t *testing.T) string {
	t.Helper()
	return "../../migrations"
}

// Migration 49 retires a table that is live in production with real history in
// it. AutoMigrate creates the new table before the SQL migrations run, so the
// copy — not a rename — is what carries that history across, and nothing but a
// real Postgres will catch it going wrong: SQLite has none of these tables'
// constraints and the unit suite never runs the SQL migrations at all.
func TestMigration49CarriesChannelHistoryIntoTheBroadcastTable(t *testing.T) {
	db := setupPostgresTestDB(t)

	// setupPostgresTestDB already ran every migration, 49 included, against
	// this schema — harmlessly, since wa_channel_posts did not exist yet. That
	// run is what RunSQLMigrations' schema_migrations bookkeeping remembers,
	// so without forgetting it here, the second call below would see version
	// 49 already applied and skip it, and the row built below would never
	// move. Forgetting only 49 is deliberate: this test is about the copy
	// migration 49 performs, not about re-running the whole set.
	require.NoError(t, db.Exec(`DELETE FROM schema_migrations WHERE version = '49'`).Error)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	// The pre-migration shape, as migration 48 left it.
	require.NoError(t, db.Exec(`
		CREATE TABLE wa_channel_posts (
			id uuid PRIMARY KEY,
			wa_account_id uuid NOT NULL,
			channel_jid varchar(128) NOT NULL,
			sender_user_id uuid NOT NULL,
			kind varchar(20) NOT NULL,
			body text,
			media_path text,
			media_mime varchar(100),
			media_size bigint,
			media_filename varchar(255),
			status varchar(20) NOT NULL,
			fail_reason text,
			wa_message_id varchar(128),
			created_at timestamptz NOT NULL DEFAULT now(),
			sent_at timestamptz
		)`).Error)

	legacyID := uuid.New()
	require.NoError(t, db.Exec(`
		INSERT INTO wa_channel_posts
			(id, wa_account_id, channel_jid, sender_user_id, kind, body, status)
		VALUES (?, ?, ?, ?, 'text', 'Selamat datang di chanel SBL Network', 'sent')`,
		legacyID, account.ID, "120363000000000001@newsletter", uuid.New()).Error)

	require.NoError(t, database.RunSQLMigrations(db, migrationsDirForTest(t)))

	var moved models.WABroadcastPost
	require.NoError(t, db.First(&moved, "id = ?", legacyID).Error)
	assert.Equal(t, models.DestinationChannel, moved.Destination)
	require.NotNil(t, moved.DestinationJID)
	assert.Equal(t, "120363000000000001@newsletter", *moved.DestinationJID)
	assert.Equal(t, "Selamat datang di chanel SBL Network", moved.Body)
	assert.Equal(t, models.BroadcastSent, moved.Status)

	var legacyLeft int64
	require.NoError(t, db.Raw(`
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = 'wa_channel_posts'`).
		Scan(&legacyLeft).Error)
	assert.Zero(t, legacyLeft, "the old table must be gone once its history has moved")
}

// The constraint is the design: a status names no channel, a channel must.
func TestPostgresRefusesADestinationThatContradictsItsJID(t *testing.T) {
	db := setupPostgresTestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	jid := "120363000000000001@newsletter"
	statusWithJID := models.WABroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationStatus,
		DestinationJID: &jid, SenderUserID: uuid.New(),
		Kind: models.MessageKindText, Body: "x", Status: models.BroadcastQueued,
	}
	assert.Error(t, db.Create(&statusWithJID).Error)

	channelWithout := models.WABroadcastPost{
		WAAccountID: account.ID, Destination: models.DestinationChannel,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText, Body: "x", Status: models.BroadcastQueued,
	}
	assert.Error(t, db.Create(&channelWithout).Error)
}
