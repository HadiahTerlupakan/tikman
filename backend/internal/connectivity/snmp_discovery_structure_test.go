package connectivity

import "testing"

func TestBuildTopologyStructure(t *testing.T) {
	tests := []struct {
		name        string
		slotMap     map[int]map[int][]ONTLocation
		statuses    map[ONTLocation]int
		inventory   map[ONTLocation]ONTInventory
		metrics     map[ONTLocation]ONTMetrics
		wantSlots   int
		wantTotalON int
	}{
		{
			name:      "empty input returns empty structure",
			slotMap:   map[int]map[int][]ONTLocation{},
			statuses:  map[ONTLocation]int{},
			inventory: map[ONTLocation]ONTInventory{},
			metrics:   map[ONTLocation]ONTMetrics{},
			wantSlots: 0,
		},
		{
			name: "single slot single port single ONT",
			slotMap: map[int]map[int][]ONTLocation{
				3: {
					1: []ONTLocation{{Slot: 3, Port: 1, ONTID: 1}},
				},
			},
			statuses: map[ONTLocation]int{
				{Slot: 3, Port: 1, ONTID: 1}: 1,
			},
			inventory: map[ONTLocation]ONTInventory{
				{Slot: 3, Port: 1, ONTID: 1}: {}, // empty inventory is fine
			},
			metrics:     map[ONTLocation]ONTMetrics{},
			wantSlots:   1,
			wantTotalON: 1,
		},
		{
			name: "multiple slots multiple ports",
			slotMap: map[int]map[int][]ONTLocation{
				1: {
					1: []ONTLocation{{Slot: 1, Port: 1, ONTID: 1}},
					2: []ONTLocation{{Slot: 1, Port: 2, ONTID: 2}},
				},
				3: {
					1: []ONTLocation{{Slot: 3, Port: 1, ONTID: 3}},
				},
			},
			statuses: map[ONTLocation]int{
				{Slot: 1, Port: 1, ONTID: 1}: 1,
				{Slot: 1, Port: 2, ONTID: 2}: 0,
				{Slot: 3, Port: 1, ONTID: 3}: 1,
			},
			inventory: map[ONTLocation]ONTInventory{
				{Slot: 1, Port: 1, ONTID: 1}: {},
				{Slot: 1, Port: 2, ONTID: 2}: {},
				{Slot: 3, Port: 1, ONTID: 3}: {},
			},
			metrics:     map[ONTLocation]ONTMetrics{},
			wantSlots:   2,
			wantTotalON: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTopologyStructure(tt.slotMap, tt.statuses, tt.inventory, tt.metrics)
			if len(result) != tt.wantSlots {
				t.Errorf("slot count: got %d, want %d", len(result), tt.wantSlots)
			}
			totalON := 0
			for _, slot := range result {
				for _, port := range slot.Ports {
					totalON += len(port.ONTs)
				}
			}
			if totalON != tt.wantTotalON {
				t.Errorf("total ONTs: got %d, want %d", totalON, tt.wantTotalON)
			}
		})
	}
}

// The worker's sync path flattens this topology straight into ONT rows. Losing
// the slot there stored ONTs without one, which the auto ONU ID allocator
// cannot see and the configure form filled its Card from.
func TestBuildTopologyStructureKeepsTheSlot(t *testing.T) {
	loc := ONTLocation{Slot: 3, Port: 1, ONTID: 15}
	slotMap := map[int]map[int][]ONTLocation{3: {1: {loc}}}

	topology := buildTopologyStructure(
		slotMap,
		map[ONTLocation]int{loc: PhaseStateOnline},
		map[ONTLocation]ONTInventory{loc: {SerialNumber: "HWTCB403E8A0"}},
		map[ONTLocation]ONTMetrics{},
	)

	if len(topology) != 1 || len(topology[0].Ports) != 1 || len(topology[0].Ports[0].ONTs) != 1 {
		t.Fatalf("unexpected topology shape: %+v", topology)
	}
	if got := topology[0].Ports[0].ONTs[0].Slot; got != 3 {
		t.Errorf("ONT slot = %d, want 3", got)
	}
}
