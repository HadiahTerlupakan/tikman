package connectivity

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// dot1qVlanStaticName is the Q-BRIDGE static VLAN name column. The OID index is
// the VLAN ID itself.
const dot1qVlanStaticName = ".1.3.6.1.2.1.17.7.1.4.3.1.1"

// OLTVLAN is one VLAN the OLT has configured.
type OLTVLAN struct {
	VLANID int    `json:"vlan_id"`
	Name   string `json:"name"`
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

	vlans := make([]OLTVLAN, 0)
	err = client.Walk(dot1qVlanStaticName, func(pdu gosnmp.SnmpPDU) error {
		vlanID, ok := vlanIDFromOID(pdu.Name)
		if !ok {
			return nil
		}
		vlans = append(vlans, OLTVLAN{VLANID: vlanID, Name: printableText(pdu.Value)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("VLAN table walk failed: %w", err)
	}

	sort.Slice(vlans, func(i, j int) bool { return vlans[i].VLANID < vlans[j].VLANID })

	return vlans, nil
}

func vlanIDFromOID(oid string) (int, bool) {
	parts := strings.Split(oid, ".")
	vlanID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || vlanID < 1 || vlanID > 4094 {
		return 0, false
	}
	return vlanID, true
}
