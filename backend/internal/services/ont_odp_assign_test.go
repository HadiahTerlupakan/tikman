package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// AssignONTToODP shares its placement rule with registration
// (validateRegisterODP in zte_register_odp.go, via validateODPPortPlacement):
// the target must be a mapping_nodes row of type odp, and the port must fit
// inside its stated capacity. The port-collision case below exercises the
// database's own uq_onts_odp_port index instead of a racy read-then-write
// check in the service.

func setupODPAssignFixture(t *testing.T) (*ONTService, *MappingService, uuid.UUID) {
	t.Helper()
	db := setupTestDB(t)
	ontService := NewONTService(db)
	mappingService := NewMappingService(db)
	olt := createTestOLT(t, db, uuid.New())
	return ontService, mappingService, olt.ID
}

func createTestODPNode(t *testing.T, mappingService *MappingService, nodeID string, capacity int) *models.MappingNode {
	t.Helper()
	node, err := mappingService.CreateNode(models.MappingNode{
		NodeID: nodeID, Type: models.NodeODP, Name: "ODP Uji",
		Latitude: -6.2, Longitude: 106.8, Capacity: capacity,
	})
	require.NoError(t, err)
	return node
}

func TestAssignONTToODPRefusesAnUnknownODP(t *testing.T) {
	ontService, _, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN01", 1, 1)

	err := ontService.AssignONTToODP(ont.ID, uuid.New(), 1)

	require.Error(t, err)
}

func TestAssignONTToODPRefusesANodeThatIsNotAnODP(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN02", 1, 1)
	node, err := mappingService.CreateNode(models.MappingNode{
		NodeID: "ODC-ASSIGN-01", Type: models.NodeODC, Name: "ODC Uji",
		Latitude: -6.2, Longitude: 106.8,
	})
	require.NoError(t, err)

	err = ontService.AssignONTToODP(ont.ID, node.ID, 1)

	require.Error(t, err)
}

func TestAssignONTToODPRefusesPortZero(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN03", 1, 1)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-01", 4)

	err := ontService.AssignONTToODP(ont.ID, node.ID, 0)

	require.Error(t, err)
}

func TestAssignONTToODPRefusesAPortPastCapacity(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN04", 1, 1)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-02", 4)

	err := ontService.AssignONTToODP(ont.ID, node.ID, 5)

	require.Error(t, err)
}

// Capacity 0 means nobody has counted this box's ports yet, the same as it
// means for registration's own ODP check and CreateEdge's slot check — not
// that the box holds nothing.
func TestAssignONTToODPAcceptsAnyPortWhenCapacityIsUnstated(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN05", 1, 1)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-03", 0)

	err := ontService.AssignONTToODP(ont.ID, node.ID, 999)

	require.NoError(t, err)
}

func TestAssignONTToODPRecordsThePlacement(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN06", 1, 1)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-04", 4)

	require.NoError(t, ontService.AssignONTToODP(ont.ID, node.ID, 2))

	updated, err := ontService.GetByID(ont.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.ODPID)
	assert.Equal(t, node.ID, *updated.ODPID)
	require.NotNil(t, updated.ODPPort)
	assert.Equal(t, 2, *updated.ODPPort)
}

// Updates does not treat zero rows affected as an error on its own, so
// without this check a stray ONT id would report success while writing
// nothing — the box and port are real, the ONT is not.
func TestAssignONTToODPRefusesAnUnknownONT(t *testing.T) {
	ontService, mappingService, _ := setupODPAssignFixture(t)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-08", 4)

	err := ontService.AssignONTToODP(uuid.New(), node.ID, 1)

	require.Error(t, err)
}

// The unique index on (odp_id, odp_port) is the final arbiter when two
// assignments race for the same port, not a read-then-write check in
// AssignONTToODP.
func TestAssignONTToODPRefusesAPortAnotherONTAlreadyHolds(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	first := createTestONT(t, ontService, oltID, "ZTEGCASSIGN07", 1, 1)
	second := createTestONT(t, ontService, oltID, "ZTEGCASSIGN08", 1, 2)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-05", 4)
	require.NoError(t, ontService.AssignONTToODP(first.ID, node.ID, 3))

	err := ontService.AssignONTToODP(second.ID, node.ID, 3)

	require.Error(t, err)
}

func TestUnassignONTFromODPClearsThePlacement(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	ont := createTestONT(t, ontService, oltID, "ZTEGCASSIGN09", 1, 1)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-06", 4)
	require.NoError(t, ontService.AssignONTToODP(ont.ID, node.ID, 1))

	require.NoError(t, ontService.UnassignONTFromODP(ont.ID))

	updated, err := ontService.GetByID(ont.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.ODPID)
	assert.Nil(t, updated.ODPPort)
}

// Freeing a port has to actually release it from the unique index, not just
// clear the ONT that held it, or the next subscriber patched into the same
// port would collide with a row that looks unassigned.
func TestUnassignONTFromODPFreesThePortForAnotherONT(t *testing.T) {
	ontService, mappingService, oltID := setupODPAssignFixture(t)
	first := createTestONT(t, ontService, oltID, "ZTEGCASSIGN10", 1, 1)
	second := createTestONT(t, ontService, oltID, "ZTEGCASSIGN11", 1, 2)
	node := createTestODPNode(t, mappingService, "ODP-ASSIGN-07", 4)
	require.NoError(t, ontService.AssignONTToODP(first.ID, node.ID, 1))
	require.NoError(t, ontService.UnassignONTFromODP(first.ID))

	err := ontService.AssignONTToODP(second.ID, node.ID, 1)

	require.NoError(t, err)
}

func TestUnassignONTFromODPRefusesAnUnknownONT(t *testing.T) {
	ontService, _, _ := setupODPAssignFixture(t)

	err := ontService.UnassignONTFromODP(uuid.New())

	require.Error(t, err)
}
