package connectivity

import (
	"fmt"
	"sort"

	"github.com/gosnmp/gosnmp"
)

// ZTE indexes these tables by rack.shelf.slot, so the OID tail is 1.1.<slot> on
// a single-shelf chassis. They are enterprise tables rather than the standard
// entPhySensorTable, which this C300 leaves empty.
const (
	zteCardTemperature = ".1.3.6.1.4.1.3902.1015.2.1.3.13.5.1.1"
	zteCardCPUPercent  = ".1.3.6.1.4.1.3902.1015.2.1.1.3.1.9"
	zteCardMemPercent  = ".1.3.6.1.4.1.3902.1015.2.1.1.3.1.11"
)

// noTemperatureReading is what a slot with no card, or a card that is down,
// reports instead of a temperature.
const noTemperatureReading = -1000

// CardHealth is one slot's temperature and load. A field is nil when the OLT
// has no reading for it, which is not the same as zero: an empty slot reports
// 0% CPU exactly like an idle card would.
type CardHealth struct {
	Slot          int  `json:"slot"`
	TemperatureC  *int `json:"temperature_c,omitempty"`
	CPUPercent    *int `json:"cpu_percent,omitempty"`
	MemoryPercent *int `json:"memory_percent,omitempty"`
}

// WalkCardHealth reads the temperature, CPU and memory of every slot the OLT
// reports. Everything here is SNMP: the equivalent CLI reads (show processor)
// agree with it, so the configuration page costs no CLI session.
func WalkCardHealth(ipAddress, community string, snmpPort int) ([]CardHealth, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	bySlot := make(map[int]*CardHealth)
	get := func(slot int) *CardHealth {
		health, seen := bySlot[slot]
		if !seen {
			health = &CardHealth{Slot: slot}
			bySlot[slot] = health
		}
		return health
	}

	collect := func(oid string, apply func(health *CardHealth, value int)) error {
		return bulkWalk(client, oid, func(pdu gosnmp.SnmpPDU) error {
			slot, ok := lastOIDSegment(pdu.Name)
			if !ok {
				return nil
			}
			value, ok := toInt64(pdu.Value)
			if !ok {
				return nil
			}
			apply(get(slot), int(value))
			return nil
		})
	}

	if err := collect(zteCardTemperature, func(h *CardHealth, v int) {
		if v != noTemperatureReading {
			h.TemperatureC = &v
		}
	}); err != nil {
		return nil, fmt.Errorf("card temperature walk failed: %w", err)
	}
	if err := collect(zteCardCPUPercent, func(h *CardHealth, v int) {
		h.CPUPercent = &v
	}); err != nil {
		return nil, fmt.Errorf("card CPU walk failed: %w", err)
	}
	if err := collect(zteCardMemPercent, func(h *CardHealth, v int) {
		h.MemoryPercent = &v
	}); err != nil {
		return nil, fmt.Errorf("card memory walk failed: %w", err)
	}

	health := make([]CardHealth, 0, len(bySlot))
	for _, entry := range bySlot {
		health = append(health, *entry)
	}
	sort.Slice(health, func(i, j int) bool { return health[i].Slot < health[j].Slot })

	return health, nil
}
