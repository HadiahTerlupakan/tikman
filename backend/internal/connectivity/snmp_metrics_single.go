package connectivity

import (
	"fmt"
	"log"
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

	queryUint64 := func(oidBase string) uint64 {
		oid := fmt.Sprintf("%s.%d.%d.1", oidBase, ifIndex, ontID)
		if result, err := client.Get([]string{oid}); err == nil && len(result.Variables) > 0 {
			if v, ok := toInt64(result.Variables[0].Value); ok {
				return uint64(v)
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
	metrics.RxBytes = queryUint64(OID_ZXGPON_ONU_RX_BYTES_TABLE)
	metrics.TxBytes = queryUint64(OID_ZXGPON_ONU_TX_BYTES_TABLE)
	metrics.RxPackets = queryUint64(OID_ZXGPON_ONU_RX_PACKETS_TABLE)
	metrics.TxPackets = queryUint64(OID_ZXGPON_ONU_TX_PACKETS_TABLE)
	metrics.RxErrors = queryUint64(OID_ZXGPON_ONU_RX_ERRORS_TABLE)
	metrics.TxErrors = queryUint64(OID_ZXGPON_ONU_TX_ERRORS_TABLE)

	log.Printf("[Realtime] ONT %d/%d/%d (ifIndex=%d): RxBytes=%d TxBytes=%d RxPkts=%d TxPkts=%d",
		slot, port, ontID, ifIndex, metrics.RxBytes, metrics.TxBytes, metrics.RxPackets, metrics.TxPackets)

	return metrics, nil
}
