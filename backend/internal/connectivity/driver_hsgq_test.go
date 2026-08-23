package connectivity

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// Real ifIndexes from the verified HSGQ-XE08ID: 0x01000101 is the ONU the device
// itself names "ONU01/01", 0x01000602 is "ONU06/02".
const (
	hsgqIfIndexPON1ONU1 = 0x01000101
	hsgqIfIndexPON6ONU2 = 0x01000602
)

func hsgqOID(table string, arcs ...any) string {
	oid := normaliseOID(table)
	for _, arc := range arcs {
		oid += fmt.Sprintf(".%v", arc)
	}
	return oid
}

// The ifIndex layout is 0x0100PPNN. This was checked against the device's own
// ONU names for all 246 ONUs with no mismatch, so it is decoded rather than
// left as an opaque label.
func TestHSGQLocationDecodesPONAndONU(t *testing.T) {
	tests := []struct {
		ifIndex uint32
		want    ONTLocation
		name    string
	}{
		{ifIndex: 0x01000101, want: ONTLocation{Port: 1, ONTID: 1}, name: "ONU01/01"},
		{ifIndex: 0x01000102, want: ONTLocation{Port: 1, ONTID: 2}, name: "ONU01/02"},
		{ifIndex: 0x01000602, want: ONTLocation{Port: 6, ONTID: 2}, name: "ONU06/02"},
		{ifIndex: 0x01000839, want: ONTLocation{Port: 8, ONTID: 57}, name: "ONU08/57"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hsgqLocation(tt.ifIndex); got != tt.want {
				t.Errorf("hsgqLocation(0x%08X) = %+v, want %+v", tt.ifIndex, got, tt.want)
			}
			// QueryONTMetrics rebuilds the ifIndex, so the two must be inverses;
			// a mismatch would query a different ONU than the caller asked for.
			if back := hsgqIfIndexFor(tt.want.Port, tt.want.ONTID); back != tt.ifIndex {
				t.Errorf("hsgqIfIndexFor(%d, %d) = 0x%08X, want 0x%08X", tt.want.Port, tt.want.ONTID, back, tt.ifIndex)
			}
		})
	}
}

