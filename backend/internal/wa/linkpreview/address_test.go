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
