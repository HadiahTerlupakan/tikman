package connectivity

import (
	"context"
	"fmt"
	"sort"

	"github.com/gosnmp/gosnmp"
)

// IF-MIB columns a switch port page shows beside the traffic WalkIfCounters
// reads. The error and discard counters are ifTable's 32-bit ones: errors do
// not climb fast enough to wrap within a poll, and ifXTable has no 64-bit
// version of them.
const (
	oidIfAlias       = "1.3.6.1.2.1.31.1.1.1.18"
	oidIfAdminStatus = "1.3.6.1.2.1.2.2.1.7"
	oidIfInDiscards  = "1.3.6.1.2.1.2.2.1.13"
	oidIfInErrors    = "1.3.6.1.2.1.2.2.1.14"
	oidIfOutErrors   = "1.3.6.1.2.1.2.2.1.20"
	ifAdminStatusUp  = 1
)

// IfState is one interface's description, admin state and error counters.
type IfState struct {
	IfIndex    int
	Alias      string
	AdminUp    bool
	InErrors   uint64
	OutErrors  uint64
	InDiscards uint64
}

type ifStateColumn struct {
	oid string
	set func(row *IfState, pdu gosnmp.SnmpPDU)
}

var ifStateColumns = []ifStateColumn{
	{oidIfAlias, func(row *IfState, pdu gosnmp.SnmpPDU) { row.Alias = pduText(pdu) }},
	{oidIfAdminStatus, func(row *IfState, pdu gosnmp.SnmpPDU) {
		status, _ := toInt64(pdu.Value)
		row.AdminUp = status == ifAdminStatusUp
	}},
	{oidIfInErrors, func(row *IfState, pdu gosnmp.SnmpPDU) { row.InErrors = pduUint(pdu) }},
	{oidIfOutErrors, func(row *IfState, pdu gosnmp.SnmpPDU) { row.OutErrors = pduUint(pdu) }},
	{oidIfInDiscards, func(row *IfState, pdu gosnmp.SnmpPDU) { row.InDiscards = pduUint(pdu) }},
}

// WalkIfState reads alias, admin status, errors and inbound discards of every
// interface, ordered by ifIndex.
func WalkIfState(ctx context.Context, host string, port int, community string) ([]IfState, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout

	rows := map[int]*IfState{}
	for _, column := range ifStateColumns {
		err := bulkWalk(client, column.oid, func(pdu gosnmp.SnmpPDU) error {
			index, err := lastSubidentifier(pdu.Name)
			if err != nil {
				return err
			}
			row, ok := rows[index]
			if !ok {
				row = &IfState{IfIndex: index}
				rows[index] = row
			}
			column.set(row, pdu)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", column.oid, err)
		}
	}

	out := make([]IfState, 0, len(rows))
	for _, row := range rows {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IfIndex < out[j].IfIndex })
	return out, nil
}
