package linkpreview

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The wa process sits on the Docker network beside postgres, redis and api,
// and one gateway hop from the host. A CS types the URL, so without this the
// server fetches whatever they name — including its own neighbours.
func TestBlockedAddresses(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"10.0.0.5",        // RFC1918
		"172.20.0.3",      // the Docker network this very process is on
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // cloud metadata — the one that leaks credentials
		"fe80::1",         // link-local v6
		"fd00::1",         // unique-local v6
		"0.0.0.0",         // unspecified
		"100.64.0.1",      // carrier-grade NAT
	}
	for _, s := range blocked {
		assert.True(t, blockedIP(net.ParseIP(s)), "%s must be refused", s)
	}
}

// The plant itself, as deployed on 2026-09-05. api carries wg0 and can reach
// every one of these, so a preview resolved there must refuse them. They are
// all RFC1918 today and the generic rules above already cover them — these
// cases exist so that stays true, and so a loosened guard fails here rather
// than on an OLT.
func TestTheTunnelAndThePlantAreRefused(t *testing.T) {
	plant := map[string]string{
		"10.88.0.1":      "wg0 on the VPS",
		"10.88.0.3":      "a site's tunnel address",
		"10.10.17.4":     "a site LAN behind the tunnel",
		"192.168.220.22": "the Depok OLT",
		"172.30.30.2":    "the Bekasi OLT",
		"172.30.30.3":    "the Cariu OLT",
		"172.30.30.10":   "the Pangkal Pinang OLT",
	}
	for addr, what := range plant {
		assert.True(t, blockedIP(net.ParseIP(addr)), "%s (%s) must be refused", addr, what)
	}
}

func TestPublicAddressesAreAllowed(t *testing.T) {
	allowed := []string{"1.1.1.1", "8.8.8.8", "157.240.0.1", "2606:4700::1111"}
	for _, s := range allowed {
		assert.False(t, blockedIP(net.ParseIP(s)), "%s must be allowed", s)
	}
}

// An unparseable address is refused rather than allowed: a guard that fails
// open is not a guard.
func TestAnUnreadableAddressIsRefused(t *testing.T) {
	assert.True(t, blockedIP(nil))
}
