package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func mappingSetup(t *testing.T) *MappingService {
	t.Helper()
	return NewMappingService(setupTestDB(t))
}

func odpNode(id, name string) models.MappingNode {
	return models.MappingNode{
		NodeID: id, Type: models.NodeODP, Name: name,
		Latitude: -6.2, Longitude: 106.8, Capacity: 8,
	}
}

// Two taps in the field: a name and a position. Everything else can wait, and
// demanding more is what left the plant map empty for a year.
func TestANodeNeedsOnlyANameAndAPosition(t *testing.T) {
	s := mappingSetup(t)

	saved, err := s.CreateNode(models.MappingNode{
		NodeID: "ODP-01", Type: models.NodeODP, Name: "ODP Depan Masjid",
		Latitude: -6.21, Longitude: 106.81,
	})

	require.NoError(t, err)
	assert.Equal(t, "ODP-01", saved.NodeID)
	assert.Equal(t, 0, saved.Capacity, "capacity may be left unset")
	assert.Empty(t, saved.Splitter)
}

func TestASecondNodeWithTheSameIDIsRefused(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Satu"))
	require.NoError(t, err)

	_, err = s.CreateNode(odpNode("ODP-01", "Dua"))

	require.ErrorIs(t, err, ErrNodeExists)
}

// mapping_edges addresses its endpoints by node_id, so an OLT with
// coordinates gets a server node to be a valid edge endpoint. The partial
// unique index on olt_id is what keeps that mirror one-to-one: without it,
// nothing would stop a second server node from claiming the same OLT, and
// whichever one a feeder cable happened to point at would be a coin flip.
func TestASecondServerNodeForTheSameOLTIsRefused(t *testing.T) {
	s := mappingSetup(t)
	oltID := uuid.New()
	_, err := s.CreateNode(models.MappingNode{
		NodeID: "SERVER-01", Type: models.NodeServer, Name: "OLT Satu",
		Latitude: -6.2, Longitude: 106.8, OLTID: &oltID,
	})
	require.NoError(t, err)

	_, err = s.CreateNode(models.MappingNode{
		NodeID: "SERVER-02", Type: models.NodeServer, Name: "OLT Satu Lagi",
		Latitude: -6.3, Longitude: 106.9, OLTID: &oltID,
	})

	require.Error(t, err, "one OLT must not end up mirrored by two server nodes")
}

// Two different OLTs must not be blocked from each having their own server
// node - only a repeated olt_id is refused, not olt_id itself being set.
func TestTwoDifferentOLTsEachGetTheirOwnServerNode(t *testing.T) {
	s := mappingSetup(t)
	first, second := uuid.New(), uuid.New()
	_, err := s.CreateNode(models.MappingNode{
		NodeID: "SERVER-01", Type: models.NodeServer, Name: "OLT Satu",
		Latitude: -6.2, Longitude: 106.8, OLTID: &first,
	})
	require.NoError(t, err)

	_, err = s.CreateNode(models.MappingNode{
		NodeID: "SERVER-02", Type: models.NodeServer, Name: "OLT Dua",
		Latitude: -6.3, Longitude: 106.9, OLTID: &second,
	})

	require.NoError(t, err)
}

func TestUpdatingANodeKeepsItsIdentity(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Nama Lama"))
	require.NoError(t, err)

	updated, err := s.UpdateNode("ODP-01", models.MappingNode{
		Name: "Nama Baru", Type: models.NodeODP,
		Latitude: -6.3, Longitude: 106.9, Capacity: 16, PPPoE: "pelanggan-baru",
	})

	require.NoError(t, err)
	assert.Equal(t, "ODP-01", updated.NodeID)
	assert.Equal(t, "Nama Baru", updated.Name)
	assert.Equal(t, 16, updated.Capacity)
	assert.Equal(t, "pelanggan-baru", updated.PPPoE)
}

