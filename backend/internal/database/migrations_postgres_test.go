package database

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrationCheckSchema is where this file builds its throwaway copy of the
// schema. A dedicated one, because the metrics tests share TEST_POSTGRES_DSN
// and wiping `public` pulled their hypertables out from under them.
const migrationCheckSchema = "migration_check"

// withSearchPath puts one schema in front of the search path, in either of the
// two formats this project's DSNs come in: a URL locally, key=value in CI.
//
// `public` stays on the path behind it, because TimescaleDB installs
// create_hypertable and friends there and the migrations call them unqualified.
// The new schema coming first is what keeps the tables out of `public`.
func withSearchPath(dsn, schema string) string {
	if strings.Contains(dsn, "://") {
		separator := "?"
		if strings.Contains(dsn, "?") {
			separator = "&"
		}
		return dsn + separator + "search_path=" + schema + ",public"
	}
	return dsn + " search_path=" + schema + ",public"
}

var (
	migrationOnce sync.Once
	migrationDB   *gorm.DB
)

// testPostgresDSN returns the DSN every schema in this package builds against,
// skipping locally or failing under CI when it is unset. A second caller
// (the mapping backfill tests, which need their own schema rather than the
// shared one below) is why this is its own function rather than inlined.
func testPostgresDSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the migrations are then never applied anywhere before production")
		}
		t.Skip("set TEST_POSTGRES_DSN to apply the migrations against Postgres")
	}
	return dsn
}

// freshPostgres builds the schema the way startup does — AutoMigrate first,
// then the versioned SQL — in an empty schema of its own, once per run.
//
// Once, not per test: rebuilding thirty-nine migrations and their continuous
// aggregates five times over raced TimescaleDB's own background workers into
// "tuple concurrently deleted". The rules below assert against rows they create
// themselves, so one schema serves them all.
func freshPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := testPostgresDSN(t)
	migrationOnce.Do(func() { migrationDB = buildMigrationSchema(t, dsn) })
	require.NotNil(t, migrationDB, "the schema failed to build")
	return migrationDB
}

func buildMigrationSchema(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	setup, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error)
	// pg_trgm goes into public on this connection, before the private schema
	// exists. Migration 33 would otherwise create it inside migration_check,
	// where gin_trgm_ops is invisible to every other test that later runs the
	// migrations against public — and IF NOT EXISTS then skips the repair.
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error)
	require.NoError(t, setup.Exec(
		"DROP SCHEMA IF EXISTS "+migrationCheckSchema+" CASCADE").Error)
	require.NoError(t, setup.Exec("CREATE SCHEMA "+migrationCheckSchema).Error)
	if sql, err := setup.DB(); err == nil {
		require.NoError(t, sql.Close())
	}

	// A session opened after the extension exists: TimescaleDB is only fully
	// loaded for such sessions, and creating it mid-session made the continuous
	// aggregates in migration 06 fail on a first run and pass on a second.
	// Opened the way production opens it. Under the simple protocol a []byte
	// parameter arrives as a bytea literal, which a jsonb column rejects, so a
	// connection made any other way would not see that failure at all.
	db, err := gorm.Open(
		postgres.New(postgres.Config{
			DSN:                  withSearchPath(dsn, migrationCheckSchema),
			PreferSimpleProtocol: true,
		}),
		&gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	require.NoError(t, RunSQLMigrations(db, "../../migrations"))
	return db
}

// TestEveryMigrationAppliesToAFreshSchema is the only place the SQL migrations
// run before production does. They are written against the tables AutoMigrate
// builds, and nothing else checks that the two still agree.
func TestEveryMigrationAppliesToAFreshSchema(t *testing.T) {
	db := freshPostgres(t)

	var tables int64
	require.NoError(t, db.Raw(
		"SELECT count(*) FROM information_schema.tables WHERE table_schema = ?", migrationCheckSchema).
		Scan(&tables).Error)
	assert.Greater(t, tables, int64(10))
}

// plantFixture makes a site and an OLT nobody else in this file shares, because
// these tests now run against one schema rather than a fresh one each.
func plantFixture(t *testing.T, db *gorm.DB) (models.Site, models.OLT) {
	t.Helper()
	unique := uuid.NewString()[:8]
	site := models.Site{Name: "Site " + unique}
	require.NoError(t, db.Create(&site).Error)
	olt := models.OLT{
		SiteID: site.ID, Name: "OLT " + unique, IPAddress: "10.0.0." + unique[:1],
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
	}
	require.NoError(t, db.Create(&olt).Error)
	return site, olt
}

// Migration 47 drops fcm_token after AutoMigrate has added fid. A drop that
// silently did nothing — a typo in the column name, say — leaves a NOT NULL
// column nothing writes to, and every push subscribe then fails on the real
// schema while passing on SQLite, which never had the old column at all.
func TestPushSubscriptionsCarryOnlyTheInstallationID(t *testing.T) {
	db := freshPostgres(t)

	var columns []string
	require.NoError(t, db.Raw(`SELECT column_name FROM information_schema.columns
		WHERE table_schema = ? AND table_name = 'push_subscriptions'
		AND column_name IN ('fid', 'fcm_token')`,
		migrationCheckSchema).Scan(&columns).Error)

	assert.Equal(t, []string{"fid"}, columns)
}
