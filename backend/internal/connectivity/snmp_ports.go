package connectivity

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// ifNameOID lives in hsgq_probe.go, which resolves ONU ifIndexes from the same
// interface naming.
const (
	ifAdminStatusOID = ".1.3.6.1.2.1.2.2.1.7"
	ifOperStatusOID  = ".1.3.6.1.2.1.2.2.1.8"
)

// Port kinds, taken from the interface name prefix the OLT reports.
const (
	PortKindPON    = "pon"
	PortKindUplink = "uplink"
	PortKindOther  = "other"
)

// ifStatusUp is the IF-MIB value shared by ifAdminStatus and ifOperStatus.
const ifStatusUp = 1

// OLTPort is one physical interface on the OLT. Rack, Shelf and Slot are parsed
// out of the name the device reports (gpon_1/3/1, gei_1/1/1), which is the same
// address the CLI uses, so a port here can be matched to a card without a
// second lookup.
type OLTPort struct {
	IfIndex     int    `json:"if_index"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Rack        int    `json:"rack"`
	Slot        int    `json:"slot"`
	Port        int    `json:"port"`
	AdminUp     bool   `json:"admin_up"`
	OperUp      bool   `json:"oper_up"`
	AdminStatus int    `json:"admin_status"`
	OperStatus  int    `json:"oper_status"`
}

// WalkPorts lists the OLT's physical interfaces with their admin and
// operational state. IF-MIB is standard, so this needs no driver and no CLI
// session; ifDescr is empty on the C300, which is why ifName is used instead.
func WalkPorts(ipAddress, community string, snmpPort int) ([]OLTPort, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	byIndex := make(map[int]*OLTPort)
	get := func(index int) *OLTPort {
		port, seen := byIndex[index]
		if !seen {
			port = &OLTPort{IfIndex: index}
			byIndex[index] = port
		}
		return port
	}

	err = bulkWalk(client, ifNameOID, func(pdu gosnmp.SnmpPDU) error {
		index, ok := lastOIDSegment(pdu.Name)
		if !ok {
			return nil
		}
		name := printableText(pdu.Value)
		if name == "" {
			return nil
		}
		port := get(index)
		port.Name = name
		port.Kind, port.Rack, port.Slot, port.Port = parsePortName(name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("interface name walk failed: %w", err)
	}

	walkStatus := func(oid string, apply func(port *OLTPort, value int)) error {
		return bulkWalk(client, oid, func(pdu gosnmp.SnmpPDU) error {
			index, ok := lastOIDSegment(pdu.Name)
			if !ok {
				return nil
			}
			// Only interfaces that reported a name are kept; the C300 lists
			// internal indexes here that have no physical port behind them.
			port, named := byIndex[index]
			if !named {
				return nil
			}
			if value, ok := toInt64(pdu.Value); ok {
				apply(port, int(value))
			}
			return nil
		})
	}

	if err := walkStatus(ifAdminStatusOID, func(p *OLTPort, v int) {
		p.AdminStatus, p.AdminUp = v, v == ifStatusUp
	}); err != nil {
		return nil, fmt.Errorf("interface admin status walk failed: %w", err)
	}
	if err := walkStatus(ifOperStatusOID, func(p *OLTPort, v int) {
		p.OperStatus, p.OperUp = v, v == ifStatusUp
	}); err != nil {
		return nil, fmt.Errorf("interface operational status walk failed: %w", err)
	}

	ports := make([]OLTPort, 0, len(byIndex))
	for _, port := range byIndex {
		ports = append(ports, *port)
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Slot != ports[j].Slot {
			return ports[i].Slot < ports[j].Slot
		}
		if ports[i].Port != ports[j].Port {
			return ports[i].Port < ports[j].Port
		}
		return ports[i].Name < ports[j].Name
	})

	return ports, nil
}

// parsePortName splits an interface name such as gpon_1/3/1 into its kind and
// rack/slot/port address. A name in any other shape is reported as-is with a
// zero address rather than dropped, so nothing the OLT lists goes missing.
func parsePortName(name string) (kind string, rack, slot, port int) {
	kind = PortKindOther
	prefix, address, found := strings.Cut(name, "_")
	if !found {
		return kind, 0, 0, 0
	}

	switch {
	case strings.EqualFold(prefix, "gpon"), strings.EqualFold(prefix, "epon"):
		kind = PortKindPON
	case strings.EqualFold(prefix, "gei"), strings.EqualFold(prefix, "xgei"),
		strings.EqualFold(prefix, "smartgroup"):
		kind = PortKindUplink
	}

	parts := strings.Split(address, "/")
	if len(parts) != 3 {
		return kind, 0, 0, 0
	}
	rack, _ = strconv.Atoi(parts[0])
	slot, _ = strconv.Atoi(parts[1])
	port, _ = strconv.Atoi(parts[2])

	return kind, rack, slot, port
}

func lastOIDSegment(oid string) (int, bool) {
	index := strings.LastIndex(oid, ".")
	if index < 0 {
		return 0, false
	}
	value, err := strconv.Atoi(oid[index+1:])
	if err != nil {
		return 0, false
	}
	return value, true
}

func sortEntities(entities []ChassisEntity) {
	sort.Slice(entities, func(i, j int) bool { return entities[i].Index < entities[j].Index })
}
