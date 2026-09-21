package services

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func includedNode(id string, typ models.NodeType) ImportedNode {
	return ImportedNode{NodeID: id, Type: typ, Name: id, Latitude: -6.2, Longitude: 106.8, Include: true}
}

func includedEdge(id, source, target string, fiber models.FiberType) ImportedEdge {
	return ImportedEdge{EdgeID: id, Source: source, Target: target, FiberType: fiber, Include: true}
}

func TestCommitImportWritesOnlyRowsMarkedInclude(t *testing.T) {
	s := mappingSetup(t)
	nodes := []ImportedNode{
		includedNode("ODP-01", models.NodeODP),
		{NodeID: "ODP-02", Type: models.NodeODP, Include: false}, // left out on purpose
	}

	result, err := s.CommitImport(nodes, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, result.NodesCreated)
	got, err := s.ListNodes()
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ODP-01", got[0].NodeID)
}

// The transaction guarantee this whole design leans on: one bad row must
// leave nothing behind, not even the good rows alongside it - a half-import
// is worse than a rejected one, since nobody could tell what landed.
func TestCommitImportWritesNothingWhenOneIncludedNodeStillCollides(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODP-01", models.NodeODP, 0)
	nodes := []ImportedNode{
		includedNode("ODP-02", models.NodeODP),
		includedNode("ODP-01", models.NodeODP), // already on the map
	}

	_, err := s.CommitImport(nodes, nil)

	require.ErrorIs(t, err, ErrImportInvalid)
	got, err := s.ListNodes()
	require.NoError(t, err)
	require.Len(t, got, 1, "only the pre-existing node may remain - ODP-02 must not have been left behind")
	assert.Equal(t, "ODP-01", got[0].NodeID)
}

// Defense in depth: even if a client cleared Blocked and set Include, the
// commit itself refuses a server-typed row by its Type, not by a flag the
// caller controls.
func TestCommitImportRefusesAServerNodeEvenWithIncludeSetAndBlockedCleared(t *testing.T) {
	s := mappingSetup(t)
	node := ImportedNode{NodeID: "SERVER-01", Type: models.NodeServer, Include: true, Blocked: false}

	_, err := s.CommitImport([]ImportedNode{node}, nil)

	require.ErrorIs(t, err, ErrImportInvalid)
	got, err := s.ListNodes()
	require.NoError(t, err)
	assert.Empty(t, got)
}

// migrations/53_network_mapping.sql: a cable naming a node that does not
// exist is a normal, reachable state, not an import error.
func TestCommitImportToleratesAnEdgeWhoseEndpointsDoNotExist(t *testing.T) {
	s := mappingSetup(t)
	edge := includedEdge("E-1", "ghost-source", "ghost-target", models.FiberDistribution)

	result, err := s.CommitImport(nil, []ImportedEdge{edge})

	require.NoError(t, err)
	assert.Equal(t, 1, result.EdgesCreated)
}

// The capacity rules an import must not bypass: two drops importing onto a
// one-slot ODP in the same batch is exactly what checkSlots already refuses
// for the drawing UI.
func TestCommitImportEnforcesCapacityAcrossEdgesInTheSameBatch(t *testing.T) {
	s := mappingSetup(t)
	// Both the cabinet and the two ONTs it would feed are new in this same
	// batch - checkSlots' target lookup only counts against a node it can
	// actually resolve to a type, so the drops need real nodes to land on,
	// not only a dangling target name.
	odp := includedNode("ODP-01", models.NodeODP)
	odp.Capacity = 1
	nodes := []ImportedNode{odp, includedNode("ONT-01", models.NodeONT), includedNode("ONT-02", models.NodeONT)}
	edges := []ImportedEdge{
		includedEdge("E-1", "ODP-01", "ONT-01", models.FiberDrop),
		includedEdge("E-2", "ODP-01", "ONT-02", models.FiberDrop),
	}

	_, err := s.CommitImport(nodes, edges)

	require.ErrorIs(t, err, ErrSlotsFull)
	nodesGot, err := s.ListNodes()
	require.NoError(t, err)
	edgesGot, err := s.ListEdges()
	require.NoError(t, err)
	assert.Empty(t, nodesGot, "the whole commit must roll back, including every node the cables would have hung off")
	assert.Empty(t, edgesGot)
}

// Named for what it actually passes: a garbage value, not a blank one - a
// blank fibre type is deliberately allowed (isValidFiberType's own "" case),
// the same as a cable drawn through the map UI with no type chosen yet. A
// test named after the behaviour it does not exercise is worse than no test.
func TestCommitImportRefusesAnInvalidFiberType(t *testing.T) {
	s := mappingSetup(t)
	edge := includedEdge("E-1", "ODC-01", "ODP-01", models.FiberType("bukan-jenis"))

	_, err := s.CommitImport(nil, []ImportedEdge{edge})

	require.ErrorIs(t, err, ErrImportInvalid)
}

func TestCommitImportAcceptsABlankFiberType(t *testing.T) {
	s := mappingSetup(t)
	edge := includedEdge("E-1", "ODC-01", "ODP-01", "")

	result, err := s.CommitImport(nil, []ImportedEdge{edge})

	require.NoError(t, err)
	assert.Equal(t, 1, result.EdgesCreated)
}

// nodesToCreate already re-derives Type, NodeID and every collision from the
// database, and refuses the caller's own Blocked flag, on the stated
// principle that a client is untrusted regardless of what the preview
// showed. Latitude/Longitude were copied straight through with no such
// check - the exact swapped-Jakarta-pair defect ImportNodesTable's own
// Lintang/Bujur columns exist to let a person catch and fix, writable
// again if nothing on the commit side actually enforces it.
func TestCommitImportRefusesOutOfRangeCoordinates(t *testing.T) {
	s := mappingSetup(t)
	node := includedNode("ODP-01", models.NodeODP)
	node.Latitude, node.Longitude = 999, -5000

	_, err := s.CommitImport([]ImportedNode{node}, nil)

	require.ErrorIs(t, err, ErrImportInvalid)
	got, err := s.ListNodes()
	require.NoError(t, err)
	assert.Empty(t, got)
}

// The swapped-pair case named in the review directly: 106.8 is a real,
// in-range-looking longitude sitting in the latitude field - not obviously
// broken the way 999 is, exactly why it is the commonest defect and exactly
// what validateCoordinates' own range check catches regardless.
func TestCommitImportRefusesASwappedLatLngPair(t *testing.T) {
	s := mappingSetup(t)
	node := includedNode("ODP-01", models.NodeODP)
	node.Latitude, node.Longitude = 106.8, -6.2

	_, err := s.CommitImport([]ImportedNode{node}, nil)

	require.ErrorIs(t, err, ErrImportInvalid)
}

func TestCommitImportRefusesNaNCoordinates(t *testing.T) {
	s := mappingSetup(t)
	node := includedNode("ODP-01", models.NodeODP)
	node.Latitude = math.NaN()

	_, err := s.CommitImport([]ImportedNode{node}, nil)

	require.ErrorIs(t, err, ErrImportInvalid)
}
