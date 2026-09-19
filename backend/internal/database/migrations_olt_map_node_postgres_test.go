package database

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// oltFixture creates an OLT on a site of its own, with the given name and
// coordinates (nil, nil for one that has never been located) — the two
// states migration 55's backfill has to tell apart.
func oltFixture(t *testing.T, db *gorm.DB, name string, latitude, longitude *float64) models.OLT {
	t.Helper()
	site := models.Site{Name: "Site " + uuid.NewString()[:8]}
	require.NoError(t, db.Create(&site).Error)
	olt := models.OLT{
		SiteID: site.ID, Name: name, IPAddress: "10.0.0." + uuid.NewString()[:1],
		Username: "admin", Password: "enc", Model: models.OLTModelZTEC300,
		Latitude: latitude, Longitude: longitude,
	}
	require.NoError(t, db.Create(&olt).Error)
	return olt
}

// This is the exact scenario production is in today: three OLTs already
// carry coordinates (migration 29) with nothing on the map to show for it.
func TestBackfillCreatesAServerNodeForAnOLTWithCoordinates(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	lat, lon := -6.2, 106.8
	olt := oltFixture(t, db, "OLT "+uuid.NewString()[:8], &lat, &lon)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var node models.MappingNode
	err := db.Where("olt_id = ?", olt.ID).First(&node).Error
	require.NoError(t, err, "an OLT with coordinates must come out with a server node linked to it")
	assert.Equal(t, models.NodeServer, node.Type)
	assert.Equal(t, olt.Name, node.Name)
	assert.Equal(t, lat, node.Latitude)
	assert.Equal(t, lon, node.Longitude)
	assert.Truef(t, strings.HasPrefix(node.NodeID, "SERVER-"),
		"node_id %q must be derived from the OLT name", node.NodeID)
}

// An OLT nobody has surveyed yet has nothing wrong to flag - it simply stays
// off the map, the same as before this migration. Unlike the ODC/ODP
// backfill in migration 53, there is no "[koordinat belum diisi]" case here.
func TestBackfillLeavesAnOLTWithoutCoordinatesOffTheMap(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	olt := oltFixture(t, db, "OLT "+uuid.NewString()[:8], nil, nil)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var count int64
	require.NoError(t, db.Model(&models.MappingNode{}).Where("olt_id = ?", olt.ID).Count(&count).Error)
	assert.Zero(t, count, "an OLT without coordinates must not get a server node")
}

// Fix for the backfill assuming OLT names are unique: nothing enforces that
// anywhere in this codebase, and this ISP could easily end up with two OLTs
// named the same after a rename. A collision must not fail the whole
// migration - which, via RunSQLMigrations, would fail cmd/api's startup.
func TestBackfillGivesTwoOLTsWithTheSameNameDistinctServerNodeIDs(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	name := "OLT " + uuid.NewString()[:8]
	lat, lon := -6.2, 106.8
	first := oltFixture(t, db, name, &lat, &lon)
	second := oltFixture(t, db, name, &lat, &lon)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var nodes []models.MappingNode
	require.NoError(t, db.Where("olt_id IN ?", []uuid.UUID{first.ID, second.ID}).
		Order("node_id").Find(&nodes).Error)
	require.Len(t, nodes, 2, "both colliding OLTs must land in mapping_nodes")
	assert.NotEqual(t, nodes[0].NodeID, nodes[1].NodeID)
	for _, n := range nodes {
		assert.Truef(t, strings.HasPrefix(n.NodeID, "SERVER-"+name),
			"node_id %q must still start with SERVER-%s", n.NodeID, name)
	}
}

// Same overflow guard as migration 53's odc_ids/odp_ids: 'SERVER-' plus a
// name at olts.name's own varchar(255) limit, plus a collision suffix, would
// overflow mapping_nodes.node_id's varchar(64) without it. name is exactly
// 255 - the longest an OLT name can actually be - and two OLTs share it so a
// fix that truncates the finished string instead of reserving room for the
// suffix would cut the suffix off entirely, colliding the two rows with each
// other on the very index this backfill exists to satisfy.
func TestBackfillTruncatesCollidingOverlongOLTNamesToDistinctNodeIDs(t *testing.T) {
	db := freshPostgresBeforeMapping(t)
	name := strings.Repeat("X", 255)
	lat, lon := -6.2, 106.8
	first := oltFixture(t, db, name, &lat, &lon)
	second := oltFixture(t, db, name, &lat, &lon)

	require.NoError(t, RunSQLMigrations(db, "../../migrations"))

	var nodes []models.MappingNode
	require.NoError(t, db.Where("olt_id IN ?", []uuid.UUID{first.ID, second.ID}).
		Order("node_id").Find(&nodes).Error)
	require.Len(t, nodes, 2, "both colliding OLTs must land in mapping_nodes")
	for _, n := range nodes {
		assert.LessOrEqualf(t, len(n.NodeID), 64, "node_id %q must fit varchar(64)", n.NodeID)
	}
	assert.NotEqual(t, nodes[0].NodeID, nodes[1].NodeID, "the disambiguating suffix must survive truncation")
}

// mapping_edges resolves a feeder's source by node_id, not olt_id, so this
// index exists purely to stop one OLT from ending up mirrored by two rows -
// enforced here against the real Postgres constraint the migration creates,
// not only through GORM's SQLite-facing tag.
func TestDatabaseRefusesTwoServerNodesForOneOLT(t *testing.T) {
	db := freshPostgres(t)
	oltID := uuid.New()
	suffix := uuid.NewString()[:8]
	insert := func(nodeID string) error {
		return db.Exec(`INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, olt_id)
			VALUES (?, ?, 'server', 'OLT Test', -6.2, 106.8, ?)`, uuid.New(), nodeID, oltID).Error
	}
	require.NoError(t, insert("SERVER-DUP-1-"+suffix))
	require.Error(t, insert("SERVER-DUP-2-"+suffix), "a second server node for the same OLT must be refused")
}
