package connectivity

import (
	"fmt"
)

// MetricsQuerier is implemented by drivers that can read the optical metrics of
// named ONUs directly, rather than sweeping the tables for every ONU on the
// OLT. Callers hold the locations already, from the status walk.
type MetricsQuerier interface {
	QueryMetricsFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTMetrics, error)
}

// metricColumn is one optical reading and where to find it.
type metricColumn struct {
	oid   string
	apply func(*ONTMetrics, int64)
}

// zteMetricColumns lists the exact instances holding one ONU's optical
// readings.
//
// The .3.50.12 family is indexed by ifIndex.onuID.1; distance sits in its own
// table indexed by ifIndex.onuID. The transforms match the walk these replace,
// so a reading is the same figure whichever path produced it.
func zteMetricColumns(loc ONTLocation) []metricColumn {
	typeSpace := OnuTypeIfIndexBase + loc.Slot*OnuTypeSlotStride + loc.Port*OnuTypeIncrement
	instance := func(table, tail string) string {
		return fmt.Sprintf("%s.%d.%d%s", table, typeSpace, loc.ONTID, tail)
	}

	return []metricColumn{
		{instance(OID_ZXGPON_ONU_RX_POWER_TABLE, ".1"), func(m *ONTMetrics, raw int64) {
			m.RxPower = decodeZxGponPower(raw)
		}},
		{instance(OID_ZXGPON_ONU_TX_POWER_TABLE, ".1"), func(m *ONTMetrics, raw int64) {
			m.TxPower = decodeZxGponPower(raw)
		}},
		{instance(OID_ZXGPON_ONU_TEMPERATURE_TABLE, ".1"), func(m *ONTMetrics, raw int64) {
			m.Temperature = float64(raw) / 256.0
		}},
		{instance(OID_ZXGPON_ONU_VOLTAGE_TABLE, ".1"), func(m *ONTMetrics, raw int64) {
			m.Voltage = float64(raw) / 10000.0
		}},
		{instance(OID_ZXGPON_ONU_TX_BIAS_CURRENT_TABLE, ".1"), func(m *ONTMetrics, raw int64) {
			m.TxBiasCurrent = float64(raw) / 500.0
		}},
		{instance(OID_ZXGPON_ONU_DISTANCE_TABLE, ""), func(m *ONTMetrics, raw int64) {
			m.Distance = int(raw)
		}},
	}
}

// QueryMetricsFor reads the optical metrics of known ONUs with batched GETs.
//
// Walking these tables does not finish on a populated OLT: the RX power walk
// consistently returned 96 of 200 rows and then timed out, so most ONUs had no
// reading at all. Each instance answers a GET in about seventeen milliseconds.
func (zteDriver) QueryMetricsFor(ipAddress, community string, snmpPort int, locations []ONTLocation) (map[ONTLocation]ONTMetrics, error) {
	metrics := make(map[ONTLocation]ONTMetrics, len(locations))
	if len(locations) == 0 {
		return metrics, nil
	}

	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	type request struct {
		loc    ONTLocation
		column metricColumn
	}
	pending := make([]request, 0, len(locations)*6)
	for _, loc := range locations {
		metrics[loc] = ONTMetrics{}
		for _, column := range zteMetricColumns(loc) {
			pending = append(pending, request{loc: loc, column: column})
		}
	}

	for start := 0; start < len(pending); start += zteGetBatchSize {
		end := start + zteGetBatchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[start:end]

		oids := make([]string, len(batch))
		for i, item := range batch {
			oids[i] = item.column.oid
		}

		// A failed batch leaves those readings unset rather than failing the
		// sweep: one unreadable column is no reason to report no metrics.
		result, err := client.Get(oids)
		if err != nil || result == nil {
			continue
		}
		for i, pdu := range result.Variables {
			if i >= len(batch) {
				break
			}
			raw, ok := toInt64(pdu.Value)
			if !ok {
				continue
			}
			reading := metrics[batch[i].loc]
			batch[i].column.apply(&reading, raw)
			metrics[batch[i].loc] = reading
		}
	}

	return metrics, nil
}
