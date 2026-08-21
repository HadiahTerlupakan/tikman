package connectivity

import "testing"

// ifIndex 268632320 = 0x10030100 -> slot 3, port 1 (read from a live ZTE C300)
const liveIfIndex = 268632320

func TestParseZxGponSuffix(t *testing.T) {
	tests := []struct {
		name    string
		oid     string
		base    string
		wantLoc ONTLocation
		wantOK  bool
	}{
		{
			name:    "phase state suffix ifIndex.onuIndex",
			oid:     "1.3.6.1.4.1.3902.1012.3.28.2.1.4.268632320.55",
			base:    OID_ZXAN_ONU_PHASE_STATE_TABLE,
			wantLoc: ONTLocation{Slot: 3, Port: 1, ONTID: 55},
			wantOK:  true,
		},
		{
			name:    "power suffix carries trailing sub-instance",
			oid:     "1.3.6.1.4.1.3902.1012.3.50.12.1.1.10.268632320.18.1",
			base:    OID_ZXGPON_ONU_RX_POWER_TABLE,
			wantLoc: ONTLocation{Slot: 3, Port: 1, ONTID: 18},
			wantOK:  true,
		},
		{
			name:   "oid outside the base is rejected",
			oid:    "1.3.6.1.2.1.1.1.0",
			base:   OID_ZXAN_ONU_PHASE_STATE_TABLE,
			wantOK: false,
		},
		{
			name:   "non-ZXGPON ifIndex marker is rejected",
			oid:    "1.3.6.1.4.1.3902.1012.3.28.2.1.4.285280008.1",
			base:   OID_ZXAN_ONU_PHASE_STATE_TABLE,
			wantOK: false,
		},
		{
			name:   "suffix with a single segment is rejected",
			oid:    "1.3.6.1.4.1.3902.1012.3.28.2.1.4.268632320",
			base:   OID_ZXAN_ONU_PHASE_STATE_TABLE,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, ok := parseZxGponSuffix(tt.oid, tt.base)
			if ok != tt.wantOK {
				t.Fatalf("parseZxGponSuffix ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && loc != tt.wantLoc {
				t.Errorf("parseZxGponSuffix = %+v, want %+v", loc, tt.wantLoc)
			}
		})
	}
}

// Raw values below were read from a live ZTE C300 at 113.192.1.98.
func TestDecodeZxGponPower(t *testing.T) {
	tests := []struct {
		name    string
		raw     int64
		wantDBm float64
		wantNil bool
	}{
		{name: "ONU 55 rx", raw: 2525, wantDBm: -24.95},
		{name: "ONU 55 tx", raw: 16107, wantDBm: 2.214},
		{name: "ONU 18 rx", raw: 880, wantDBm: -28.24},
		{name: "no signal sentinel 65535", raw: 65535, wantNil: true},
		{name: "no signal at sentinel boundary", raw: 30000, wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeZxGponPower(tt.raw)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("decodeZxGponPower(%d) = %v, want nil (no signal)", tt.raw, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("decodeZxGponPower(%d) = nil, want %v", tt.raw, tt.wantDBm)
			}
			if diff := *got - tt.wantDBm; diff > 0.001 || diff < -0.001 {
				t.Errorf("decodeZxGponPower(%d) = %v, want %v", tt.raw, *got, tt.wantDBm)
			}
		})
	}
}

func TestEncodeZxGponIfIndexMatchesLiveDevice(t *testing.T) {
	// The ZXGPON frame marker is 0x10, not the rack/frame number.
	if got := encodeZxGponIfIndex(0x10, 3, 1); got != liveIfIndex {
		t.Errorf("encodeZxGponIfIndex(0x10, 3, 1) = %d (0x%08x), want %d (0x%08x)",
			got, got, liveIfIndex, liveIfIndex)
	}
}

func TestToInt64(t *testing.T) {
	if v, ok := toInt64(3); !ok || v != 3 {
		t.Errorf("toInt64(int 3) = %v, %v; want 3, true", v, ok)
	}
	if v, ok := toInt64(uint(7)); !ok || v != 7 {
		t.Errorf("toInt64(uint 7) = %v, %v; want 7, true", v, ok)
	}
	// A nil value is what the device returns for an absent instance; it must not
	// be silently coerced into a phase state.
	if _, ok := toInt64(nil); ok {
		t.Error("toInt64(nil) reported ok=true, want false")
	}
	if _, ok := toInt64("3"); ok {
		t.Error("toInt64(string) reported ok=true, want false")
	}
}
