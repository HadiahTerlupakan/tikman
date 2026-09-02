package services

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/database"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupPostgresTestDB builds the CS module tables on real Postgres.
//
// Search relies on the tsv generated column, which AutoMigrate cannot create
// (GORM tags have no way to say "generated always as"); it is added by
// migration 41, so this helper runs the real SQL migrations, not just
// AutoMigrate.
func setupPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the tsv column is then never exercised anywhere")
		}
		t.Skip("set TEST_POSTGRES_DSN to search against Postgres")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	for _, table := range []string{"cs_messages", "cs_quick_replies", "cs_conversations", "wa_accounts"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	// Dropping the CS tables above also erases what migration 41 built on them
	// (the tsv column, its constraints). Forget that it ran, so a repeat run
	// against the same disposable container rebuilds them instead of skipping.
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(20) PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error)
	require.NoError(t, db.Exec("DELETE FROM schema_migrations WHERE version = '41'").Error)
	require.NoError(t, models.AutoMigrate(db))
	require.NoError(t, database.RunSQLMigrations(db, "../../migrations"))
	return db
}

// Full-text search is the one part of the message service SQLite cannot answer
// for: the tsvector column and its GIN index exist only in migration 41.
func TestSearchFindsAMessageByItsWordsOnPostgres(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("set TEST_POSTGRES_DSN to search against Postgres")
	}

	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0SEARCH",
		Kind: models.MessageKindText, Body: "lampu los merah berkedip", At: time.Now(),
	})
	require.NoError(t, err)

	found, err := messages.Search("los", 10)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "lampu los merah berkedip", found[0].Body)
}
