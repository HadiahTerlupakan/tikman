package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func plantNode(t *testing.T, s *MappingService, id string, kind models.NodeType, capacity int) {
	t.Helper()
	_, err := s.CreateNode(models.MappingNode{
		NodeID: id, Type: kind, Name: id,
		Latitude: -6.2, Longitude: 106.8, Capacity: capacity,
	})
	require.NoError(t, err)
}

func drawCable(s *MappingService, id, from, to string, kind models.FiberType) error {
	_, err := s.CreateEdge(models.MappingEdge{
		EdgeID: id, Source: from, Target: to, FiberType: kind,
	})
	return err
}

// A cabinet with eight slots holds eight boxes. The ninth is a mistake worth
// catching at the moment it is drawn, not at the moment someone visits the site.
func TestACabinetRefusesOneBoxPastItsSlots(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 8)
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 8)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}
	plantNode(t, s, "ODP-09", models.NodeODP, 8)

	err := drawCable(s, "E-ODP-09", "ODC-01", "ODP-09", models.FiberDistribution)

	require.ErrorIs(t, err, ErrSlotsFull)
	assert.Contains(t, err.Error(), "8/8", "the refusal has to say how full it is")
}

// Cascaded boxes are counted against their own kind, so feeding one cabinet
// from another does not eat the slots meant for distribution boxes.
func TestACascadeIsCountedApartFromOrdinaryDrops(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODP-01", models.NodeODP, 1)
	plantNode(t, s, "ONT-01", models.NodeONT, 0)
	plantNode(t, s, "ODP-02", models.NodeODP, 1)

	require.NoError(t, drawCable(s, "E-1", "ODP-01", "ONT-01", models.FiberDrop))

	err := drawCable(s, "E-2", "ODP-01", "ODP-02", models.FiberODPToODP)

	require.NoError(t, err, "a cascade does not consume the drop slot")
}

// Nobody counts the ports on every box the day they place it.
func TestACapacityOfZeroNeverRefuses(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 0)
	for i := 1; i <= 20; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 0)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}
}

func TestASecondCableWithTheSameIDIsRefused(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 0)
	plantNode(t, s, "ODP-01", models.NodeODP, 0)
	require.NoError(t, drawCable(s, "E-1", "ODC-01", "ODP-01", models.FiberDistribution))

	err := drawCable(s, "E-1", "ODC-01", "ODP-01", models.FiberDistribution)

	require.ErrorIs(t, err, ErrEdgeExists)
}

// UpdateEdge can repoint a cable at a different box entirely, so the same
// capacity rule that guards CreateEdge has to guard a move too — otherwise
// re-pointing an existing cable is a free pass around the slot count.
func TestMovingACableOntoAFullODCIsRefused(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 8)
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 8)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}
	plantNode(t, s, "ODC-02", models.NodeODC, 8)
	plantNode(t, s, "ODP-99", models.NodeODP, 8)
	require.NoError(t, drawCable(s, "E-MOVE", "ODC-02", "ODP-99", models.FiberDistribution))

	_, err := s.UpdateEdge("E-MOVE", models.MappingEdge{
		EdgeID: "E-MOVE", Source: "ODC-01", Target: "ODP-99", FiberType: models.FiberDistribution,
	})

	require.ErrorIs(t, err, ErrSlotsFull)
}

// The cable being edited is already one of the slots counted against its own
// box. Excluding anything but itself would refuse a notes-only edit of a box
// that is legitimately full, forever.
func TestANotesOnlyEditOfACableOnAFullODCSucceeds(t *testing.T) {
	s := mappingSetup(t)
	plantNode(t, s, "ODC-01", models.NodeODC, 8)
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("ODP-%02d", i)
		plantNode(t, s, id, models.NodeODP, 8)
		require.NoError(t, drawCable(s, "E-"+id, "ODC-01", id, models.FiberDistribution))
	}

	_, err := s.UpdateEdge("E-ODP-01", models.MappingEdge{
		EdgeID: "E-ODP-01", Source: "ODC-01", Target: "ODP-01",
		FiberType: models.FiberDistribution, Notes: "cek ulang redaman",
	})

	require.NoError(t, err, "a notes-only edit must not be blocked by its own slot")
}

// A delete that finds nothing is how a second editor learns the cable is
// already gone — the API layer branches on this exact sentinel to answer 404.
// If this ever silently returned nil, the UI would report a successful delete
// of a cable that was never there.
func TestDeletingACableThatIsNotThereSaysSo(t *testing.T) {
	s := mappingSetup(t)

	err := s.DeleteEdge("E-TIDAK-ADA")

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
