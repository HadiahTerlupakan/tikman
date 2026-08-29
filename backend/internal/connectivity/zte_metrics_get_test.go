package connectivity

import (
	"fmt"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func metricOID(table string, slot, port, ontID int, tail string) string {
	ifIndex := OnuTypeIfIndexBase + slot*OnuTypeSlotStride + port*OnuTypeIncrement
	return fmt.Sprintf("%s.%d.%d%s", table, ifIndex, ontID, tail)
}

// Sweeping these tables does not finish on a populated OLT: the RX power walk
// returned 96 of 200 rows and then timed out, so most ONUs had no reading at
// all. Reading named instances has to produce the same figures the walk did.
func TestQueryMetricsForDecodesLikeTheWalk(t *testing.T) {
	loc := ONTLocation{Slot: 3, Port: 1, ONTID: 1}
	_, snmpPort := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: metricOID(OID_ZXGPON_ONU_RX_POWER_TABLE, 3, 1, 1, ".1"), Type: gosnmp.Integer, Value: 2159},
		{Name: metricOID(OID_ZXGPON_ONU_TX_POWER_TABLE, 3, 1, 1, ".1"), Type: gosnmp.Integer, Value: 16299},
		{Name: metricOID(OID_ZXGPON_ONU_TEMPERATURE_TABLE, 3, 1, 1, ".1"), Type: gosnmp.Integer, Value: 12800},
		{Name: metricOID(OID_ZXGPON_ONU_VOLTAGE_TABLE, 3, 1, 1, ".1"), Type: gosnmp.Integer, Value: 32000},
		{Name: metricOID(OID_ZXGPON_ONU_TX_BIAS_CURRENT_TABLE, 3, 1, 1, ".1"), Type: gosnmp.Integer, Value: 5000},
		{Name: metricOID(OID_ZXGPON_ONU_DISTANCE_TABLE, 3, 1, 1, ""), Type: gosnmp.Integer, Value: 1004},
	})

	metrics, err := zteDriver{}.QueryMetricsFor("127.0.0.1", "public", snmpPort, []ONTLocation{loc})
	require.NoError(t, err)

	reading := metrics[loc]
	require.NotNil(t, reading.RxPower)
	assert.InDelta(t, *decodeZxGponPower(2159), *reading.RxPower, 0.001)
	require.NotNil(t, reading.TxPower)
	assert.InDelta(t, *decodeZxGponPower(16299), *reading.TxPower, 0.001)
	assert.InDelta(t, 50.0, reading.Temperature, 0.001)
	assert.InDelta(t, 3.2, reading.Voltage, 0.001)
	assert.InDelta(t, 10.0, reading.TxBiasCurrent, 0.001)
	assert.Equal(t, 1004, reading.Distance)
}

// Every ONU asked for has to come back, which is what the truncated walk failed
// to do.
func TestQueryMetricsForCoversEveryONU(t *testing.T) {
	pdus := []gosnmp.SnmpPDU{}
	locations := []ONTLocation{}
	for _, onu := range []int{1, 15, 82} {
		locations = append(locations, ONTLocation{Slot: 3, Port: 1, ONTID: onu})
		pdus = append(pdus, gosnmp.SnmpPDU{
			Name:  metricOID(OID_ZXGPON_ONU_DISTANCE_TABLE, 3, 1, onu, ""),
			Type:  gosnmp.Integer,
			Value: 100 * onu,
		})
	}
	_, snmpPort := newUncfgAgent(t, pdus)

	metrics, err := zteDriver{}.QueryMetricsFor("127.0.0.1", "public", snmpPort, locations)
	require.NoError(t, err)

	for _, loc := range locations {
		assert.Equal(t, 100*loc.ONTID, metrics[loc].Distance, "ONU %d", loc.ONTID)
	}
}

func TestQueryMetricsForWithoutLocations(t *testing.T) {
	metrics, err := zteDriver{}.QueryMetricsFor("127.0.0.1", "public", 1, nil)

	require.NoError(t, err)
	assert.Empty(t, metrics)
}
