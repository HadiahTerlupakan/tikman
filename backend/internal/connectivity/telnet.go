package connectivity

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// TelnetTest performs Telnet connectivity and authentication test.
//
// This function:
// 1. Establishes TCP connection to the Telnet port
// 2. Reads initial prompt/banner
// 3. Sends username and waits for password prompt
// 4. Sends password and verifies successful login
//
// Returns:
//   - nil if connection and authentication succeed
//   - error if connection fails, authentication fails, or times out
func TelnetTest(ipAddress string, port int, username, password string, timeout time.Duration) error {
	address := net.JoinHostPort(ipAddress, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return fmt.Errorf("timeout after %s", timeout)
		}
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Set overall deadline
	_ = conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)

	// Helper function to read until we see a specific pattern or timeout
	readUntilPattern := func(patterns []string, maxWait time.Duration) (string, error) {
		var buffer strings.Builder
		deadline := time.Now().Add(maxWait)

		for time.Now().Before(deadline) {
			_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

			b, err := reader.ReadByte()
			if err != nil {
				if err == io.EOF {
					break
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Check current buffer for patterns
					current := strings.ToLower(buffer.String())
					for _, pattern := range patterns {
						if strings.Contains(current, strings.ToLower(pattern)) {
							return buffer.String(), nil
						}
					}
					continue
				}
				return buffer.String(), err
			}

			buffer.WriteByte(b)

			// Check if we've seen any of the patterns
			current := strings.ToLower(buffer.String())
			for _, pattern := range patterns {
				if strings.Contains(current, strings.ToLower(pattern)) {
					return buffer.String(), nil
				}
			}
		}

		return buffer.String(), nil
	}

	// Step 1: Read initial banner and wait for username prompt
	banner, err := readUntilPattern([]string{"username:", "login:", "user:"}, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to read banner: %w", err)
	}

	bannerLower := strings.ToLower(banner)
	if !strings.Contains(bannerLower, "username") &&
	   !strings.Contains(bannerLower, "login") &&
	   !strings.Contains(bannerLower, "user") {
		return fmt.Errorf("expected username prompt, got: %s", banner)
	}

	// Step 2: Send username
	_, err = conn.Write([]byte(username + "\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send username: %w", err)
	}

	// Step 3: Wait for password prompt
	passwordPrompt, err := readUntilPattern([]string{"password:"}, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to read password prompt: %w", err)
	}

	if !strings.Contains(strings.ToLower(passwordPrompt), "password") {
		return fmt.Errorf("expected password prompt, got: %s", passwordPrompt)
	}

	// Step 4: Send password
	_, err = conn.Write([]byte(password + "\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send password: %w", err)
	}

	// Step 5: Read response and verify login success
	response, err := readUntilPattern([]string{">", "#", "$", "error", "incorrect", "failed", "denied"}, 3*time.Second)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Check for authentication failure indicators
	responseLower := strings.ToLower(response)
	if strings.Contains(responseLower, "login incorrect") ||
	   strings.Contains(responseLower, "authentication failed") ||
	   strings.Contains(responseLower, "access denied") ||
	   strings.Contains(responseLower, "invalid password") ||
	   strings.Contains(responseLower, "login failed") ||
	   strings.Contains(responseLower, "error") ||
	   strings.Contains(responseLower, "bad password") {
		return fmt.Errorf("authentication failed")
	}

	// Check if we got a prompt (successful login indicator)
	if strings.Contains(response, ">") ||
	   strings.Contains(response, "#") ||
	   strings.Contains(response, "$") {
		return nil
	}

	// If we have some response but no clear indicator, assume success
	if len(response) > 0 {
		return nil
	}

	return fmt.Errorf("unable to verify login success: no response received")
}
