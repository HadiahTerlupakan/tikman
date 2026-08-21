package connectivity

import "testing"

// Live values read from a ZTE C300: ifIndex 285278977 = 0x11010301
// -> base 0x11010000 + 3*256 + 1 -> slot 3, port 1.
func TestParseOnuIDSuffix(t *testing.T) {
	tests := []struct {
		name    string
		oid     string
		base    string
		wantLoc ONTLocation
		wantOK  bool
	}{
		{
			name:    "octet rate suffix decodes slot 3 port 1",
			oid:     "1.3.6.1.4.1.3902.1082.500.4.2.2.2.1.3.285278977.4",
			base:    OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE,
			wantLoc: ONTLocation{Slot: 3, Port: 1, ONTID: 4},
			wantOK:  true,
		},
		{
			name:    "second pon port on same slot",
			oid:     "1.3.6.1.4.1.3902.1082.500.4.2.2.2.1.46.285278978.128",
			base:    OID_ZXGPON_ONU_TX_OCTET_RATE_TABLE,
			wantLoc: ONTLocation{Slot: 3, Port: 2, ONTID: 128},
			wantOK:  true,
		},
		{
			name:   "ifIndex below ONU-ID base is rejected",
			oid:    "1.3.6.1.4.1.3902.1082.500.4.2.2.2.1.3.285278207.1",
			base:   OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE,
			wantOK: false,
		},
		{
			name:   "TYPE-space ifIndex is rejected",
			oid:    "1.3.6.1.4.1.3902.1082.500.4.2.2.2.1.3.268632320.1",
			base:   OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE,
			wantOK: false,
		},
		{
			name:   "oid outside the base is rejected",
			oid:    "1.3.6.1.4.1.3902.1082.500.4.2.2.2.1.46.285278977.1",
			base:   OID_ZXGPON_ONU_RX_OCTET_RATE_TABLE,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, ok := parseOnuIDSuffix(tt.oid, tt.base)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && loc != tt.wantLoc {
				t.Errorf("loc = %+v, want %+v", loc, tt.wantLoc)
			}
		})
	}
}
