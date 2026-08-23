package connectivity

import (
	"context"
	"errors"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// A clean correlation resolves the polarity: the ONUs with light are exactly the
// ones reporting status 1.
func TestInferHSGQOnlineStatusFromCleanCorrelation(t *testing.T) {
	rows := []HSGQProbeRow{
		{IfIndex: 1, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -1250},
		{IfIndex: 2, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -2010},
		{IfIndex: 3, HasStatus: true, StatusRaw: 0, HasRx: true, RxRaw: 0},
		{IfIndex: 4, HasStatus: true, StatusRaw: 0, HasRx: false},
	}

	online, confident := inferHSGQOnlineStatus(rows)
	if !confident {
		t.Fatal("confident = false, want true for a clean split")
	}
	if online != 1 {
		t.Errorf("online status = %d, want 1", online)
	}
}

// The inverse dataset must yield the inverse answer, otherwise the function is
// hardcoding a guess rather than reading the data.
func TestInferHSGQOnlineStatusDetectsInvertedPolarity(t *testing.T) {
	rows := []HSGQProbeRow{
		{IfIndex: 1, HasStatus: true, StatusRaw: 0, HasRx: true, RxRaw: -1250},
		{IfIndex: 2, HasStatus: true, StatusRaw: 1, HasRx: false},
	}

	online, confident := inferHSGQOnlineStatus(rows)
	if !confident {
		t.Fatal("confident = false, want true")
	}
	if online != 0 {
		t.Errorf("online status = %d, want 0", online)
	}
}

// Anything short of a clean split must stay unresolved. Guessing here is the
// failure the spec singled out: an inverted polarity reports a healthy network
// during an outage.
func TestInferHSGQOnlineStatusRefusesAmbiguousData(t *testing.T) {
	tests := []struct {
		name string
		rows []HSGQProbeRow
	}{
		{
			name: "only one status value observed",
			rows: []HSGQProbeRow{
				{IfIndex: 1, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -1250},
				{IfIndex: 2, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -1300},
			},
		},
		{
			name: "a status group mixes ONUs with and without signal",
			rows: []HSGQProbeRow{
				{IfIndex: 1, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -1250},
				{IfIndex: 2, HasStatus: true, StatusRaw: 1, HasRx: false},
				{IfIndex: 3, HasStatus: true, StatusRaw: 0, HasRx: false},
			},
		},
		{
			name: "both groups have signal, so neither means offline",
			rows: []HSGQProbeRow{
				{IfIndex: 1, HasStatus: true, StatusRaw: 1, HasRx: true, RxRaw: -1250},
				{IfIndex: 2, HasStatus: true, StatusRaw: 0, HasRx: true, RxRaw: -1300},
			},
		},
		{
			name: "no rows at all",
			rows: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, confident := inferHSGQOnlineStatus(tt.rows); confident {
				t.Error("confident = true, want false")
			}
		})
	}
}

// The sentinel is the implausible value that repeats across dark ONUs, so it
// must outrank a one-off oddity.
func TestHSGQSentinelCandidatesRankByFrequency(t *testing.T) {
	rows := []HSGQProbeRow{
		{HasRx: true, RxRaw: -10000}, // implausible, appears three times
		{HasRx: true, RxRaw: -10000},
		{HasRx: true, RxRaw: -10000},
		{HasRx: true, RxRaw: 30000}, // implausible, once
		{HasRx: true, RxRaw: -1250}, // a real level, must not be listed
		{HasRx: false, RxRaw: -10000},
	}

	got := hsgqSentinelCandidates(rows)
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), got)
	}
	if got[0] != (HSGQSentinel{Raw: -10000, Count: 3}) {
		t.Errorf("first candidate = %+v, want {-10000 3}", got[0])
	}
	if got[1] != (HSGQSentinel{Raw: 30000, Count: 1}) {
		t.Errorf("second candidate = %+v, want {30000 1}", got[1])
	}
}

func TestHSGQSentinelCandidatesEmptyWhenAllLevelsArePlausible(t *testing.T) {
	rows := []HSGQProbeRow{{HasRx: true, RxRaw: -1250}, {HasRx: true, RxRaw: -2200}}

	if got := hsgqSentinelCandidates(rows); got != nil {
		t.Errorf("candidates = %+v, want nil", got)
	}
}

// MAC is the ONU identity on EPON, and a column that is not a MAC must show up
// as a wrong length rather than as plausible hex.
func TestHSGQProbeRowMACRendering(t *testing.T) {
	row := HSGQProbeRow{MACRaw: []byte{0xEC, 0x23, 0x7B, 0xD7, 0x1F, 0xA8}}

	mac, hex := row.MACRendering()
	if mac != "EC:23:7B:D7:1F:A8" {
		t.Errorf("mac = %q, want EC:23:7B:D7:1F:A8", mac)
	}
	if hex != "EC 23 7B D7 1F A8 " {
		t.Errorf("hex = %q", hex)
	}

	short := HSGQProbeRow{MACRaw: []byte{0x01, 0x02}}
	if mac, _ := short.MACRendering(); mac != "" {
		t.Errorf("mac = %q for a 2-byte value, want empty", mac)
	}
}

