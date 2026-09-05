// Package linkpreview turns a URL a CS typed into the title, description and
// thumbnail WhatsApp shows above a message.
//
// Everything here runs against an address the CS chose, so the guard is the
// point of the package rather than a detail of it: this process sits on the
// Docker network beside postgres, redis and api, one hop from the host.
package linkpreview

import "net"

// carrierGradeNAT is RFC 6598 space. It is not RFC1918, so IsPrivate misses
// it, and some hosting providers put internal services there.
var carrierGradeNAT = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// blockedIP reports whether an address must not be fetched.
//
// It refuses a nil address rather than allowing it: a guard that fails open on
// input it could not read is not a guard.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		carrierGradeNAT.Contains(ip)
}
