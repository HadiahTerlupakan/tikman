package connectivity

import (
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
)

const (
	sysDescrOID       = ".1.3.6.1.2.1.1.1.0"
	sysNameOID        = ".1.3.6.1.2.1.1.5.0"
	sysUpTimeOID      = ".1.3.6.1.2.1.1.3.0"
	entPhysicalDescr  = ".1.3.6.1.2.1.47.1.1.1.1.2"
	entPhysicalClass  = ".1.3.6.1.2.1.47.1.1.1.1.5"
	entPhysicalSerial = ".1.3.6.1.2.1.47.1.1.1.1.11"
	entPhysicalSWRev  = ".1.3.6.1.2.1.47.1.1.1.1.10"
)

// ChassisEntity is one physical entity the OLT reports through ENTITY-MIB: the
// shelf itself, a line card, a power module or the switch control card. Index
// is the OLT's own entity numbering and is not the slot number: on this C300
// the cards fitted to slots 3 and 4 report as 40 and 50. Slot numbers come from
// the interface names instead, which carry the address the CLI uses.
type ChassisEntity struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Class       int    `json:"class"`
	Serial      string `json:"serial,omitempty"`
	Software    string `json:"software,omitempty"`
}

// OLTSystemInfo is the chassis summary the OLT configuration header shows. It
// comes entirely from standard MIBs, so reading it costs no CLI session.
type OLTSystemInfo struct {
	Description   string          `json:"description"`
	Name          string          `json:"name"`
	UptimeSeconds int64           `json:"uptime_seconds"`
	Entities      []ChassisEntity `json:"entities"`
}

// ReadSystemInfo reads the chassis description, uptime and physical inventory.
// The ZTE C300 exposes no entPhySensorTable, so temperature, CPU and memory are
// deliberately absent here rather than faked: those need the CLI.
func ReadSystemInfo(ipAddress, community string, snmpPort int) (OLTSystemInfo, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return OLTSystemInfo{}, err
	}
	defer func() { _ = client.Conn.Close() }()

	info := OLTSystemInfo{Entities: make([]ChassisEntity, 0)}

	scalars, err := client.Get([]string{sysDescrOID, sysNameOID, sysUpTimeOID})
	if err != nil {
		return OLTSystemInfo{}, fmt.Errorf("system scalar read failed: %w", err)
	}
	for _, pdu := range scalars.Variables {
		switch {
		case strings.HasPrefix(pdu.Name, sysDescrOID):
			info.Description = printableText(pdu.Value)
		case strings.HasPrefix(pdu.Name, sysNameOID):
			info.Name = printableText(pdu.Value)
		case strings.HasPrefix(pdu.Name, sysUpTimeOID):
			// sysUpTime counts hundredths of a second since the last restart.
			if ticks, ok := toInt64(pdu.Value); ok {
				info.UptimeSeconds = ticks / 100
			}
		}
	}

	entities, err := walkEntities(client)
	if err != nil {
		return OLTSystemInfo{}, err
	}
	info.Entities = entities

	return info, nil
}

func walkEntities(client *gosnmp.GoSNMP) ([]ChassisEntity, error) {
	byIndex := make(map[int]*ChassisEntity)

	collect := func(oid string, apply func(entity *ChassisEntity, pdu gosnmp.SnmpPDU)) error {
		return client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
			index, ok := lastOIDSegment(pdu.Name)
			if !ok {
				return nil
			}
			entity, seen := byIndex[index]
			if !seen {
				entity = &ChassisEntity{Index: index}
				byIndex[index] = entity
			}
			apply(entity, pdu)
			return nil
		})
	}

	if err := collect(entPhysicalDescr, func(e *ChassisEntity, pdu gosnmp.SnmpPDU) {
		e.Description = printableText(pdu.Value)
	}); err != nil {
		return nil, fmt.Errorf("entity description walk failed: %w", err)
	}
	if err := collect(entPhysicalClass, func(e *ChassisEntity, pdu gosnmp.SnmpPDU) {
		if class, ok := toInt64(pdu.Value); ok {
			e.Class = int(class)
		}
	}); err != nil {
		return nil, fmt.Errorf("entity class walk failed: %w", err)
	}
	_ = collect(entPhysicalSerial, func(e *ChassisEntity, pdu gosnmp.SnmpPDU) {
		e.Serial = printableText(pdu.Value)
	})
	_ = collect(entPhysicalSWRev, func(e *ChassisEntity, pdu gosnmp.SnmpPDU) {
		e.Software = printableText(pdu.Value)
	})

	entities := make([]ChassisEntity, 0, len(byIndex))
	for _, entity := range byIndex {
		if entity.Description == "" {
			continue
		}
		entities = append(entities, *entity)
	}
	sortEntities(entities)

	return entities, nil
}
