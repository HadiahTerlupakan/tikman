package connectivity

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
)

// ifGetIndexesPerRequest keeps one GET at 50 varbinds (ten interfaces by five
// columns), under gosnmp's default of 60 per PDU.
const ifGetIndexesPerRequest = 10

// GetIfCounters reads the same columns as WalkIfCounters for the given
// ifIndexes only, by GET. A BRAS lists an interface per PPP session, so walking
// its tables reads thousands of rows to use a handful. An index the agent does
// not have (noSuchInstance) is left out of the result.
func GetIfCounters(ctx context.Context, host string, port int, community string, indexes []int) ([]IfCounters, error) {
	client, err := newSNMPClientWithContext(ctx, host, community, port)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()
	client.Timeout = ifCounterRequestTimeout

	rows := map[int]*IfCounters{}
	for start := 0; start < len(indexes); start += ifGetIndexesPerRequest {
		chunk := indexes[start:min(start+ifGetIndexesPerRequest, len(indexes))]
		packet, err := client.Get(ifGetOIDs(chunk))
		if err != nil {
			return nil, fmt.Errorf("get interface counters: %w", err)
		}
		for _, pdu := range packet.Variables {
			if err := applyIfPDU(rows, pdu); err != nil {
				return nil, err
			}
		}
	}

	out := make([]IfCounters, 0, len(indexes))
	for _, index := range indexes {
		if row, ok := rows[index]; ok && row.Name != "" {
			out = append(out, *row)
		}
	}
	return out, nil
}

func ifGetOIDs(indexes []int) []string {
	oids := make([]string, 0, len(indexes)*len(ifColumns))
	for _, index := range indexes {
		for _, column := range ifColumns {
			oids = append(oids, fmt.Sprintf("%s.%d", column.oid, index))
		}
	}
	return oids
}

func applyIfPDU(rows map[int]*IfCounters, pdu gosnmp.SnmpPDU) error {
	if pdu.Type == gosnmp.NoSuchInstance || pdu.Type == gosnmp.NoSuchObject || pdu.Type == gosnmp.Null {
		return nil
	}
	name := strings.TrimPrefix(pdu.Name, ".")
	for _, column := range ifColumns {
		if !strings.HasPrefix(name, column.oid+".") {
			continue
		}
		index, err := lastSubidentifier(name)
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
	}
	return nil
}