func TestListingReturnsEveryNodePlaced(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-01", "Satu"))
	require.NoError(t, err)
	_, err = s.CreateNode(odpNode("ODP-02", "Dua"))
	require.NoError(t, err)

	nodes, err := s.ListNodes()

	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestDeletingANodeThatIsNotThereSaysSo(t *testing.T) {
	s := mappingSetup(t)

	err := s.DeleteNode("ODP-TIDAK-ADA")

	require.Error(t, err)
}

// An ONT is a real subscriber's service record, not a sketch: deleting the box
// under it would leave odp_id pointing at nothing while still holding the
// subscriber's slot in uq_onts_odp_port.
func TestDeletingANodeWithAnONTAttachedIsRefused(t *testing.T) {
	db := setupTestDB(t)
	mappingService := NewMappingService(db)
	ontService := NewONTService(db)
	olt := createTestOLT(t, db, uuid.New())
	node, err := mappingService.CreateNode(odpNode("ODP-DELETE-01", "ODP Terpakai"))
	require.NoError(t, err)
	ont := createTestONT(t, ontService, olt.ID, "ZTEGCDELETE01", 1, 1)
	require.NoError(t, ontService.AssignONTToODP(ont.ID, node.ID, 1))

	err = mappingService.DeleteNode(node.NodeID)

	require.ErrorIs(t, err, ErrNodeInUse)
	assert.Contains(t, err.Error(), "1", "the refusal has to say how many ONTs are in the way")
}

func TestDeletingANodeWithNoONTsSucceeds(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(odpNode("ODP-DELETE-02", "ODP Kosong"))
	require.NoError(t, err)

	err = s.DeleteNode("ODP-DELETE-02")

	require.NoError(t, err)
}

// Migration 53 keeps no foreign key from edge to node on purpose: a cable is
// drawn before its ends are named. A node with only a cable pointing at it
// must still be deletable, or that design decision would be undone here.
func TestDeletingANodeWithOnlyCablesAttachedSucceeds(t *testing.T) {
	s := mappingSetup(t)
	_, err := s.CreateNode(models.MappingNode{
		NodeID: "ODC-DELETE-01", Type: models.NodeODC, Name: "ODC",
		Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)
	_, err = s.CreateNode(odpNode("ODP-DELETE-03", "ODP"))
	require.NoError(t, err)
	_, err = s.CreateEdge(models.MappingEdge{
		EdgeID: "E-DELETE-01", Source: "ODC-DELETE-01", Target: "ODP-DELETE-03",
		FiberType: models.FiberDistribution,
	})
	require.NoError(t, err)

	err = s.DeleteNode("ODP-DELETE-03")

	require.NoError(t, err)
}

// oltBackedNode creates a real OLT and a server node mirroring it, the shape
// OLTService's own sync produces - so these tests exercise the guard against
// a node MappingService itself never builds this way outside a test.
func oltBackedNode(t *testing.T, db *gorm.DB, s *MappingService, nodeID string, lat, lon float64) (*models.OLT, *models.MappingNode) {
	t.Helper()
	olt := createTestOLT(t, db, uuid.New())
	// The real sync (olt_map_node.go) never leaves these two disagreeing;
	// setting them here is what makes this fixture the shape it produces,
	// rather than a node that merely happens to carry an olt_id.
	require.NoError(t, db.Model(olt).Updates(map[string]any{"latitude": lat, "longitude": lon}).Error)
	node, err := s.CreateNode(models.MappingNode{
		NodeID: nodeID, Type: models.NodeServer, Name: olt.Name,
		Latitude: lat, Longitude: lon, OLTID: &olt.ID,
	})
	require.NoError(t, err)
	return olt, node
}

// Deleting an OLT-backed node from the map would read as removing a pin, but
// the OLT record, its credentials and everything provisioned under it would
// still exist with no way back onto the map - so this must refuse, not just
// warn.
func TestDeletingAnOLTBackedNodeIsRefused(t *testing.T) {
	db := setupTestDB(t)
	s := NewMappingService(db)
	_, node := oltBackedNode(t, db, s, "SERVER-DELETE-01", -6.2, 106.8)

	err := s.DeleteNode(node.NodeID)

	require.ErrorIs(t, err, ErrNodeMirrorsOLT)
	still, getErr := s.GetNode(node.NodeID)
	require.NoError(t, getErr, "the refusal must leave the node in place")
	assert.Equal(t, node.ID, still.ID)
}

// Moving a pin on the map is exactly how someone corrects where the OLT
// actually sits, and the OLT record must not disagree with the pin
// afterwards.
func TestUpdatingAnOLTBackedNodeMovesTheOLTToo(t *testing.T) {
	db := setupTestDB(t)
	s := NewMappingService(db)
	olt, node := oltBackedNode(t, db, s, "SERVER-MOVE-01", -6.2, 106.8)

	newLat, newLon := -7.0, 108.0
	updated, err := s.UpdateNode(node.NodeID, models.MappingNode{
		Type: models.NodeServer, Name: node.Name, Latitude: newLat, Longitude: newLon,
	})

	require.NoError(t, err)
	assert.Equal(t, newLat, updated.Latitude)
	assert.Equal(t, newLon, updated.Longitude)
	var storedOLT models.OLT
	require.NoError(t, db.First(&storedOLT, "id = ?", olt.ID).Error)
	require.NotNil(t, storedOLT.Latitude)
	require.NotNil(t, storedOLT.Longitude)
	assert.Equal(t, newLat, *storedOLT.Latitude)
	assert.Equal(t, newLon, *storedOLT.Longitude)
}

// Name and type describe the OLT record, not the pin: node_id is already
// never part of an update, and an OLT-backed node's name/type must be just
// as immune, or the map could quietly retype someone's OLT into an ODP.
func TestUpdatingAnOLTBackedNodeIgnoresNameAndTypeChanges(t *testing.T) {
	db := setupTestDB(t)
	s := NewMappingService(db)
	_, node := oltBackedNode(t, db, s, "SERVER-RENAME-01", -6.2, 106.8)

	updated, err := s.UpdateNode(node.NodeID, models.MappingNode{
		Type: models.NodeODP, Name: "Bukan OLT Lagi", Latitude: -6.3, Longitude: 106.9,
	})

	require.NoError(t, err)
	assert.Equal(t, node.Name, updated.Name, "name must stay whatever the OLT record already said")
	assert.Equal(t, models.NodeServer, updated.Type, "type must stay server")
}

// A pin moved onto an impossible coordinate must not reach the OLT record
// either - the same range validateCoordinates already enforces for the OLT
// menu's own latitude/longitude fields.
func TestUpdatingAnOLTBackedNodeRejectsAnOutOfRangeCoordinate(t *testing.T) {
	db := setupTestDB(t)
	s := NewMappingService(db)
	olt, node := oltBackedNode(t, db, s, "SERVER-BADCOORD-01", -6.2, 106.8)

	_, err := s.UpdateNode(node.NodeID, models.MappingNode{
		Type: models.NodeServer, Name: node.Name, Latitude: 200, Longitude: 106.8,
	})

	require.ErrorIs(t, err, ErrValidation)
	var storedOLT models.OLT
	require.NoError(t, db.First(&storedOLT, "id = ?", olt.ID).Error)
	require.NotNil(t, storedOLT.Latitude)
	assert.Equal(t, -6.2, *storedOLT.Latitude, "the OLT record must not move on a rejected update")
}
