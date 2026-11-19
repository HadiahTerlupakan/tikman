package connectivity

import (
	"context"
	"fmt"
	"sort"

	"github.com/gosnmp/gosnmp"
)

// MIKROTIK-MIB mtxrGaugeTable columns. The table holds what
// `/system health print` shows, one row per sensor, named by the router.
const (
	oidMtxrGaugeName  = "1.3.6.1.4.1.14988.1.1.3.100.1.2"
	oidMtxrGaugeValue = "1.3.6.1.4.1.14988.1.1.3.100.1.3"
	oidMtxrGaugeUnit  = "1.3.6.1.4.1.14988.1.1.3.100.1.4"
)

// Gauge is one health sensor. Unit is MIKROTIK-MIB's enum: 1 celsius, 2 rpm,
// 3 dV, 4 dA, 5 dW, 6 status (0 ok, 1 fail).
type Gauge struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Unit  int    `json:"unit"`
}

type gaugeRow struct {
	index int
	gauge Gauge
}

type gaugeColumn struct {
	oid string
	set func(row *Gauge, pdu gosnmp.SnmpPDU)
}

var gaugeColumns = []gaugeColumn{
	{oidMtxrGaugeName, func(row *Gauge, pdu gosnmp.SnmpPDU) { row.Name = pduText(pdu) }},
	{oidMtxrGaugeValue, func(row *Gauge, pdu gosnmp.SnmpPDU) { row.Value, _ = toInt64(pdu.Value) }},
	{oidMtxrGaugeUnit, func(row *Gauge, pdu gosnmp.SnmpPDU) {
		unit, _ := toInt64(pdu.Value)
		row.Unit = int(unit)
	}},
}

// WalkGauges reads every row of the router's gauge table, ordered by index. A
// model without the table returns no rows and no error.
func WalkGauges(ctx context.Context, host string, port int, community string) ([]Gauge, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout

	rows := map[int]*gaugeRow{}
	for _, column := range gaugeColumns {
		err := bulkWalk(client, column.oid, func(pdu gosnmp.SnmpPDU) error {
			index, err := lastSubidentifier(pdu.Name)
			if err != nil {
				return err
			}
			row, ok := rows[index]
			if !ok {
				row = &gaugeRow{index: index}
				rows[index] = row
			}
			column.set(&row.gauge, pdu)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", column.oid, err)
		}
	}

	sorted := make([]*gaugeRow, 0, len(rows))
	for _, row := range rows {
		sorted = append(sorted, row)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].index < sorted[j].index })
	out := make([]Gauge, 0, len(sorted))
	for _, row := range sorted {
		out = append(out, row.gauge)
	}
	return out, nil
}