// Polarity is confirmed: on the verified device all 208 ONUs reporting 1 had a
// plausible optical level and none of the 38 reporting 2 had any reading.
func TestHSGQDecodeStatus(t *testing.T) {
	tests := []struct {
		raw  int64
		want int
		name string
	}{
		{raw: 1, want: PhaseStateOnline, name: "1 is online"},
		{raw: 2, want: PhaseStateOffline, name: "2 is offline"},
		// An unrecognised value must not fall through to offline: a firmware that
		// adds a third state would otherwise report a fake outage.
		{raw: 3, want: PhaseStateUnknown, name: "unknown value stays unknown"},
		{raw: 0, want: PhaseStateUnknown, name: "zero stays unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hsgqDecodeStatus(tt.raw); got != tt.want {
				t.Errorf("hsgqDecodeStatus(%d) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestHSGQDecodePower(t *testing.T) {
	tests := []struct {
		name string
		raw  int64
		want *float64
	}{
		// Both ends of the range actually observed on the device.
		{name: "weakest observed rx", raw: -2958, want: ptr(-29.58)},
		{name: "strongest observed rx", raw: -717, want: ptr(-7.17)},
		{name: "observed tx", raw: 247, want: ptr(2.47)},
		// There is no sentinel on this device - a dark ONU has no optical row at
		// all - so these guard against malformed values, and zero is refused
		// because 0.00 dBm would render as a perfect signal on a dark fibre.
		{name: "zero is not a reading", raw: 0},
		{name: "absurdly low", raw: -900000},
		{name: "absurdly high", raw: 900000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hsgqDecodePower(tt.raw)
			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("hsgqDecodePower(%d) = %.2f, want nil", tt.raw, *got)
			case tt.want != nil && got == nil:
				t.Fatalf("hsgqDecodePower(%d) = nil, want %.2f", tt.raw, *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Errorf("hsgqDecodePower(%d) = %.2f, want %.2f", tt.raw, *got, *tt.want)
			}
		})
	}
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

// EPON identifies an ONU by MAC and the device exposes no serial column, so the
// MAC fills SerialNumber too - that is the identity the application keys on.
func TestHSGQInventoryUsesMACAsIdentity(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{
			Name:  hsgqOID(hsgqONUMAC, hsgqIfIndexPON1ONU1),
			Type:  gosnmp.OctetString,
			Value: []byte{0xEC, 0x23, 0x7B, 0xD7, 0x1F, 0xA8},
		},
		{Name: hsgqOID(hsgqONUName, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("ONU01/01")},
		{Name: hsgqOID(hsgqONUModel, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("F460910")},
		{Name: hsgqOID(hsgqONUFirmware, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("V6.0.3P1T1")},
		{Name: hsgqOID(hsgqONUHardware, hsgqIfIndexPON1ONU1), Type: gosnmp.OctetString, Value: []byte("V6.0")},
		// An ONU the caller did not ask about must not appear in the result.
		{Name: hsgqOID(hsgqONUName, hsgqIfIndexPON6ONU2), Type: gosnmp.OctetString, Value: []byte("ONU06/02")},
	})

	loc := ONTLocation{Port: 1, ONTID: 1}
	inventory, err := hsgqDriver{}.Inventory("127.0.0.1", "public", port, []ONTLocation{loc})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}

	if len(inventory) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(inventory), inventory)
	}

	got := inventory[loc]
	want := ONTInventory{
		SerialNumber: "EC237BD71FA8",
		MACAddress:   "EC:23:7B:D7:1F:A8",
		Name:         "ONU01/01",
		DeviceType:   "F460910",
		// The dots must survive: ExtractName would render these "V603P1T1" and
		// "V60", which is a wrong firmware version shown to the operator.
		SoftwareVersion: "V6.0.3P1T1",
		HardwareVersion: "V6.0",
	}
	if got != want {
		t.Errorf("inventory = %+v, want %+v", got, want)
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

// The IF-MIB index is assigned dynamically and only to registered ONUs, so it
// shifts when an ONU drops. Resolution must therefore happen per read, by name.
// A cached mapping would keep reading the old index and bill one subscriber's
// traffic to another.
func TestHSGQResolveIfMibIndexesFollowsRenumbering(t *testing.T) {
	resolve := func(t *testing.T, pdus []gosnmp.SnmpPDU) map[ONTLocation]uint32 {
		t.Helper()
		_, port := newUncfgAgent(t, pdus)
		client, err := newSNMPClient("127.0.0.1", "public", port)
		if err != nil {
			t.Fatalf("client: %v", err)
		}
		defer func() { _ = client.Conn.Close() }()

		indexes, err := hsgqResolveIfMibIndexes(client)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		return indexes
	}

	before := resolve(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("PON01")},
		{Name: hsgqOID(ifDescrOID, 17), Type: gosnmp.OctetString, Value: []byte("ONU01/01")},
		{Name: hsgqOID(ifDescrOID, 18), Type: gosnmp.OctetString, Value: []byte("ONU01/02")},
	})
	if got := before[ONTLocation{Port: 1, ONTID: 2}]; got != 18 {
		t.Fatalf("ONU01/02 = %d, want 18", got)
	}
	// Physical ports must not be picked up as ONUs.
	if len(before) != 2 {
		t.Fatalf("got %d ONU entries, want 2: %+v", len(before), before)
	}

	// ONU01/01 drops and the OLT renumbers, so ONU01/02 now answers on 17.
	after := resolve(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("PON01")},
		{Name: hsgqOID(ifDescrOID, 17), Type: gosnmp.OctetString, Value: []byte("ONU01/02")},
	})
	if got := after[ONTLocation{Port: 1, ONTID: 2}]; got != 17 {
		t.Errorf("after renumbering ONU01/02 = %d, want 17", got)
	}
	if _, stillThere := after[ONTLocation{Port: 1, ONTID: 1}]; stillThere {
		t.Error("ONU01/01 is gone from the OLT but still resolved")
	}
}

func TestToUint64RejectsNegativeAndPreservesLargeCounters(t *testing.T) {
	// Above 2^63: routing this through int64 would make it negative and then an
	// absurd byte total.
	if got, ok := toUint64(uint64(1) << 63); !ok || got != 1<<63 {
		t.Errorf("toUint64(2^63) = %d, %v", got, ok)
	}
	if _, ok := toUint64(-5); ok {
		t.Error("a negative counter must be rejected, not wrapped")
	}
	if _, ok := toUint64("not a number"); ok {
		t.Error("non-numeric must be rejected")
	}
}

func TestHSGQFormatMAC(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "six bytes", value: []byte{0xEC, 0x23, 0x7B, 0xD7, 0x1F, 0xA8}, want: "EC:23:7B:D7:1F:A8"},
		{name: "wrong length is rejected", value: []byte{0x01, 0x02}, want: ""},
		{name: "non-bytes is rejected", value: 42, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hsgqFormatMAC(tt.value); got != tt.want {
				t.Errorf("hsgqFormatMAC(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// The counter table is keyed by physical port, not by ONU, so there is no
// per-ONU rate to report and the driver must say so rather than invent one.
func TestHSGQTrafficRatesAreUnsupported(t *testing.T) {
	if _, err := (hsgqDriver{}).WalkTrafficRates("127.0.0.1", "public", 161); !errors.Is(err, ErrUnsupported) {
		t.Errorf("WalkTrafficRates error = %v, want ErrUnsupported", err)
	}
	if _, err := (hsgqDriver{}).QueryTrafficRates("127.0.0.1", "public", 161, 0, 1, 1); !errors.Is(err, ErrUnsupported) {
		t.Errorf("QueryTrafficRates error = %v, want ErrUnsupported", err)
	}
}

func TestHSGQUnconfiguredScanIsUnsupported(t *testing.T) {
	_, err := hsgqDriver{}.WalkUnconfigured(context.Background(), "127.0.0.1", "public", 161)
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want ErrUnsupported", err)
	}
}

func ptr(v float64) *float64 { return &v }

func formatPtr(v *float64) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%.2f", *v)
}
