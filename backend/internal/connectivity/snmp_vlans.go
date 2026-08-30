package connectivity

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// Q-BRIDGE static VLAN columns. The OID index is the VLAN ID itself. The two
// port columns are bitmaps: a VLAN's untagged ports are a subset of its egress
// ports, so a port in egress but not untagged is carrying the VLAN tagged.
const (
	dot1qVlanStaticName     = ".1.3.6.1.2.1.17.7.1.4.3.1.1"
	dot1qVlanStaticEgress   = ".1.3.6.1.2.1.17.7.1.4.3.1.2"
	dot1qVlanStaticUntagged = ".1.3.6.1.2.1.17.7.1.4.3.1.4"
)

// bridgePortsPerSlot is the stride ZTE uses to number bridge ports: the ports
// of slot 10 are 641-645 and those of slot 11 are 705-709, so a bridge port is
// slot*64 + port. Confirmed against the interface names the same OLT reports.
const bridgePortsPerSlot = 64

// VLANPort is one port carrying a VLAN, at the rack/slot/port address the CLI
// uses. Tagged distinguishes a trunk member from an access one.
type VLANPort struct {
	Slot   int  `json:"slot"`
	Port   int  `json:"port"`
	Tagged bool `json:"tagged"`
}

// OLTVLAN is one VLAN the OLT has configured, with the ports it is present on.
type OLTVLAN struct {
	VLANID int        `json:"vlan_id"`
	Name   string     `json:"name"`
	Ports  []VLANPort `json:"ports"`
}

// WalkVLANs lists the VLANs configured on an OLT, ordered by VLAN ID. The table
// is standard Q-BRIDGE rather than vendor specific, so this needs no driver:
// the provisioning form wants the same list whatever the model is.
func WalkVLANs(ipAddress, community string, snmpPort int) ([]OLTVLAN, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	byID := make(map[int]*OLTVLAN)
	err = bulkWalk(client, dot1qVlanStaticName, func(pdu gosnmp.SnmpPDU) error {
		vlanID, ok := vlanIDFromOID(pdu.Name)
		if !ok {
			return nil
		}
		byID[vlanID] = &OLTVLAN{VLANID: vlanID, Name: printableText(pdu.Value), Ports: []VLANPort{}}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("VLAN table walk failed: %w", err)
	}

	// A VLAN the name column did not list has no row to attach ports to, so the
	// membership walks only fill in what is already known.
	egress := walkPortBitmaps(client, dot1qVlanStaticEgress, byID)
	untagged := walkPortBitmaps(client, dot1qVlanStaticUntagged, byID)
	for vlanID, vlan := range byID {
		vlan.Ports = mergeVLANPorts(egress[vlanID], untagged[vlanID])
	}

	vlans := make([]OLTVLAN, 0, len(byID))
	for _, vlan := range byID {
		vlans = append(vlans, *vlan)
	}
	sort.Slice(vlans, func(i, j int) bool { return vlans[i].VLANID < vlans[j].VLANID })

	return vlans, nil
}

func walkPortBitmaps(client *gosnmp.GoSNMP, oid string, known map[int]*OLTVLAN) map[int][]int {
	bitmaps := make(map[int][]int, len(known))
	// A device that does not publish the column leaves every VLAN with no port
	// list, which the page renders as "not reported" rather than as an error.
	_ = bulkWalk(client, oid, func(pdu gosnmp.SnmpPDU) error {
		vlanID, ok := vlanIDFromOID(pdu.Name)
		if !ok {
			return nil
		}
		if _, listed := known[vlanID]; !listed {
			return nil
		}
		raw, ok := pdu.Value.([]byte)
		if !ok {
			return nil
		}
		bitmaps[vlanID] = decodePortBitmap(raw)
		return nil
	})
	return bitmaps
}

// decodePortBitmap turns a Q-BRIDGE port list into bridge port numbers. Bit 0
// of the first octet is port 1, most significant bit first.
func decodePortBitmap(raw []byte) []int {
	ports := make([]int, 0)
	for index, octet := range raw {
		for bit := 0; bit < 8; bit++ {
			if octet&(0x80>>bit) != 0 {
				ports = append(ports, index*8+bit+1)
			}
		}
	}
	return ports
}

func mergeVLANPorts(egress, untagged []int) []VLANPort {
	access := make(map[int]bool, len(untagged))
	for _, port := range untagged {
		access[port] = true
	}

	ports := make([]VLANPort, 0, len(egress))
	for _, bridgePort := range egress {
		slot, port := bridgePortAddress(bridgePort)
		ports = append(ports, VLANPort{Slot: slot, Port: port, Tagged: !access[bridgePort]})
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Slot != ports[j].Slot {
			return ports[i].Slot < ports[j].Slot
		}
		return ports[i].Port < ports[j].Port
	})
	return ports
}

func bridgePortAddress(bridgePort int) (slot, port int) {
	return bridgePort / bridgePortsPerSlot, (bridgePort-1)%bridgePortsPerSlot + 1
}

func vlanIDFromOID(oid string) (int, bool) {
	parts := strings.Split(oid, ".")
	vlanID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || vlanID < 1 || vlanID > 4094 {
		return 0, false
	}
	return vlanID, true
}
