package connectivity

import (
	"fmt"
	"testing"

	"github.com/gosnmp/gosnmp"
)

func hsgqOID(table string, arcs ...any) string {
	oid := normaliseOID(table)
	for _, arc := range arcs {
		oid += fmt.Sprintf(".%v", arc)
	}
	return oid
}

func TestHSGQWalkStatusesDecodesEveryONU(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(hsgqONUStatus, hsgqIfIndexPON1ONU1), Type: gosnmp.Integer, Value: 1},
		{Name: hsgqOID(hsgqONUStatus, hsgqIfIndexPON6ONU2), Type: gosnmp.Integer, Value: 2},
	})

	statuses, err := hsgqDriver{}.WalkStatuses("127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("WalkStatuses: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("got %d ONUs, want 2: %+v", len(statuses), statuses)
	}
	if got := statuses[ONTLocation{Port: 1, ONTID: 1}]; got != PhaseStateOnline {
		t.Errorf("ONU01/01 state = %d, want PhaseStateOnline", got)
	}
	if got := statuses[ONTLocation{Port: 6, ONTID: 2}]; got != PhaseStateOffline {
		t.Errorf("ONU06/02 state = %d, want PhaseStateOffline", got)
	}
}

// The optical table carries both ends of the link in one column: .0.0 is the
// ONU, .65535.65535 the PON port. Only the ONU end belongs to an ONT.
func TestHSGQWalkMetricsIgnoresPONSideOfOpticalRow(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(hsgqRxPower, hsgqIfIndexPON1ONU1, 0, 0), Type: gosnmp.Integer, Value: -2275},
		{Name: hsgqOID(hsgqRxPower, 0x01000100, 65535, 65535), Type: gosnmp.Integer, Value: -3154},
		{Name: hsgqOID(hsgqTxPower, hsgqIfIndexPON1ONU1, 0, 0), Type: gosnmp.Integer, Value: 247},
		{Name: hsgqOID(hsgqBiasCurrent, hsgqIfIndexPON1ONU1, 0, 0), Type: gosnmp.Integer, Value: 1350},
		{Name: hsgqOID(hsgqVoltage, hsgqIfIndexPON1ONU1, 0, 0), Type: gosnmp.Integer, Value: 324},
		{Name: hsgqOID(hsgqTemperature, hsgqIfIndexPON1ONU1, 0, 0), Type: gosnmp.Integer, Value: 4400},
		{Name: hsgqOID(hsgqONUDistance, hsgqIfIndexPON1ONU1), Type: gosnmp.Integer, Value: 1424},
	})

	metrics, err := hsgqDriver{}.WalkMetrics("127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("WalkMetrics: %v", err)
	}

	loc := ONTLocation{Port: 1, ONTID: 1}
	if len(metrics) != 1 {
		t.Fatalf("got %d ONUs, want only %+v: %+v", len(metrics), loc, metrics)
	}

	m := metrics[loc]
	if m.RxPower == nil || *m.RxPower != -22.75 {
		t.Errorf("RxPower = %v, want -22.75 (the ONU end, not the PON's -31.54)", formatPtr(m.RxPower))
	}
	if m.TxPower == nil || *m.TxPower != 2.47 {
		t.Errorf("TxPower = %v, want 2.47", formatPtr(m.TxPower))
	}
	if m.TxBiasCurrent != 13.5 {
		t.Errorf("TxBiasCurrent = %v, want 13.5", m.TxBiasCurrent)
	}
	if m.Voltage != 3.24 {
		t.Errorf("Voltage = %v, want 3.24", m.Voltage)
	}
	if m.Temperature != 44 {
		t.Errorf("Temperature = %v, want 44", m.Temperature)
	}
	if m.Distance != 1424 {
		t.Errorf("Distance = %d, want 1424", m.Distance)
	}
}

// Traffic counters live in the IF-MIB under an ifIndex unrelated to the
// enterprise one: ONU01/01 is 0x01000101 in the vendor tables and 17 here. The
// only link between the two is the interface name, so it has to be parsed.
//
// Direction follows IF-MIB's convention, which is relative to the OLT: In is
// received from the ONU (upstream, Rx) and Out is sent to it (downstream, Tx).
func TestHSGQWalkMetricsReadsIFMIBTrafficCounters(t *testing.T) {
	const ifMib1, ifMib2 = 17, 18

	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(hsgqONUStatus, hsgqIfIndexPON1ONU1), Type: gosnmp.Integer, Value: 1},

		// IF-MIB names are what tie an interface to a physical position. A PON port
		// shares the table and must not be mistaken for an ONU.
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("PON01")},
		{Name: hsgqOID(ifDescrOID, ifMib1), Type: gosnmp.OctetString, Value: []byte("ONU01/01")},
		{Name: hsgqOID(ifDescrOID, ifMib2), Type: gosnmp.OctetString, Value: []byte("ONU06/02")},

		{Name: hsgqOID(ifHCInOctetsOID, ifMib1), Type: gosnmp.Counter64, Value: uint64(150030075803)},
		{Name: hsgqOID(ifHCOutOctetsOID, ifMib1), Type: gosnmp.Counter64, Value: uint64(367574144554)},
		{Name: hsgqOID(ifHCInUcastPktsOID, ifMib1), Type: gosnmp.Counter64, Value: uint64(236873065)},
		{Name: hsgqOID(ifHCOutUcastPktsOID, ifMib1), Type: gosnmp.Counter64, Value: uint64(329029836)},
		{Name: hsgqOID(ifInErrorsOID, ifMib1), Type: gosnmp.Counter32, Value: uint(3)},
		{Name: hsgqOID(ifOutErrorsOID, ifMib1), Type: gosnmp.Counter32, Value: uint(5)},

		// A counter on the PON port itself must not land on any ONT.
		{Name: hsgqOID(ifHCInOctetsOID, 1), Type: gosnmp.Counter64, Value: uint64(999999999999)},
	})

	metrics, err := hsgqDriver{}.WalkMetrics("127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("WalkMetrics: %v", err)
	}

	m := metrics[ONTLocation{Port: 1, ONTID: 1}]
	if m.RxBytes != 150030075803 {
		t.Errorf("RxBytes = %d, want 150030075803 (ifHCInOctets, upstream)", m.RxBytes)
	}
	if m.TxBytes != 367574144554 {
		t.Errorf("TxBytes = %d, want 367574144554 (ifHCOutOctets, downstream)", m.TxBytes)
	}
	if m.RxPackets != 236873065 || m.TxPackets != 329029836 {
		t.Errorf("packets = %d/%d, want 236873065/329029836", m.RxPackets, m.TxPackets)
	}
	if m.RxErrors != 3 || m.TxErrors != 5 {
		t.Errorf("errors = %d/%d, want 3/5", m.RxErrors, m.TxErrors)
	}

	// The PON port's counter must not have been attributed to an ONT.
	for loc, got := range metrics {
		if got.RxBytes == 999999999999 {
			t.Errorf("PON port counter leaked onto ONT %+v", loc)
		}
	}
}
