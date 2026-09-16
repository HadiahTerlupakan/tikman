package services

import (
	"testing"

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
