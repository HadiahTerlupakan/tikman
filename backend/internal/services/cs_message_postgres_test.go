package services

import (
	"os"
	"strings"
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

// csTestSchema keeps this test's tables out of `public`.
//
// It has to be its own schema, not a shared one: the tsv column comes from
// migration 41, reaching it means running the whole migration set, and that
// set builds hypertables and continuous aggregates on ont_metrics. Built in
// `public`, those dependencies make the metrics tests' own DROP TABLE fail —
// tests that pass alone and break when this one runs first.
const csTestSchema = "cs_check"

// csSearchPath puts one schema in front of the path, in either of the two DSN
// shapes this project uses: a URL locally, key=value in CI. `public` stays
// behind it, because TimescaleDB installs create_hypertable there and the
// migrations call it unqualified.
func csSearchPath(dsn, schema string) string {
	if strings.Contains(dsn, "://") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		return dsn + separator + "search_path=" + schema + ",public"
	}
	return dsn + " search_path=" + schema + ",public"
}

// setupPostgresTestDB builds the CS module tables on real Postgres, in a
// schema of their own.
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

	// Extensions land in `public` on this connection, before the private schema
	// exists. Created from inside it instead, migration 33's pg_trgm would put
	// gin_trgm_ops somewhere no other session can see, and IF NOT EXISTS would
	// then skip the repair for everyone.
	setup, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error)
	require.NoError(t, setup.Exec("DROP SCHEMA IF EXISTS "+csTestSchema+" CASCADE").Error)
	require.NoError(t, setup.Exec("CREATE SCHEMA "+csTestSchema).Error)
	if sql, err := setup.DB(); err == nil {
		require.NoError(t, sql.Close())
	}

	// A session opened after the extensions exist: TimescaleDB is only fully
	// loaded for such sessions, and creating it mid-session makes the
	// continuous aggregates fail on a first run and pass on a second.
	db, err := gorm.Open(
		postgres.Open(csSearchPath(dsn, csTestSchema)),
		&gorm.Config{Logger: logger.Discard},
	)
	require.NoError(t, err)
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
