package database

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A node id names a box in the field. Two boxes with one name would make every
// cable that points at it ambiguous.
func TestDatabaseRefusesTwoNodesWithOneNodeID(t *testing.T) {
	db := freshPostgres(t)
	insert := func() error {
		return db.Exec(`INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude)
			VALUES (?, 'ODP-01', 'odp', 'ODP Satu', -6.2, 106.8)`, uuid.New()).Error
	}
	require.NoError(t, insert())
	require.Error(t, insert(), "a second node with the same node_id must be refused")
}

func TestDatabaseRefusesAnUnknownNodeType(t *testing.T) {
	db := freshPostgres(t)
	err := db.Exec(`INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude)
		VALUES (?, 'X-01', 'splitter', 'Bukan Jenis', -6.2, 106.8)`, uuid.New()).Error
	require.Error(t, err)
}

func TestDatabaseRefusesAnUnknownFiberType(t *testing.T) {
	db := freshPostgres(t)
	err := db.Exec(`INSERT INTO mapping_edges (id, edge_id, source, target, fiber_type)
		VALUES (?, 'E-01', 'A', 'B', 'kabel-listrik')`, uuid.New()).Error
	require.Error(t, err)
}

func TestDatabaseAcceptsEveryFiberTypeTheFieldUses(t *testing.T) {
	db := freshPostgres(t)
	for i, ft := range []string{
		"feeder", "distribution", "drop",
		"odp_to_odp", "odp_to_odp_ratio", "odc_to_odc", "odc_to_odc_ratio",
	} {
		err := db.Exec(`INSERT INTO mapping_edges (id, edge_id, source, target, fiber_type)
			VALUES (?, ?, 'A', 'B', ?)`, uuid.New(), "E-"+ft, ft).Error
		require.NoErrorf(t, err, "fiber type %d (%s) must be accepted", i, ft)
	}
}

// mappingBackfillSchema is separate from migrationCheckSchema: tests using
// that one need migration 53 already applied, so the backfill it runs has
// only ever executed over empty odcs/odps tables — never over a real row, so
// a review's two findings against it (colliding codes, unsurveyed boxes
// landing at (0,0)) could not have been caught there. This schema stops short
// of 53 so a test can seed rows before the backfill ever sees them.
const mappingBackfillSchema = "mapping_backfill_check"

// freshPostgresBeforeMapping builds AutoMigrate plus every SQL migration
// older than 53, following buildMigrationSchema's shape but stopping short so
// a test can seed odcs/odps before calling RunSQLMigrations itself.
//
// A schema of its own per call, not the sync.Once singleton freshPostgres
// uses: migration 53 can only "apply" once per schema, so two tests sharing
// one would mean the second test's rows are seeded after 53 already ran and
// so never migrated at all.
func freshPostgresBeforeMapping(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := testPostgresDSN(t)

	setup, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb").Error)
	require.NoError(t, setup.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error)
	require.NoError(t, setup.Exec("DROP SCHEMA IF EXISTS "+mappingBackfillSchema+" CASCADE").Error)
	require.NoError(t, setup.Exec("CREATE SCHEMA "+mappingBackfillSchema).Error)
	if sqlDB, err := setup.DB(); err == nil {
		require.NoError(t, sqlDB.Close())
	}

	db, err := gorm.Open(
		postgres.New(postgres.Config{
			DSN:                  withSearchPath(dsn, mappingBackfillSchema),
			PreferSimpleProtocol: true,
		}),
		&gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	require.NoError(t, models.AutoMigrate(db))
	require.NoError(t, applyMigrationsBefore(db, "../../migrations", "53"))
	return db
}

// applyMigrationsBefore replays every migration older than stopVersion
// through the same executeMigration that RunSQLMigrations itself uses, so
// each is recorded in schema_migrations exactly as a real run would record
// it. A later RunSQLMigrations(db, dir) call then sees them as already
// applied and runs only the ones still pending.
func applyMigrationsBefore(db *gorm.DB, dir, stopVersion string) error {
	if err := ensureMigrationTable(db); err != nil {
		return err
	}
	files, err := loadMigrationFiles(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.version >= stopVersion {
			continue
		}
		sqlBytes, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		if err := executeMigration(db, string(sqlBytes), file.version); err != nil {
			return err
		}
	}
	return nil
}

// odcFixture creates an ODC on a site of its own — the minimum the pre-53
// schema's foreign key requires (fk_odcs_site). code deliberately carries no
// uniqueness constraint of its own, which is exactly what these tests rely on
// being able to violate.
func odcFixture(t *testing.T, db *gorm.DB, code, notes string) {
	t.Helper()
	site := models.Site{Name: "Site " + uuid.NewString()[:8]}
	require.NoError(t, db.Create(&site).Error)
	require.NoError(t, db.Create(&models.ODC{SiteID: site.ID, Code: code, Notes: notes}).Error)
}

// Fix for the backfill assuming ODC.Code is unique: nothing enforces that in
// the model tags, in migrations 39/40, or in CreateODC, and this repo's own
// tests create more than one ODC coded "ODC". A collision must not stop the
// migration — RunSQLMigrations failing here fails cmd/api's startup for the
// whole API, not just the map.
func TestBackfillGivesCollidingODCCodesDistinctNodeIDs(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	code := "ODC-" + uuid.NewString()[:8]
	odcFixture(t, db, code, "")
	odcFixture(t, db, code, "")

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var nodes []models.MappingNode
	require.NoError(t, db.Where("type = ? AND name = ?", string(models.NodeODC), code).
		Order("node_id").Find(&nodes).Error)
	require.Len(t, nodes, 2, "both colliding ODCs must land in mapping_nodes")
	assert.NotEqual(t, nodes[0].NodeID, nodes[1].NodeID)
	for _, n := range nodes {
		assert.Truef(t, strings.HasPrefix(n.NodeID, "ODC-"+code),
			"node_id %q must still start with ODC-%s", n.NodeID, code)
	}
}

// Fix for the backfill placing an unsurveyed box at (0,0): this ISP's plant
// sits between 95 and 141 degrees east, so a row landing at (0,0) is
// certainly wrong and must say so in notes rather than look indistinguishable
// from a real, surveyed position.
func TestBackfillMarksAnODCWithNoCoordinates(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	code := "ODC-" + uuid.NewString()[:8]
	odcFixture(t, db, code, "catatan lama")

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var node models.MappingNode
	require.NoError(t, db.Where("node_id = ?", "ODC-"+code).First(&node).Error)
	assert.Equal(t, 0.0, node.Latitude)
	assert.Equal(t, 0.0, node.Longitude)
	assert.True(t, strings.HasPrefix(node.Notes, "[koordinat belum diisi] "), node.Notes)
	assert.Contains(t, node.Notes, "catatan lama")
}
