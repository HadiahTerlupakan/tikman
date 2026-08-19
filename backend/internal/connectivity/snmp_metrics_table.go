package connectivity

import (
	"fmt"

	"github.com/gosnmp/gosnmp"
)

// walkONTMetricTable is a generic helper that walks a metrics table and collects
// values keyed by the ONT location decoded from each OID.
func walkONTMetricTable[T any](ipAddress, community string, snmpPort int, baseOID string, mapper func(ONTLocation, int64) T) (map[ONTLocation]T, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	results := make(map[ONTLocation]T)

	err = client.Walk(baseOID, func(pdu gosnmp.SnmpPDU) error {
		loc, ok := parseZxGponSuffix(pdu.Name, baseOID)
		if !ok {
			return nil // skip non-ZXGPON entries
		}

		value, ok := toInt64(pdu.Value)
		if !ok || value >= 30000 {
			return nil // ignore invalid or "no signal" readings
		}

		results[loc] = mapper(loc, value)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	return results, nil
}
