package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// serverNodeFor is the one server node mirroring oltID, or nil if there is
// none.
func serverNodeFor(t *testing.T, db *gorm.DB, oltID interface{}) *models.MappingNode {
	t.Helper()
	var nodes []models.MappingNode
	require.NoError(t, db.Where("olt_id = ?", oltID).Find(&nodes).Error)
	require.LessOrEqual(t, len(nodes), 1, "an OLT must never be mirrored by more than one server node")
	if len(nodes) == 0 {
		return nil
	}
	return &nodes[0]
}

// A hand-typed community would try a live SNMP probe Create cannot pass in a
// test environment; every test in this file leaves it empty to reach the
// coordinate-sync logic without touching the network.
func mapNodeOLTInput(name string, lat, lon *float64) CreateOLTInput {
	return CreateOLTInput{
		Name: name, IPAddress: "192.0.2.1", Username: "admin", Password: "pass",
		Model: models.OLTModelZTEC300, SSHPort: 22, TelnetPort: 23, SNMPPort: 161,
		PreferredProtocol: models.OLTProtocolSSH, Latitude: lat, Longitude: lon,
	}
}

// Whoever sets an OLT's coordinates must get a pin on the map for it,
// whether they came in through the OLT menu (this path) or the map itself.
func TestCreatingAnOLTWithCoordinatesCreatesAServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)

	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Satu", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)

	node := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, node, "an OLT created with coordinates must get a server node")
	assert.Equal(t, models.NodeServer, node.Type)
	assert.Equal(t, olt.Name, node.Name)
	assert.Equal(t, lat, node.Latitude)
	assert.Equal(t, lon, node.Longitude)
	assert.True(t, strings.HasPrefix(node.NodeID, "SERVER-"), node.NodeID)
}

// An OLT nobody has located yet has nothing to draw on the map, and must not
// get a placeholder pin for the operator to puzzle over.
func TestCreatingAnOLTWithoutCoordinatesCreatesNoServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)

	in := mapNodeOLTInput("OLT Tanpa Lokasi", nil, nil)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)

	assert.Nil(t, serverNodeFor(t, db, olt.ID))
}

// Locating an OLT after the fact - the common case, since discovery runs
// long before anyone visits the site to note its position - must place it on
// the map exactly as if it had been located at creation.
func TestSettingCoordinatesOnAnExistingOLTCreatesAServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	in := mapNodeOLTInput("OLT Belum Lokasi", nil, nil)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)
	require.Nil(t, serverNodeFor(t, db, olt.ID))

	lat, lon := -6.9, 107.6
	err = oltService.Update(olt.ID, map[string]interface{}{"latitude": &lat, "longitude": &lon})
	require.NoError(t, err)

	node := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, node, "setting coordinates must place a server node")
	assert.Equal(t, lat, node.Latitude)
	assert.Equal(t, lon, node.Longitude)
}

// Correcting a mistaken position must move the same pin, not plant a second
// one next to the first.
func TestChangingAnOLTsCoordinatesMovesItsServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Pindah", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)
	original := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, original)

	newLat, newLon := -7.0, 108.0
	err = oltService.Update(olt.ID, map[string]interface{}{"latitude": &newLat, "longitude": &newLon})
	require.NoError(t, err)

	var count int64
	require.NoError(t, db.Model(&models.MappingNode{}).Where("olt_id = ?", olt.ID).Count(&count).Error)
	require.Equal(t, int64(1), count, "moving an OLT must not create a second node")
	moved := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, moved)
	assert.Equal(t, original.NodeID, moved.NodeID, "the node identity must not change on a move")
	assert.Equal(t, newLat, moved.Latitude)
	assert.Equal(t, newLon, moved.Longitude)
}

// name is the one label anything reading mapping_nodes directly - the map's
// own Daftar table included - has for this row; unlike node_id (a cable
// endpoint, frozen on purpose) or a popup that could resolve olt_id itself,
// there is no other place for a rename to reach.
func TestRenamingALocatedOLTRenamesItsServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Nama Lama", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)
	before := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, before)

	err = oltService.Update(olt.ID, map[string]interface{}{"name": "OLT Nama Baru"})
	require.NoError(t, err)

	after := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, after)
	assert.Equal(t, "OLT Nama Baru", after.Name)
	assert.Equal(t, before.NodeID, after.NodeID, "node_id must never move even though the name does")
}

