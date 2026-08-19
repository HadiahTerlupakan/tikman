package utils

import "testing"

// Verified ZXGPON-MIB ifIndex sample from NetManeger field verification:
// ifIndex 268896256 = hex 0x10070800 -> slot=7, port=8
func TestEncodeZxGponIfIndex(t *testing.T) {
	tests := []struct {
		name string
		slot int
		port int
		want uint32
	}{
		{"slot 7 port 8 (verified C300)", 7, 8, 0x10070800},
		{"slot 8 port 1", 8, 1, 0x10080100},
		{"slot 9 port 4", 9, 4, 0x10090400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeZxGponIfIndex(tt.slot, tt.port)
			if got != tt.want {
				t.Errorf("EncodeZxGponIfIndex(%d, %d) = 0x%08x, want 0x%08x",
					tt.slot, tt.port, got, tt.want)
			}
		})
	}
}

func TestDecodeZxGponIfIndex(t *testing.T) {
	slot, port, ok := DecodeZxGponIfIndex(0x10070800)
	if !ok {
		t.Fatal("DecodeZxGponIfIndex(0x10070800) returned ok=false, want true")
	}
	if slot != 7 || port != 8 {
		t.Errorf("DecodeZxGponIfIndex(0x10070800) = slot %d port %d, want slot 7 port 8", slot, port)
	}

	// Marker other than 0x10 belongs to a different card type and must be rejected
	if _, _, ok := DecodeZxGponIfIndex(0x11010708); ok {
		t.Error("DecodeZxGponIfIndex(0x11010708) returned ok=true, want false (ZTE-AN marker, not ZXGPON)")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for slot := 1; slot <= 20; slot++ {
		for port := 1; port <= 16; port++ {
			gotSlot, gotPort, ok := DecodeZxGponIfIndex(EncodeZxGponIfIndex(slot, port))
			if !ok || gotSlot != slot || gotPort != port {
				t.Fatalf("round trip failed for slot %d port %d: got slot %d port %d ok %v",
					slot, port, gotSlot, gotPort, ok)
			}
		}
	}
}

// Phase state values verified against `show gpon onu state` on ZTE C300 V2.1.0
func TestStatusMap(t *testing.T) {
	tests := []struct {
		phaseState int
		want       string
	}{
		{3, "online"},
		{4, "dying_gasp"},
		{6, "offline"},
		{1, "los"},
		{2, "unknown"},
		{5, "unknown"},
		{0, "unknown"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		if got := StatusMap(tt.phaseState); got != tt.want {
			t.Errorf("StatusMap(%d) = %q, want %q", tt.phaseState, got, tt.want)
		}
	}
}
