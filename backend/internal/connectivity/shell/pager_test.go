package shell

import (
	"strings"
	"testing"
)

func TestRenderLineAppliesErasure(t *testing.T) {
	cases := map[string]string{
		"plain": "plain",
		"  ---- More ----\x1b[16D                \x1b[16Dinterface 10GE1/0/2": "interface 10GE1/0/2",
		" --More-- \b\b\b\b\b\b\b\b\b\b          \b\b\b\b\b\b\b\b\b\bvlan 20": "vlan 20",
		"  ---- More ----\rvlan 30":       "vlan 30",
		"\x1b[?25hdescription to SW-EDGE": "description to SW-EDGE",
	}
	for raw, want := range cases {
		if got := renderLine(raw); got != want {
			t.Errorf("renderLine(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestCleanOutputDropsUnerasedPagerAndUnwraps(t *testing.T) {
	long := strings.Repeat("a", terminalWidth)
	raw := "display x\r\nline one\r\n  ---- More ----\r\nline two\r\n" + long + "\r\nbbb\r\n<SW-TEST>"

	got := commandOutput(raw, "display x")

	want := "line one\nline two\n" + long + "bbb"
	if got != want {
		t.Errorf("commandOutput = %q, want %q", got, want)
	}
}

func TestPendingReply(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"line\r\n  ---- More ----", " "},
		{"line\r\n --More-- ", " "},
		{"Change now? [Y/N]:", "N"},
		{"Continue? [y/n]: ", "N"},
		{"display version\r\nHuawei", ""},
	}
	for _, c := range cases {
		if got := pendingReply(c.text); got != c.want {
			t.Errorf("pendingReply(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}
