// Package mikrotik reads RouterOS v7 routers over their REST API.
package mikrotik

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// routerTimeLayout is how RouterOS 7.10+ prints a timestamp such as
// last-started. It carries no zone: the router's clock zone applies.
const routerTimeLayout = "2006-01-02 15:04:05"

var durationUnits = map[string]time.Duration{
	"w":  7 * 24 * time.Hour,
	"d":  24 * time.Hour,
	"h":  time.Hour,
	"m":  time.Minute,
	"s":  time.Second,
	"ms": time.Millisecond,
	"us": time.Microsecond,
}

// ParseDuration reads a RouterOS duration such as "4w6d1h44m21s" or
// "15h53m9s990ms". time.ParseDuration rejects both: it knows neither weeks nor
// days.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	var total time.Duration
	for rest := s; rest != ""; {
		digits := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
		if digits <= 0 {
			return 0, fmt.Errorf("duration %q: expected a number followed by a unit", s)
		}
		n, err := strconv.ParseInt(rest[:digits], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("duration %q: %w", s, err)
		}
		rest = rest[digits:]
		unitEnd := strings.IndexFunc(rest, func(r rune) bool { return r >= '0' && r <= '9' })
		if unitEnd < 0 {
			unitEnd = len(rest)
		}
		unit, ok := durationUnits[rest[:unitEnd]]
		if !ok {
			return 0, fmt.Errorf("duration %q: unknown unit %q", s, rest[:unitEnd])
		}
		total += time.Duration(n) * unit
		rest = rest[unitEnd:]
	}
	return total, nil
}

// ParseInt reads a number the REST API sent as a string, as it sends every
// value.
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// ParseBool reads a RouterOS flag. Anything but "true" or "yes" is false, which
// is also what an absent flag means.
func ParseBool(s string) bool {
	return s == "true" || s == "yes"
}

// ParseOffset reads /system/clock's gmt-offset, such as "+07:00".
func ParseOffset(s string) (*time.Location, error) {
	if len(s) != len("+07:00") || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return nil, fmt.Errorf("gmt-offset %q: want ±HH:MM", s)
	}
	hours, errH := strconv.Atoi(s[1:3])
	minutes, errM := strconv.Atoi(s[4:6])
	if errH != nil || errM != nil {
		return nil, fmt.Errorf("gmt-offset %q: want ±HH:MM", s)
	}
	seconds := hours*3600 + minutes*60
	if s[0] == '-' {
		seconds = -seconds
	}
	return time.FixedZone(s, seconds), nil
}

// ParseTime reads a RouterOS timestamp in the router's own zone.
func ParseTime(s string, loc *time.Location) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	return time.ParseInLocation(routerTimeLayout, s, loc)
}
