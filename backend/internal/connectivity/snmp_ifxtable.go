package connectivity

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// IF-MIB columns read for every interface. ifXTable's 64-bit counters, not
// ifTable's 32-bit ones: a 32-bit octet counter wraps in under four seconds at
// 10 Gbps, so a one-minute poll could not tell how many times it had.
const (
	oidIfName        = "1.3.6.1.2.1.31.1.1.1.1"
	oidIfHCInOctets  = "1.3.6.1.2.1.31.1.1.1.6"
	oidIfHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10"
	oidIfHighSpeed   = "1.3.6.1.2.1.31.1.1.1.15"
	oidIfOperStatus  = "1.3.6.1.2.1.2.2.1.8"
	ifOperStatusUp   = 1
)

// ifCounterRequestTimeout is the per-request SNMP timeout for a router. A
// router answers in milliseconds; the walk is bounded as a whole by the
// caller's context.
const ifCounterRequestTimeout = 2 * time.Second

// IfCounters is one interface's row of IF-MIB.
type IfCounters struct {
	IfIndex   int
	Name      string
	InOctets  uint64
	OutOctets uint64
	SpeedMbps uint64
	OperUp    bool
}

type ifColumn struct {
	oid string
	set func(row *IfCounters, pdu gosnmp.SnmpPDU)
}

var ifColumns = []ifColumn{
	{oidIfName, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.Name = pduText(pdu) }},
	{oidIfHCInOctets, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.InOctets = pduUint(pdu) }},
	{oidIfHCOutOctets, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.OutOctets = pduUint(pdu) }},
	{oidIfHighSpeed, func(row *IfCounters, pdu gosnmp.SnmpPDU) { row.SpeedMbps = pduUint(pdu) }},
	{oidIfOperStatus, func(row *IfCounters, pdu gosnmp.SnmpPDU) {
		status, _ := toInt64(pdu.Value)
		row.OperUp = status == ifOperStatusUp
	}},
}

// WalkIfCounters reads name, 64-bit octet counters, speed and operational
// status of every interface, ordered by ifIndex.
func WalkIfCounters(ctx context.Context, host string, port int, community string) ([]IfCounters, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout

	rows := map[int]*IfCounters{}
	for _, column := range ifColumns {
		err := bulkWalk(client, column.oid, func(pdu gosnmp.SnmpPDU) error {
			index, err := lastSubidentifier(pdu.Name)
			if err != nil {
				return err
			}
			row, ok := rows[index]
			if !ok {
				row = &IfCounters{IfIndex: index}
				rows[index] = row
			}
			column.set(row, pdu)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", column.oid, err)
		}
	}

	out := make([]IfCounters, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IfIndex < out[j].IfIndex })
	return out, nil
}

func lastSubidentifier(oid string) (int, error) {
	index, err := strconv.Atoi(oid[strings.LastIndex(oid, ".")+1:])
	if err != nil {
		return 0, fmt.Errorf("OID %s has no numeric index", oid)
	}
	return index, nil
}

func pduText(pdu gosnmp.SnmpPDU) string {
	if raw, ok := pdu.Value.([]byte); ok {
		return string(raw)
	}
	return fmt.Sprint(pdu.Value)
}

// pduUint reads a Counter64 without going through int64, which would turn a
// counter past 2^63 negative.
func pduUint(pdu gosnmp.SnmpPDU) uint64 {
	value := gosnmp.ToBigInt(pdu.Value)
	if value.Sign() < 0 || value.Cmp(new(big.Int).SetUint64(^uint64(0))) > 0 {
		return 0
	}
	return value.Uint64()
}
