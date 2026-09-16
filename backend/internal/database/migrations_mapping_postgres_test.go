package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
