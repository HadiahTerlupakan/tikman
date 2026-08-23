package connectivity

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// UnconfiguredONU is an ONU the OLT detected optically but that has no
// provisioning config yet, so it cannot pass traffic until an operator
// registers its serial number.
type UnconfiguredONU struct {
	Slot            int    `json:"slot"`
	Port            int    `json:"port"`
	SerialNumber    string `json:"serial_number"`
	DeviceType      string `json:"device_type,omitempty"`
	SoftwareVersion string `json:"software_version,omitempty"`
}

// uncfgIndex identifies a row of the autofind table. The trailing sequence
// number is assigned by the OLT per detection, not an ONU ID, so it is only
// used to join columns of the same row and never surfaced.
type uncfgIndex struct {
	slot int
	port int
	seq  int
}

// uncfgScanTimeout bounds the three sequential walks a scan performs. The HTTP
// client gives up at 30s, so the scan has to fail first for the operator to see
// a real error instead of a generic network failure.
const uncfgScanTimeout = 20 * time.Second

// walkUnconfiguredONUs walks the ZTE autofind table and returns the ONUs
// awaiting provisioning. An empty result means every detected ONU is
// configured; it is indistinguishable from the OLT having nothing on its PON
// ports.
func walkUnconfiguredONUs(ctx context.Context, ipAddress, community string, snmpPort int) ([]UnconfiguredONU, error) {
	client, err := newSNMPClientWithContext(ctx, ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	serials := make(map[uncfgIndex]string)
	if err := walkUncfgColumn(client, OnuUncfgSerialNumberPrefix, func(idx uncfgIndex, pdu gosnmp.SnmpPDU) {
		if sn := formatAutofindSerial(pdu.Value); sn != "" {
			serials[idx] = sn
		}
	}); err != nil {
		return nil, fmt.Errorf("autofind serial walk failed: %w", err)
	}

	if len(serials) == 0 {
		log.Printf("[Uncfg] No unconfigured ONUs found on %s", ipAddress)
		return []UnconfiguredONU{}, nil
	}

	// Model and firmware are cosmetic, so a failure here still yields a usable
	// list keyed on the serial the operator needs to provision. A spent deadline
	// is the exception: it means the scan as a whole ran out of time.
	deviceTypes := make(map[uncfgIndex]string)
	if err := walkUncfgColumn(client, OnuUncfgDeviceTypePrefix, func(idx uncfgIndex, pdu gosnmp.SnmpPDU) {
		deviceTypes[idx] = ExtractName(pdu.Value)
	}); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("autofind device type walk failed: %w", err)
		}
		log.Printf("[Uncfg] Device type walk failed: %v", err)
	}

	softwareVersions := make(map[uncfgIndex]string)
	if err := walkUncfgColumn(client, OnuUncfgSoftwareVerPrefix, func(idx uncfgIndex, pdu gosnmp.SnmpPDU) {
		softwareVersions[idx] = ExtractName(pdu.Value)
	}); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("autofind software version walk failed: %w", err)
		}
		log.Printf("[Uncfg] Software version walk failed: %v", err)
	}

	result := make([]UnconfiguredONU, 0, len(serials))
	for idx, serial := range serials {
		result = append(result, UnconfiguredONU{
			Slot:            idx.slot,
			Port:            idx.port,
			SerialNumber:    serial,
			DeviceType:      deviceTypes[idx],
			SoftwareVersion: softwareVersions[idx],
		})
	}

	sortUnconfiguredONUs(result)
	log.Printf("[Uncfg] Found %d unconfigured ONUs on %s", len(result), ipAddress)
	return result, nil
}

// walkUncfgColumn walks one column of the autofind table, decoding each OID
// suffix into an index before handing the value to visit.
func walkUncfgColumn(client *gosnmp.GoSNMP, prefix string, visit func(uncfgIndex, gosnmp.SnmpPDU)) error {
	oid := BaseOID2 + prefix
	base := strings.TrimPrefix(oid, ".") + "."

	return client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		suffix := strings.TrimPrefix(pdu.Name, ".")
		if !strings.HasPrefix(suffix, base) {
			return nil
		}

		idx, ok := parseUncfgSuffix(strings.TrimPrefix(suffix, base))
		if !ok {
			return nil
		}

		visit(idx, pdu)
		return nil
	})
}

// parseUncfgSuffix decodes an "<ifIndex>.<seq>" autofind OID suffix.
func parseUncfgSuffix(suffix string) (uncfgIndex, bool) {
	parts := strings.Split(suffix, ".")
	if len(parts) < 2 {
		return uncfgIndex{}, false
	}

	ifIndex, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return uncfgIndex{}, false
	}

	seq, err := strconv.Atoi(parts[1])
	if err != nil {
		return uncfgIndex{}, false
	}

	slot, port, ok := decodeZxGponIfIndex(uint32(ifIndex))
	if !ok {
		return uncfgIndex{}, false
	}

	return uncfgIndex{slot: slot, port: port, seq: seq}, true
}