func TestHSGQProbeRowIfIndexBreakdown(t *testing.T) {
	got := HSGQProbeRow{IfIndex: 268501248}.IfIndexBreakdown()
	want := "268501248 (0x10010100, bytes 16.1.1.0)"
	if got != want {
		t.Errorf("breakdown = %q, want %q", got, want)
	}
}

// The PON separators are the whole point of reading the label, so they must
// survive. ExtractName would reduce this to "gpon-onu_1111".
func TestPrintableTextPreservesSeparators(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "byte slice keeps slashes and colon", value: []byte("gpon-onu_1/1/1:1"), want: "gpon-onu_1/1/1:1"},
		{name: "string form", value: "gpon-onu_2/3/4:5", want: "gpon-onu_2/3/4:5"},
		{name: "surrounding whitespace trimmed", value: []byte("  xpon 1/1  "), want: "xpon 1/1"},
		{name: "control bytes dropped", value: []byte("pon\x1b[31m1/2"), want: "pon[31m1/2"},
		{name: "non-text value", value: 42, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printableText(tt.value); got != tt.want {
				t.Errorf("printableText(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// End to end over real GetNext traffic: the probe must enumerate from the status
// table, pick the ONU end of the optical row, attach the IF-MIB label that
// resolves the ifIndex layout, and infer the polarity.
func TestProbeHSGQCollectsAndCorrelates(t *testing.T) {
	const up, down = 4001, 4002

	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(hsgqONUStatus, up), Type: gosnmp.Integer, Value: 1},
		{Name: hsgqOID(hsgqONUStatus, down), Type: gosnmp.Integer, Value: 0},

		{Name: hsgqOID(hsgqRxPower, up, 0, 0), Type: gosnmp.Integer, Value: -1250},
		// The OLT end of the same row must not be mistaken for the ONU's level.
		{Name: hsgqOID(hsgqRxPower, up, 65535, 65535), Type: gosnmp.Integer, Value: -2500},
		{Name: hsgqOID(hsgqRxPower, down, 0, 0), Type: gosnmp.Integer, Value: -10000},

		{Name: hsgqOID(hsgqONUName, up), Type: gosnmp.OctetString, Value: []byte("pelanggan-01")},
		{
			Name:  hsgqOID(hsgqONUMAC, up),
			Type:  gosnmp.OctetString,
			Value: []byte{0xEC, 0x23, 0x7B, 0xD7, 0x1F, 0xA8},
		},

		{Name: hsgqOID(ifDescrOID, up), Type: gosnmp.OctetString, Value: []byte("gpon-onu_1/1/1:1")},
		// An interface the status table never reported must be ignored.
		{Name: hsgqOID(ifDescrOID, 9999), Type: gosnmp.OctetString, Value: []byte("uplink-1")},
	})

	report, err := ProbeHSGQ(context.Background(), "127.0.0.1", "public", port)
	if err != nil {
		t.Fatalf("ProbeHSGQ: %v", err)
	}

	if len(report.Rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(report.Rows), report.Rows)
	}

	// Rows are sorted by ifIndex, so the up ONU comes first.
	first := report.Rows[0]
	if first.IfIndex != up {
		t.Fatalf("first row ifIndex = %d, want %d", first.IfIndex, up)
	}
	if first.RxRaw != -1250 {
		t.Errorf("RxRaw = %d, want -1250 (the ONU end, not the OLT's -2500)", first.RxRaw)
	}
	if first.IfLabel != "gpon-onu_1/1/1:1" {
		t.Errorf("IfLabel = %q, want the IF-MIB name that reveals the PON position", first.IfLabel)
	}
	if first.Name != "pelanggan-01" {
		t.Errorf("Name = %q, want pelanggan-01", first.Name)
	}
	if mac, _ := first.MACRendering(); mac != "EC:23:7B:D7:1F:A8" {
		t.Errorf("mac = %q, want EC:23:7B:D7:1F:A8", mac)
	}

	// Status 1 is the group that has light, so it is online; -10000 is the value
	// the dark ONU reports and therefore the sentinel candidate.
	if !report.PolarityConfident || report.OnlineStatusValue != 1 {
		t.Errorf("polarity = (%d, %v), want (1, true)", report.OnlineStatusValue, report.PolarityConfident)
	}
	if len(report.SentinelCandidates) != 1 || report.SentinelCandidates[0].Raw != -10000 {
		t.Errorf("sentinels = %+v, want a single candidate -10000", report.SentinelCandidates)
	}
}

// An empty status table is the headline finding: these third-party OIDs do not
// match the operator's firmware. It must surface as ErrUnsupported, not as an
// empty report that reads like a healthy OLT with no ONUs.
func TestProbeHSGQReportsUnsupportedWhenStatusTableIsEmpty(t *testing.T) {
	_, port := newUncfgAgent(t, []gosnmp.SnmpPDU{
		{Name: hsgqOID(ifDescrOID, 1), Type: gosnmp.OctetString, Value: []byte("uplink-1")},
	})

	_, err := ProbeHSGQ(context.Background(), "127.0.0.1", "public", port)
	if !errors.Is(err, ErrUnsupported) {
		t.Errorf("error = %v, want ErrUnsupported", err)
	}
}
