package connectivity

import (
	"time"
)

// DiscoveredONT represents an ONT discovered via SNMP with full details
type DiscoveredONT struct {
	// Slot completes the ONT's address. Without it a discovered ONT cannot be
	// matched against a card, and the auto ONU ID allocator — which looks up
	// used IDs by slot, port and OLT — cannot see it.
	Slot            int       `json:"slot"`
	PortID          int       `json:"port_id"`
	ONTID           int       `json:"ont_id"`
	SerialNumber    string    `json:"serial_number"`
	RunState        int       `json:"run_state"`
	Name            string    `json:"name,omitempty"`
	Description     string    `json:"description,omitempty"`
	DeviceType      string    `json:"device_type,omitempty"`
	HardwareVersion string    `json:"hardware_version,omitempty"`
	SoftwareVersion string    `json:"software_version,omitempty"`
	IPAddress       string    `json:"ip_address,omitempty"`
	MACAddress      string    `json:"mac_address,omitempty"`
	RxPower         *float64  `json:"rx_power,omitempty"`
	TxPower         *float64  `json:"tx_power,omitempty"`
	Distance        int       `json:"distance,omitempty"`
	Status          string    `json:"status,omitempty"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	// Traffic statistics
	RxBytes   uint64 `json:"rx_bytes,omitempty"`
	TxBytes   uint64 `json:"tx_bytes,omitempty"`
	RxPackets uint64 `json:"rx_packets,omitempty"`
	TxPackets uint64 `json:"tx_packets,omitempty"`
	RxErrors  uint64 `json:"rx_errors,omitempty"`
	TxErrors  uint64 `json:"tx_errors,omitempty"`
}

// DiscoverONTs retrieves all ONTs on an OLT flattened into a single list.
func DiscoverONTs(driver Driver, ipAddress, community string, port int) ([]DiscoveredONT, error) {
	topology, err := DiscoverOLTTopology(driver, ipAddress, community, port)
	if err != nil {
		return nil, err
	}

	var result []DiscoveredONT
	for _, slot := range topology {
		for _, gponPort := range slot.Ports {
			result = append(result, gponPort.ONTs...)
		}
	}

	return result, nil
}
