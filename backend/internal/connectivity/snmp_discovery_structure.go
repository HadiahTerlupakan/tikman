package connectivity

import (
	"sort"
	"time"
)

// buildTopologyStructure assembles the hierarchical topology from the status,
// inventory and metric maps collected by a driver.
func buildTopologyStructure(slotMap map[int]map[int][]ONTLocation, statuses map[ONTLocation]int,
	inventory map[ONTLocation]ONTInventory, metrics map[ONTLocation]ONTMetrics) []GPONSlot {

	slots := make([]int, 0, len(slotMap))
	for slot := range slotMap {
		slots = append(slots, slot)
	}
	sort.Ints(slots)

	topology := make([]GPONSlot, 0, len(slots))
	for _, slot := range slots {
		gponSlot := GPONSlot{Slot: slot}

		ports := make([]int, 0, len(slotMap[slot]))
		for port := range slotMap[slot] {
			ports = append(ports, port)
		}
		sort.Ints(ports)

		for _, port := range ports {
			onts := make([]DiscoveredONT, 0, len(slotMap[slot][port]))
			for _, loc := range slotMap[slot][port] {
				statusStr := "unknown"
				switch statuses[loc] {
				case PhaseStateOnline:
					statusStr = "online"
				case PhaseStateDyingGasp:
					statusStr = "dying_gasp"
				case PhaseStateOffline:
					statusStr = "offline"
				case PhaseStateLOS:
					statusStr = "los"
				}

				inv := inventory[loc]
				ont := DiscoveredONT{
					// Dropping the slot here left the worker's sync path storing
					// ONTs without one. The auto ONU ID allocator matches on it,
					// and the configure form fills its Card from it.
					Slot:            loc.Slot,
					PortID:          loc.Port,
					ONTID:           loc.ONTID,
					SerialNumber:    inv.SerialNumber,
					RunState:        mapPhaseToRunState(statuses[loc]),
					Name:            inv.Name,
					Description:     inv.Description,
					DeviceType:      inv.DeviceType,
					HardwareVersion: inv.HardwareVersion,
					SoftwareVersion: inv.SoftwareVersion,
					IPAddress:       inv.IPAddress,
					MACAddress:      inv.MACAddress,
					Status:          statusStr,
					LastSeenAt:      time.Now(),
				}

				// A reading is only attached when the ONT actually reported one:
				// an ONT with no optical values must not surface as 0 dBm at 0 m.
				if m, ok := metrics[loc]; ok && (m.RxPower != nil || m.TxPower != nil || m.Distance > 0) {
					ont.RxPower = m.RxPower
					ont.TxPower = m.TxPower
					ont.Distance = m.Distance
					ont.RxBytes = m.RxBytes
					ont.TxBytes = m.TxBytes
					ont.RxPackets = m.RxPackets
					ont.TxPackets = m.TxPackets
					ont.RxErrors = m.RxErrors
					ont.TxErrors = m.TxErrors
				}

				onts = append(onts, ont)
			}
			gponSlot.Ports = append(gponSlot.Ports, GponPort{
				PortID: port,
				ONTs:   onts,
			})
		}

		topology = append(topology, gponSlot)
	}

	return topology
}
