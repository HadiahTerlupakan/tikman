package shell

import "testing"

func TestLearnHostname(t *testing.T) {
	cases := []struct {
		dialect Dialect
		line    string
		want    string
	}{
		{DialectVRP, "<SW-TEST>", "SW-TEST"},
		{DialectVRP, "Info: The max number of VTY users is 5.", ""},
		{DialectFiberHome, "SW-TEST>", "SW-TEST"},
		{DialectFiberHome, "SW-TEST#", "SW-TEST"},
		{DialectFiberHome, "SW-TEST(config)#", "SW-TEST"},
		{DialectFiberHome, "Password:", ""},
	}
	for _, c := range cases {
		if got := learnHostname(c.dialect, c.line); got != c.want {
			t.Errorf("learnHostname(%s, %q) = %q, want %q", c.dialect, c.line, got, c.want)
		}
	}
}

func TestPromptMatcher(t *testing.T) {
	cases := []struct {
		dialect Dialect
		line    string
		want    bool
	}{
		{DialectVRP, "<SW-TEST>", true},
		{DialectVRP, "[~SW-TEST]", true},
		{DialectVRP, "[*SW-TEST]", true},
		{DialectVRP, "[SW-TEST]", true},
		{DialectVRP, "[~SW-TEST-10GE1/0/1]", true},
		{DialectVRP, "[*SW-TEST-vlan10]", true},
		{DialectVRP, "<SW-OTHER>", false},
		{DialectVRP, "<SW-TEST> display version", false},
		{DialectVRP, "[~SW-TESTX]", false},
		{DialectFiberHome, "SW-TEST>", true},
		{DialectFiberHome, "SW-TEST#", true},
		{DialectFiberHome, "SW-TEST(config)#", true},
		{DialectFiberHome, "SW-TEST(config-if-ge-1/1)#", true},
		{DialectFiberHome, "SW-TESTX#", false},
		{DialectFiberHome, "vlan 10 name SW-TEST#", false},
	}
	for _, c := range cases {
		matcher := newPromptMatcher(c.dialect, "SW-TEST")
		if got := matcher.matches(c.line); got != c.want {
			t.Errorf("%s matches(%q) = %v, want %v", c.dialect, c.line, got, c.want)
		}
	}
}
