package mikrotik

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"4w6d1h44m21s", 34*24*time.Hour + time.Hour + 44*time.Minute + 21*time.Second},
		{"15h53m9s990ms", 15*time.Hour + 53*time.Minute + 9*time.Second + 990*time.Millisecond},
		{"1d14h52m22s720ms", 38*time.Hour + 52*time.Minute + 22*time.Second + 720*time.Millisecond},
		{"1m30s", 90 * time.Second},
		{"0s", 0},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestParseDurationRejectsWhatIsNotADuration(t *testing.T) {
	for _, in := range []string{"", "infinity", "12", "5x", "h5"} {
		_, err := ParseDuration(in)
		assert.Error(t, err, in)
	}
}

func TestParseInt(t *testing.T) {
	got, err := ParseInt("1093192")
	require.NoError(t, err)
	assert.EqualValues(t, 1093192, got)

	_, err = ParseInt("")
	assert.Error(t, err)
	_, err = ParseInt("12k")
	assert.Error(t, err)
}

func TestParseBool(t *testing.T) {
	assert.True(t, ParseBool("true"))
	assert.True(t, ParseBool("yes"))
	assert.False(t, ParseBool("false"))
	assert.False(t, ParseBool(""))
}

// last-started carries no zone. A router on WIB that says 16:06:43 means
// 09:06:43 UTC, and reading it as UTC would place every event seven hours late.
func TestParseTimeUsesTheRoutersOffset(t *testing.T) {
	loc, err := ParseOffset("+07:00")
	require.NoError(t, err)

	got, err := ParseTime("2026-10-11 16:06:43", loc)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 10, 11, 9, 6, 43, 0, time.UTC), got.UTC())
}

func TestParseOffset(t *testing.T) {
	for in, seconds := range map[string]int{"+07:00": 7 * 3600, "-03:30": -(3*3600 + 1800), "+00:00": 0} {
		loc, err := ParseOffset(in)
		require.NoError(t, err, in)
		_, offset := time.Date(2026, 1, 1, 0, 0, 0, 0, loc).Zone()
		assert.Equal(t, seconds, offset, in)
	}
	for _, in := range []string{"", "7", "+7", "WIB"} {
		_, err := ParseOffset(in)
		assert.Error(t, err, in)
	}
}

func TestParseTimeRejectsEmpty(t *testing.T) {
	_, err := ParseTime("", time.UTC)
	assert.Error(t, err)
}
