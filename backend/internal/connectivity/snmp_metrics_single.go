package connectivity

import (
	"fmt"
)

// zteQueryONTMetrics queries metrics for a single ONT via SNMP in real-time
func zteQueryONTMetrics(ipAddress, community string, snmpPort int, slot, port, ontID int) (*ONTMetrics, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Conn.Close() }()

	ifIndex := int(encodeZxGponIfIndex(0x10, slot, port))
	metrics := &ONTMetrics{}

	queryInt64 := func(oidBase string) int64 {
		oid := fmt.Sprintf("%s.%d.%d.1", oidBase, ifIndex, ontID)
		if result, err := client.Get([]string{oid}); err == nil && len(result.Variables) > 0 {
			if v, ok := toInt64(result.Variables[0].Value); ok {
				return v
			}
		}
		return 0
	}

	queryOpticalPower := func(oidBase string) *float64 {
		oid := fmt.Sprintf("%s.%d.%d.1", oidBase, ifIndex, ontID)
		if result, err := client.Get([]string{oid}); err == nil && len(result.Variables) > 0 {
			if v, ok := toInt64(result.Variables[0].Value); ok {
				return decodeZxGponPower(v)
			}
		}
		return nil
	}

	queryFloat := func(oidBase string, divisor float64) float64 {
		oid := fmt.Sprintf("%s.%d.%d.1", oidBase, ifIndex, ontID)
		if result, err := client.Get([]string{oid}); err == nil && len(result.Variables) > 0 {
			if v, ok := toInt64(result.Variables[0].Value); ok && v < 30000 {
				return float64(v) / divisor
			}
		}
		return 0
	}

	metrics.RxPower = queryOpticalPower(OID_ZXGPON_ONU_RX_POWER_TABLE)
	metrics.TxPower = queryOpticalPower(OID_ZXGPON_ONU_TX_POWER_TABLE)
	metrics.Temperature = queryFloat(OID_ZXGPON_ONU_TEMPERATURE_TABLE, 256.0)
	metrics.Voltage = queryFloat(OID_ZXGPON_ONU_VOLTAGE_TABLE, 10000.0)
	metrics.TxBiasCurrent = queryFloat(OID_ZXGPON_ONU_TX_BIAS_CURRENT_TABLE, 500.0)
	metrics.Distance = int(queryInt64(OID_ZXGPON_ONU_DISTANCE_TABLE))
	// Traffic counters are indexed in the ONU-ID space, not this one, and are
	// read by the traffic-rate query alongside the gauges.

	return metrics, nil
}
