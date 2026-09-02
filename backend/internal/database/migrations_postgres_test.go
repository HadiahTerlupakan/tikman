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

// freshPostgres builds the schema the way startup does — AutoMigrate first,
// then the versioned SQL — in an empty schema of its own, once per run.
//
// Once, not per test: rebuilding thirty-nine migrations and their continuous
// aggregates five times over raced TimescaleDB's own background workers into
// "tuple concurrently deleted". The rules below assert against rows they create
// themselves, so one schema serves them all.
func freshPostgres(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_POSTGRES_DSN is unset under CI; the migrations are then never applied anywhere before production")
		}
		t.Skip("set TEST_POSTGRES_DSN to apply the migrations against Postgres")
	}

	migrationOnce.Do(func() { migrationDB = buildMigrationSchema(t, dsn) })
	require.NotNil(t, migrationDB, "the schema failed to build")
	return migrationDB
}

func buildMigrationSchema(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	setup, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error)
	require.NoError(t, setup.Exec(
		"DROP SCHEMA IF EXISTS "+migrationCheckSchema+" CASCADE").Error)
	require.NoError(t, setup.Exec("CREATE SCHEMA "+migrationCheckSchema).Error)
	if sql, err := setup.DB(); err == nil {
		require.NoError(t, sql.Close())
	}

	// A session opened after the extension exists: TimescaleDB is only fully
	// loaded for such sessions, and creating it mid-session made the continuous
	// aggregates in migration 06 fail on a first run and pass on a second.
	db, err := gorm.Open(postgres.Open(withSearchPath(dsn, migrationCheckSchema)),
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

func TestDatabaseRefusesADistributionBoxWithTwoParents(t *testing.T) {
	db := freshPostgres(t)
	site, olt := plantFixture(t, db)
	odc := models.ODC{SiteID: site.ID, Code: "ODC"}
	require.NoError(t, db.Create(&odc).Error)
	slot, port := 1, 4

	// The service refuses this too, but an import or a hand-written UPDATE
	// answers only to the database, which is why the rule is stated twice.
	err := db.Create(&models.ODP{
		Code: "ODP", PortCount: 8, ODCID: &odc.ID,
		OLTID: &olt.ID, Slot: &slot, PortID: &port,
	}).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "odps_exactly_one_parent")
}

func TestDatabaseRefusesADistributionBoxWithNoParent(t *testing.T) {
	db := freshPostgres(t)

	err := db.Create(&models.ODP{Code: "ODP", PortCount: 8}).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "odps_exactly_one_parent")
}

func TestDatabaseRefusesTwoSubscribersOnOneDistributionPort(t *testing.T) {
	db := freshPostgres(t)
	site, olt := plantFixture(t, db)
	odc := models.ODC{SiteID: site.ID, Code: "ODC"}
	require.NoError(t, db.Create(&odc).Error)
	odp := models.ODP{Code: "ODP", PortCount: 8, ODCID: &odc.ID}
	require.NoError(t, db.Create(&odp).Error)
	slot, port := 1, 1

	require.NoError(t, db.Create(&models.ONT{
		OLTID: olt.ID, Slot: &slot, PortID: 1, ONTID: 1,
		SerialNumber: uuid.NewString()[:12], Status: models.ONTStatusOnline,
		ODPID: &odp.ID, ODPPort: &port,
	}).Error)

	err := db.Create(&models.ONT{
		OLTID: olt.ID, Slot: &slot, PortID: 1, ONTID: 2,
		SerialNumber: uuid.NewString()[:12], Status: models.ONTStatusOnline,
		ODPID: &odp.ID, ODPPort: &port,
	}).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_onts_odp_port")
}

func TestDatabaseAllowsManySubscribersWithNoDistributionPortYet(t *testing.T) {
	db := freshPostgres(t)
	_, olt := plantFixture(t, db)
	slot := 1

	// Every subscriber starts unassigned, and a unique index that treated
	// "no port yet" as a value would let exactly one of them exist.
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&models.ONT{
			OLTID: olt.ID, Slot: &slot, PortID: 1, ONTID: i,
			SerialNumber: uuid.NewString()[:12], Status: models.ONTStatusOnline,
		}).Error)
	}
}

func TestSchemaCarriesTheCableRoutes(t *testing.T) {
	db := freshPostgres(t)

	// Columns AutoMigrate adds from the model tags, which is why stage two
	// needed no migration of its own. If that ever stops being true, the map
	// draws nothing and this says so first.
	for _, table := range []string{"odps", "odc_feeds"} {
		var columns int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ?
			AND column_name IN ('route', 'route_meters')`,
			migrationCheckSchema, table).Scan(&columns).Error)
		assert.EqualValues(t, 2, columns, table)
	}
}
