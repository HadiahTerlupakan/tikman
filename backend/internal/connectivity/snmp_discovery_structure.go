package connectivity

import (
	"sort"
	"time"
)

// buildTopologyStructure assembles the hierarchical topology from attribute maps.
func buildTopologyStructure(slotMap map[int]map[int][]ONTLocation, statuses map[ONTLocation]int,
	serials, names, descriptions, deviceTypes, hwVersions, swVersions, ipAddresses, macAddresses map[ONTLocation]string,
	metrics map[ONTLocation]*ONTMetrics) []GPONSlot {

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
				case 3:
					statusStr = "online"
				case 4:
					statusStr = "dying_gasp"
				case 6:
					statusStr = "offline"
				case 1:
					statusStr = "los"
				}

				ont := DiscoveredONT{
					PortID:          loc.Port,
					ONTID:           loc.ONTID,
					SerialNumber:    serials[loc],
					RunState:        mapPhaseToRunState(statuses[loc]),
					Name:            names[loc],
					Description:     descriptions[loc],
					DeviceType:      deviceTypes[loc],
					HardwareVersion: hwVersions[loc],
					SoftwareVersion: swVersions[loc],
					IPAddress:       ipAddresses[loc],
					MACAddress:      macAddresses[loc],
					Status:          statusStr,
					LastSeenAt:      time.Now(),
				}

				if metric, ok := metrics[loc]; ok && metric != nil {
					if metric.RxPower != nil || metric.TxPower != nil || metric.Distance > 0 {
						ont.RxPower = metric.RxPower
						ont.TxPower = metric.TxPower
						ont.Distance = metric.Distance
					}
					ont.RxBytes = metric.RxBytes
					ont.TxBytes = metric.TxBytes
					ont.RxPackets = metric.RxPackets
					ont.TxPackets = metric.TxPackets
					ont.RxErrors = metric.RxErrors
					ont.TxErrors = metric.TxErrors
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
