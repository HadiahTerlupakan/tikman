package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
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
