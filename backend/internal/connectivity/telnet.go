package connectivity

import (
	"fmt"
	"net"
	"time"
)

// TelnetTest performs TCP connectivity test to a Telnet port.
//
// IMPORTANT: This function only tests TCP connectivity (port reachability).
// It does NOT perform authentication or validate credentials.
//
// The username and password parameters are accepted for API consistency with
// other connection test functions (SSHTest, etc.) but are intentionally unused.
// Authentication testing would require full Telnet protocol implementation.
//
// Returns:
//   - nil if TCP connection succeeds (port is open and accepting connections)
//   - error if connection fails or times out
func TelnetTest(ipAddress string, port int, username, password string, timeout time.Duration) error {
	// Note: username and password parameters are unused - this is TCP connectivity only
	_ = username // Explicitly mark as unused to prevent accidental future use
	_ = password // Explicitly mark as unused to prevent accidental logging

	address := net.JoinHostPort(ipAddress, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("timeout after %s", timeout)
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Set deadline for read/write operations
	conn.SetDeadline(time.Now().Add(timeout))

	return nil
}
