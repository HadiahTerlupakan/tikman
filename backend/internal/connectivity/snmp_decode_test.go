package connectivity

import (
	"fmt"
	"testing"
)

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

// Raw values below were read from a live ZTE C300, not invented.
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

func TestDecodeOnuIDIfIndex(t *testing.T) {
	tests := []struct {
		ifIndex uint32
		slot    int
		port    int
		ok      bool
	}{
		// Valid cases from encode example (0x10030100 = frame=0x10=16, slot=3, port=1)
		{ifIndex: 0x10030100, slot: 3, port: 1, ok: true},
		{ifIndex: 0x10010100, slot: 1, port: 1, ok: true},
		{ifIndex: 0x10ff0100, slot: 255, port: 1, ok: true}, // max slot
		{ifIndex: 0x1003ff00, slot: 3, port: 255, ok: true}, // max port

		// Edge cases that should fail per implementation logic
		{ifIndex: 0x10000100, slot: 0, port: 1, ok: false}, // slot=0 invalid
		{ifIndex: 0x10010000, slot: 1, port: 0, ok: false}, // port=0 invalid
		{ifIndex: 0x00000000, slot: 0, port: 0, ok: false}, // zero value
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ifIndex=%08x", tt.ifIndex), func(t *testing.T) {
			slot, port, ok := decodeOnuIDIfIndex(tt.ifIndex)
			if ok != tt.ok {
				t.Errorf("ok mismatch: got %v, want %v", ok, tt.ok)
			}
			if ok && (slot != tt.slot || port != tt.port) {
				t.Errorf("decode result: got slot=%d port=%d, want slot=%d port=%d",
					slot, port, tt.slot, tt.port)
			}
		})
	}
}

func TestDecodeOnuTypeIfIndex(t *testing.T) {
	tests := []struct {
		name    string
		ifIndex uint32
		slot    int
		port    int
		ok      bool
	}{
		{
			name:    "valid ONU-type ifIndex for slot=1, port=1",
			ifIndex: OnuTypeIfIndexBase + 0x00010100, // offset = 0x100, slot=1, port=1
			slot:    1,
			port:    1,
			ok:      true,
		},
		{
			name:    "valid ONU-type ifIndex for slot=2, port=3",
			ifIndex: OnuTypeIfIndexBase + 0x00020300, // offset = 0x203*stride, slot=2, port=3
			slot:    2,
			port:    3,
			ok:      true,
		},
		{
			name:    "below base is rejected",
			ifIndex: OnuTypeIfIndexBase - 1,
			slot:    0,
			port:    0,
			ok:      false,
		},
		{
			name:    "zero slot derived is rejected",
			ifIndex: OnuTypeIfIndexBase, // exact base, offset=0 -> slot=0
			slot:    0,
			port:    0,
			ok:      false,
		},
		{
			name:    "zero port derived is rejected",
			ifIndex: OnuTypeIfIndexBase + 0x01000000, // offset aligned to stride boundary -> port=0
			slot:    1,
			port:    0,
			ok:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot, port, ok := decodeOnuTypeIfIndex(tt.ifIndex)
			if ok != tt.ok {
				t.Errorf("ok mismatch: got %v, want %v", ok, tt.ok)
			}
			if ok && (slot != tt.slot || port != tt.port) {
				t.Errorf("decode result: got slot=%d port=%d, want slot=%d port=%d",
					slot, port, tt.slot, tt.port)
			}
		})
	}
}

func TestEncodeOnuIDIfIndex(t *testing.T) {
	tests := []struct {
		frame int
		slot  int
		port  int
		want  uint32
	}{
		// encode uses hardcoded prefix 0x1101 for ONU-ID space
		{frame: 1, slot: 1, port: 1, want: 0x11010101},
		{frame: 1, slot: 3, port: 1, want: 0x11010301},
		{frame: 1, slot: 255, port: 255, want: 0x1101FFFF},
		{frame: 15, slot: 1, port: 1, want: 0xF1010101}, // high frame affects top nibble

		// Zero slot/port still encodes but decodes back as ok=false
		{frame: 1, slot: 0, port: 1, want: 0x11010001},
		{frame: 0, slot: 1, port: 1, want: 0x11010101},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("frame=%d_slot=%d_port=%d", tt.frame, tt.slot, tt.port), func(t *testing.T) {
			got := encodeOnuIDIfIndex(tt.frame, tt.slot, tt.port)
			if got != tt.want {
				t.Errorf("encode result: got 0x%08x, want 0x%08x", got, tt.want)
			}
		})
	}
}

func TestParseZteHexTimestamp(t *testing.T) {
	// Format: 8 bytes = year(2) month(1) day(1) hour(1) min(1) sec(1) ms(2)
	tests := []struct {
		name        string
		hexBytes    []byte
		wantErr     bool
		expectYear  int
		expectMonth int
		expectDay   int
	}{
		// Example: 2022-12-15 14:30:40
		{
			name:        "valid timestamp 2022-12-15 14:30:40",
			hexBytes:    []byte{0x07, 0xE6, 0x0C, 0x0F, 0x0E, 0x1E, 0x28, 0x00},
			wantErr:     false,
			expectYear:  2022,
			expectMonth: 12,
			expectDay:   15,
		},
		{
			name:       "year minimum 1970-01-01",
			hexBytes:   []byte{0x07, 0xB2, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00},
			wantErr:    false,
			expectYear: 1970,
		},
		{
			name:       "year maximum 2100-12-31",
			hexBytes:   []byte{0x08, 0x34, 0x0C, 0x1F, 0x00, 0x00, 0x00, 0x00},
			wantErr:    false,
			expectYear: 2100,
		},
		// Invalid cases
		{
			name:     "year below minimum",
			hexBytes: []byte{0x07, 0xB1, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00}, // 1969 < 1970
			wantErr:  true,
		},
		{
			name:     "year above maximum",
			hexBytes: []byte{0x08, 0x35, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00}, // 2101 > 2100
			wantErr:  true,
		},
		{
			name:     "invalid month zero",
			hexBytes: []byte{0x07, 0xE6, 0x00, 0x0F, 0x00, 0x00, 0x00, 0x00}, // month=0
			wantErr:  true,
		},
		{
			name:     "invalid day zero",
			hexBytes: []byte{0x07, 0xE6, 0x0C, 0x00, 0x00, 0x00, 0x00, 0x00}, // day=0
			wantErr:  true,
		},
		{
			name:     "too short",
			hexBytes: []byte{0x07, 0xE6}, // length 2 < 8
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseZteHexTimestamp(tt.hexBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("error: got err=%v, wantErr=%v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.Year() != tt.expectYear {
				t.Errorf("year: got %d, want %d", got.Year(), tt.expectYear)
			}
			if tt.expectMonth != 0 && int(got.Month()) != tt.expectMonth {
				t.Errorf("month: got %d, want %d", got.Month(), tt.expectMonth)
			}
			if tt.expectDay != 0 && int(got.Day()) != tt.expectDay {
				t.Errorf("day: got %d, want %d", got.Day(), tt.expectDay)
			}
		})
	}
}
