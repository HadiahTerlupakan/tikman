package connectivity

import "testing"

func TestFormatAutofindSerial(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "live C300 autofind value",
			value: []byte{0x48, 0x57, 0x54, 0x43, 0xB4, 0x03, 0xE8, 0xA0},
			want:  "HWTCB403E8A0",
		},
		{
			name:  "ZTE vendor prefix keeps uppercase hex",
			value: []byte{0x5A, 0x54, 0x45, 0x47, 0xCA, 0xFF, 0xC2, 0xFD},
			want:  "ZTEGCAFFC2FD",
		},
		{
			name:  "zero identifier bytes are still rendered",
			value: []byte{0x48, 0x57, 0x54, 0x43, 0x00, 0x00, 0x0A, 0x0B},
			want:  "HWTC00000A0B",
		},
		{
			name:  "string carrying the same bytes",
			value: "HWTC\xb4\x03\xe8\xa0",
			want:  "HWTCB403E8A0",
		},
		{
			name:  "short value rejected",
			value: []byte{0x48, 0x57, 0x54, 0x43},
			want:  "",
		},
		{
			name:  "long value rejected",
			value: []byte{0x48, 0x57, 0x54, 0x43, 0xB4, 0x03, 0xE8, 0xA0, 0x01},
			want:  "",
		},
		{
			name:  "non-printable vendor bytes rejected",
			value: []byte{0x00, 0x01, 0x02, 0x03, 0xB4, 0x03, 0xE8, 0xA0},
			want:  "",
		},
		{
			name:  "integer value rejected",
			value: 42,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAutofindSerial(tt.value); got != tt.want {
				t.Errorf("formatAutofindSerial() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseUncfgSuffix(t *testing.T) {
	tests := []struct {
		name     string
		suffix   string
		wantSlot int
		wantPort int
		wantSeq  int
		wantOK   bool
	}{
		{
			name:     "live slot 3 port 1 sequence 1",
			suffix:   "268632320.1",
			wantSlot: 3,
			wantPort: 1,
			wantSeq:  1,
			wantOK:   true,
		},
		{
			name:     "slot 4 port 5",
			suffix:   "268698880.2",
			wantSlot: 4,
			wantPort: 5,
			wantSeq:  2,
			wantOK:   true,
		},
		{
			name:   "missing sequence rejected",
			suffix: "268632320",
			wantOK: false,
		},
		{
			name:   "non-numeric ifIndex rejected",
			suffix: "abc.1",
			wantOK: false,
		},
		{
			name:   "non-numeric sequence rejected",
			suffix: "268632320.x",
			wantOK: false,
		},
		{
			name:   "wrong frame byte rejected",
			suffix: "1.1",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseUncfgSuffix(tt.suffix)
			if ok != tt.wantOK {
				t.Fatalf("parseUncfgSuffix() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got.slot != tt.wantSlot || got.port != tt.wantPort || got.seq != tt.wantSeq {
				t.Errorf("parseUncfgSuffix() = %+v, want slot=%d port=%d seq=%d",
					got, tt.wantSlot, tt.wantPort, tt.wantSeq)
			}
		})
	}
}

func TestSortUnconfiguredONUs(t *testing.T) {
	onus := []UnconfiguredONU{
		{Slot: 4, Port: 1, SerialNumber: "HWTC00000001"},
		{Slot: 3, Port: 2, SerialNumber: "HWTC00000002"},
		{Slot: 3, Port: 1, SerialNumber: "ZTEG00000003"},
		{Slot: 3, Port: 1, SerialNumber: "HWTC00000004"},
	}

	sortUnconfiguredONUs(onus)

	want := []string{"HWTC00000004", "ZTEG00000003", "HWTC00000002", "HWTC00000001"}
	for i, serial := range want {
		if onus[i].SerialNumber != serial {
			t.Errorf("index %d = %q, want %q", i, onus[i].SerialNumber, serial)
		}
	}
}
