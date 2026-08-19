package connectivity

import (
	"fmt"
)

// ONTMetrics represents collected metrics from an ONT.
// RxPower/TxPower are pointers because ZTE returns a sentinel value when there
// is no optical signal - a nil pointer means "no reading", which must not be
// confused with a genuine 0.00 dBm measurement.
type ONTMetrics struct {
	RxPower       *float64 // in dBm, nil when no signal
	TxPower       *float64 // in dBm, nil when no signal
	Temperature   float64  // in Celsius
	Voltage       float64  // in Volts
	TxBiasCurrent float64  // in mA
	Distance      int      // in meters
	RxBytes       uint64
	TxBytes       uint64
	RxPackets     uint64
	TxPackets     uint64
	RxErrors      uint64
	TxErrors      uint64
	SerialNumber  string // ONU serial number
}

// QueryONTMetricsWithDynamicPort queries power metrics for a single ONT using
// dynamic slot detection from the device. Slot parameter is ignored here because
// the ZXGPON ifIndex encoding already includes the slot value. We keep this
// function for backwards compatibility.
func QueryONTMetricsWithDynamicPort(ipAddress, community string, snmpPort int, slot, gponPort, ontID int) (ONTMetrics, error) {
	// Build ZXGPON ifIndex from parameters
	zxIfIndex := encodeZxGponIfIndex(1, slot, gponPort)

	metrics := ONTMetrics{}

	// RX power
	rxOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_RX_POWER_TABLE, zxIfIndex, ontID)
	val, err := pollSNMPInteger(ipAddress, community, snmpPort, rxOID)
	if err == nil && val < 30000 {
		rxDbm := decodeZxGponPower(val)
		metrics.RxPower = rxDbm
	}

	// TX power
	txOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_TX_POWER_TABLE, zxIfIndex, ontID)
	val, err = pollSNMPInteger(ipAddress, community, snmpPort, txOID)
	if err == nil && val < 30000 {
		txDbm := decodeZxGponPower(val)
		metrics.TxPower = txDbm
	}

	// Distance
	distOID := fmt.Sprintf("%s.%d.%d", OID_ZXGPON_ONU_DISTANCE_TABLE, zxIfIndex, ontID)
	val, err = pollSNMPInteger(ipAddress, community, snmpPort, distOID)
	if err == nil && val > 0 && val < 30000 {
		metrics.Distance = int(val)
	}

	return metrics, nil
}

// pollSNMPInteger performs a simple SNMP GET for a numeric value
func pollSNMPInteger(ipAddress, community string, snmpPort int, oid string) (int64, error) {
	client, err := newSNMPClient(ipAddress, community, snmpPort)
	if err != nil {
		return 0, err
	}
	defer func() { _ = client.Conn.Close() }()

	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, err
	}

	if len(result.Variables) == 0 {
		return 0, fmt.Errorf("no response for OID %s", oid)
	}

	val, ok := toInt64(result.Variables[0].Value)
	if !ok {
		return 0, fmt.Errorf("invalid SNMP value type for OID %s", oid)
	}

	return val, nil
}
