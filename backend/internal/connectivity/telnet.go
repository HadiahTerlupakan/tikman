package connectivity

import (
	"fmt"
	"net"
	"time"
)

// TelnetTest performs Telnet connection test
func TelnetTest(ipAddress string, port int, username, password string, timeout time.Duration) error {
	address := fmt.Sprintf("%s:%d", ipAddress, port)
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

	// Basic connection test - we can't easily test auth without full telnet protocol implementation
	// This verifies the port is open and accepting connections
	return nil
}
