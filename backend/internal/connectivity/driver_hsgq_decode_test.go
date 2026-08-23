package connectivity

import (
	"context"
	"errors"
	"testing"
)

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