// Fix for the same overflow migration 55_olt_map_node.sql's backfill already
// guards against: olts.name is varchar(255) but mapping_nodes.name is only
// varchar(120). Sharing the OLT write's own transaction means an unguarded
// rename would not just fail to update the mirror - a real Postgres "value
// too long" error here would roll the rename itself back too.
func TestRenamingAnOLTToAnOverlongNameTruncatesItsServerNodesName(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Pendek", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)

	err = oltService.Update(olt.ID, map[string]interface{}{"name": strings.Repeat("X", 255)})
	require.NoError(t, err)

	after := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, after)
	assert.LessOrEqual(t, len(after.Name), 120, "name must fit mapping_nodes.name's varchar(120)")
}

// Same guard, at creation rather than on a later rename: an OLT located and
// named at olts.name's own maximum in the same request must not fail to
// create its mirror.
func TestCreatingAnOLTWithAnOverlongNameTruncatesItsServerNodesName(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput(strings.Repeat("X", 255), &lat, &lon)
	in.SiteID = site.ID

	olt, err := oltService.Create(in)
	require.NoError(t, err)

	node := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, node)
	assert.LessOrEqual(t, len(node.Name), 120, "name must fit mapping_nodes.name's varchar(120)")
}

// Clearing a wrongly entered position is how an operator takes an OLT back
// off the map. Its cables are not this function's concern - migration
// 55_olt_map_node.sql and MappingService.checkSlots already tolerate a
// dangling edge by design - only the pin itself must go.
func TestClearingAnOLTsCoordinatesRemovesItsServerNode(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Dihapus Titiknya", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)
	require.NotNil(t, serverNodeFor(t, db, olt.ID))

	err = oltService.Update(olt.ID, map[string]interface{}{
		"latitude": (*float64)(nil), "longitude": (*float64)(nil),
	})
	require.NoError(t, err)

	assert.Nil(t, serverNodeFor(t, db, olt.ID), "clearing coordinates must remove the server node")
}

// An edit that never touches latitude/longitude must leave the existing pin
// alone - both its identity and its position - not just "not crash".
func TestUpdatingUnrelatedOLTFieldsLeavesItsServerNodeUntouched(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8
	in := mapNodeOLTInput("OLT Ganti Password", &lat, &lon)
	in.SiteID = site.ID
	olt, err := oltService.Create(in)
	require.NoError(t, err)
	before := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, before)

	err = oltService.Update(olt.ID, map[string]interface{}{"password": "new-password"})
	require.NoError(t, err)

	after := serverNodeFor(t, db, olt.ID)
	require.NotNil(t, after)
	assert.Equal(t, before.NodeID, after.NodeID)
	assert.Equal(t, before.Latitude, after.Latitude)
	assert.Equal(t, before.Longitude, after.Longitude)
}

// Fix for the backfill (and this same live path) assuming OLT names are
// unique: nothing enforces that, so two OLTs sharing a name must still each
// get a usable, distinct node_id rather than the second Create failing.
func TestCreatingTwoOLTsWithTheSameNameGetsDistinctServerNodeIDs(t *testing.T) {
	db := setupTestDB(t)
	siteService := NewSiteService(db)
	oltService := NewOLTService(db, testEncryptionKey)
	site, err := siteService.Create("Site", "Loc", "Desc")
	require.NoError(t, err)
	lat, lon := -6.2, 106.8

	firstIn := mapNodeOLTInput("OLT Kembar", &lat, &lon)
	firstIn.SiteID = site.ID
	firstIn.IPAddress = "192.0.2.1"
	first, err := oltService.Create(firstIn)
	require.NoError(t, err)

	secondIn := mapNodeOLTInput("OLT Kembar", &lat, &lon)
	secondIn.SiteID = site.ID
	secondIn.IPAddress = "192.0.2.2"
	second, err := oltService.Create(secondIn)
	require.NoError(t, err)

	firstNode := serverNodeFor(t, db, first.ID)
	secondNode := serverNodeFor(t, db, second.ID)
	require.NotNil(t, firstNode)
	require.NotNil(t, secondNode)
	assert.NotEqual(t, firstNode.NodeID, secondNode.NodeID)
}
