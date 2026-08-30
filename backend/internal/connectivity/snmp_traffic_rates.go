package connectivity

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// ONUTrafficRates holds what the OLT reports for one ONU's traffic: the live
// Gauge32 rates in bytes per second, and the Counter64 totals beside them.
// RxOctet is the ONU→OLT direction (upload), TxOctet is OLT→ONU (download).
//
// The totals are lifetime counters, so a report of data used over a period is
// the difference between two readings, not a sum of them.
type ONUTrafficRates struct {
	RxOctetBps uint64
	TxOctetBps uint64
	RxOctets   uint64
	TxOctets   uint64
	RxPackets  uint64
	TxPackets  uint64
}

// zteWalkTrafficRates walks the zxAnPonOnuIf{Rx,Tx}OctetRate gauge tables and
// returns per-ONT rates keyed by physical location. The gauges are read
// directly rather than derived from counter deltas because the octet counters
// on C300/C320 are fragment snapshots that oscillate between polls.
func zteWalkTrafficRates(ipAddress, community string, snmpPort int) (map[ONTLocation]ONUTrafficRates, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	rates := make(map[ONTLocation]ONUTrafficRates)

	collect := func(baseOID string, assign func(*ONUTrafficRates, uint64)) error {
		return bulkWalk(client, baseOID, func(pdu gosnmp.SnmpPDU) error {
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

	// A counter walk that fails leaves the rates usable rather than failing the
	// whole poll: the gauges are what the live graphs read.
	if err := collect(OID_ZXGPON_ONU_RX_OCTETS_TABLE, func(r *ONUTrafficRates, v uint64) { r.RxOctets = v }); err != nil {
		log.Printf("[TrafficRates] RX octet counter walk failed: %v", err)
	}
	if err := collect(OID_ZXGPON_ONU_TX_OCTETS_TABLE, func(r *ONUTrafficRates, v uint64) { r.TxOctets = v }); err != nil {
		log.Printf("[TrafficRates] TX octet counter walk failed: %v", err)
	}
	if err := collect(OID_ZXGPON_ONU_RX_PACKETS_TABLE, func(r *ONUTrafficRates, v uint64) { r.RxPackets = v }); err != nil {
		log.Printf("[TrafficRates] RX packet counter walk failed: %v", err)
	}
	if err := collect(OID_ZXGPON_ONU_TX_PACKETS_TABLE, func(r *ONUTrafficRates, v uint64) { r.TxPackets = v }); err != nil {
		log.Printf("[TrafficRates] TX packet counter walk failed: %v", err)
	}

	log.Printf("[TrafficRates] Walked %d ONT rate gauges", len(rates))
	return rates, nil
}

// zteQueryTrafficRates fetches the live rate gauges for a single ONT via SNMP
// GET. The gauge tables are indexed in the ONU-ID space:
// OnuIDIfIndexBase + slot*OnuIDSlotStride + port, followed by the ONT ID.
func zteQueryTrafficRates(ipAddress, community string, snmpPort, slot, port, ontID int) (*ONUTrafficRates, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ifIndex := OnuIDIfIndexBase + slot*OnuIDSlotStride + port
	instance := func(table string) string {
		return fmt.Sprintf("%s.%d.%d", table, ifIndex, ontID)
	}

	rates := &ONUTrafficRates{}
	// The gauges and the lifetime counters are columns of one table, so a single
	// GET fetches both.
	assign := []func(uint64){
		func(v uint64) { rates.RxOctetBps = v },
		func(v uint64) { rates.TxOctetBps = v },
		func(v uint64) { rates.RxOctets = v },
		func(v uint64) { rates.TxOctets = v },
		func(v uint64) { rates.RxPackets = v },
		func(v uint64) { rates.TxPackets = v },
	}
	oids := []string{
		instance(OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE),
		instance(OID_ZXGPON_ONU_TX_OCTET_RATE_TABLE),
		instance(OID_ZXGPON_ONU_RX_OCTETS_TABLE),
		instance(OID_ZXGPON_ONU_TX_OCTETS_TABLE),
		instance(OID_ZXGPON_ONU_RX_PACKETS_TABLE),
		instance(OID_ZXGPON_ONU_TX_PACKETS_TABLE),
	}

	result, err := client.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("rate gauge GET failed: %w", err)
	}

	got := 0
	for i, v := range result.Variables {
		if i >= len(assign) {
			break
		}
		value, ok := toInt64(v.Value)
		if !ok || value < 0 {
			continue
		}
		assign[i](uint64(value))
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
