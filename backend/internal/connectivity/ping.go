package connectivity

import (
	"fmt"
	"time"

	"github.com/go-ping/ping" //nolint:staticcheck // No maintained alternative available
)

// PingTest performs an ICMP ping test to verify host reachability
func PingTest(ipAddress string, timeout time.Duration) error {
	pinger, err := ping.NewPinger(ipAddress)
	if err != nil {
		return fmt.Errorf("failed to create pinger: %w", err)
	}

	pinger.Count = 3
	pinger.Timeout = timeout
	pinger.SetPrivileged(false) // Use unprivileged mode (works without root)

	err = pinger.Run()
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return fmt.Errorf("ping timeout: host unreachable (0/%d packets received)", stats.PacketsSent)
	}

	return nil
}
