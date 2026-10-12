package services

import "time"

// InterfaceRate returns bits per second between two samples of an octet
// counter, and false when the pair cannot give an honest rate: no time passed,
// or the counter went backwards because the router rebooted.
func InterfaceRate(prevOctets, curOctets uint64, prevAt, curAt time.Time) (float64, bool) {
	elapsed := curAt.Sub(prevAt).Seconds()
	if elapsed <= 0 || curOctets < prevOctets {
		return 0, false
	}
	return float64(curOctets-prevOctets) * 8 / elapsed, true
}
