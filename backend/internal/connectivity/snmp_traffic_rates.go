package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// ONUTrafficRates holds live traffic rates reported by the OLT as Gauge32
// values in bytes per second. RxOctet is the ONU→OLT direction (upload),
// TxOctet is OLT→ONU (download).
type ONUTrafficRates struct {
	RxOctetBps uint64
	TxOctetBps uint64
}

// WalkONUTrafficRates walks the zxAnPonOnuIf{Rx,Tx}OctetRate gauge tables and
// returns per-ONT rates keyed by physical location. The gauges are read
// directly rather than derived from counter deltas because the octet counters
// on C300/C320 are fragment snapshots that oscillate between polls.
func WalkONUTrafficRates(ipAddress, community string, snmpPort int) (map[ONTLocation]ONUTrafficRates, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	rates := make(map[ONTLocation]ONUTrafficRates)

	collect := func(baseOID string, assign func(*ONUTrafficRates, uint64)) error {
		return client.Walk(baseOID, func(pdu gosnmp.SnmpPDU) error {
			loc, ok := parseOnuIDSuffix(pdu.Name, baseOID)
			if !ok {
				return nil
			}
			value, ok := toInt64(pdu.Value)
			if !ok || value < 0 {
				return nil
			}
			r := rates[loc]
			assign(&r, uint64(value))
			rates[loc] = r
			return nil
		})
	}

	if err := collect(OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE, func(r *ONUTrafficRates, v uint64) { r.RxOctetBps = v }); err != nil {
		return nil, fmt.Errorf("rx octet rate walk failed: %w", err)
	}
	if err := collect(OID_ZXGPON_ONU_TX_OCTET_RATE_TABLE, func(r *ONUTrafficRates, v uint64) { r.TxOctetBps = v }); err != nil {
		return nil, fmt.Errorf("tx octet rate walk failed: %w", err)
	}

	log.Printf("[TrafficRates] Walked %d ONT rate gauges", len(rates))
	return rates, nil
}

// QueryONUTrafficRates fetches the live rate gauges for a single ONT via SNMP
// GET. The gauge tables are indexed in the ONU-ID space:
// OnuIDIfIndexBase + slot*OnuIDSlotStride + port, followed by the ONT ID.
func QueryONUTrafficRates(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONUTrafficRates, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ifIndex := OnuIDIfIndexBase + slot*OnuIDSlotStride + port
	rxOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE, ifIndex, ontID)
	txOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_TX_OCTET_RATE_TABLE, ifIndex, ontID)

	result, err := client.Get([]string{rxOID, txOID})
	if err != nil {
		return nil, fmt.Errorf("rate gauge GET failed: %w", err)
	}

	rates := &ONUTrafficRates{}
	got := 0
	for i, v := range result.Variables {
		value, ok := toInt64(v.Value)
		if !ok || value < 0 {
			continue
		}
		if i == 0 {
			rates.RxOctetBps = uint64(value)
		} else {
			rates.TxOctetBps = uint64(value)
		}
		got++
	}
	if got == 0 {
		return nil, fmt.Errorf("no rate gauges returned for slot %d port %d onu %d", slot, port, ontID)
	}

	return rates, nil
}

// parseOnuIDSuffix decodes an OID suffix in the ONU-ID index space
// (0x1101SSPP), used by the octet-rate gauge tables. Unlike
// parseZxGponSuffix it accepts the 0x11 frame marker and applies no
// optical-sentinel filtering, since gauge rates legitimately exceed 30000.
func parseOnuIDSuffix(oid, base string) (ONTLocation, bool) {
	trimmed := strings.TrimPrefix(oid, ".")
	baseTrimmed := strings.TrimPrefix(base, ".")
	if !strings.HasPrefix(trimmed, baseTrimmed+".") {
		return ONTLocation{}, false
	}

	parts := strings.Split(strings.TrimPrefix(trimmed, baseTrimmed+"."), ".")
	if len(parts) < 2 {
		return ONTLocation{}, false
	}

	ifIndexStr, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return ONTLocation{}, false
	}
	onuIndex, err := strconv.Atoi(parts[1])
	if err != nil {
		return ONTLocation{}, false
	}

	if ifIndexStr < OnuIDIfIndexBase {
		return ONTLocation{}, false
	}
	offset := uint32(ifIndexStr) - OnuIDIfIndexBase
	slot := int(offset / OnuIDSlotStride)
	port := int(offset % OnuIDSlotStride)
	if slot == 0 || port == 0 {
		return ONTLocation{}, false
	}

	return ONTLocation{Slot: slot, Port: port, ONTID: onuIndex}, true
}
