package connectivity

import (
	"bufio"
	"fmt"
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
	defer conn.Close()

	// Set overall deadline
	conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)

	// Step 1: Read initial banner/prompt (wait for username prompt)
	var buffer strings.Builder
	readTimeout := time.Now().Add(2 * time.Second)
	for time.Now().Before(readTimeout) {
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		line, err := reader.ReadString('\n')
		if err != nil {
			// Timeout or EOF is expected when waiting for prompt
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Check if we have any data
				if buffer.Len() > 0 {
					break
				}
				continue
			}
			// Non-timeout error
			if buffer.Len() == 0 {
				return fmt.Errorf("failed to read banner: %w", err)
			}
			break
		}
		buffer.WriteString(line)

		// Check if we got a username prompt
		lowerLine := strings.ToLower(buffer.String())
		if strings.Contains(lowerLine, "login:") ||
		   strings.Contains(lowerLine, "username:") ||
		   strings.Contains(lowerLine, "user:") {
			break
		}
	}

	// Step 2: Send username
	_, err = conn.Write([]byte(username + "\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send username: %w", err)
	}

	// Step 3: Wait for password prompt
	buffer.Reset()
	readTimeout = time.Now().Add(2 * time.Second)
	for time.Now().Before(readTimeout) {
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if buffer.Len() > 0 {
					break
				}
				continue
			}
			if buffer.Len() == 0 {
				return fmt.Errorf("failed to read password prompt: %w", err)
			}
			break
		}
		buffer.WriteString(line)

		// Check for password prompt
		lowerLine := strings.ToLower(buffer.String())
		if strings.Contains(lowerLine, "password:") {
			break
		}
	}

	// Verify we got password prompt
	if !strings.Contains(strings.ToLower(buffer.String()), "password") {
		return fmt.Errorf("expected password prompt, got: %s", buffer.String())
	}

	// Step 4: Send password
	_, err = conn.Write([]byte(password + "\r\n"))
	if err != nil {
		return fmt.Errorf("failed to send password: %w", err)
	}

	// Step 5: Read response and verify login success
	buffer.Reset()
	readTimeout = time.Now().Add(3 * time.Second)
	for time.Now().Before(readTimeout) {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		line, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if buffer.Len() > 0 {
					// We have some data, check it
					break
				}
				continue
			}
			// EOF or other error
			if buffer.Len() > 0 {
				break
			}
			return fmt.Errorf("failed to read login response: %w", err)
		}
		buffer.WriteString(line)

		// Check for successful login indicators (prompt symbols)
		response := buffer.String()
		if strings.Contains(response, ">") ||
		   strings.Contains(response, "#") ||
		   strings.Contains(response, "$") {
			// Looks like a shell prompt, login successful
			break
		}
	}

	// Check for authentication failure indicators
	response := strings.ToLower(buffer.String())
	if strings.Contains(response, "login incorrect") ||
	   strings.Contains(response, "authentication failed") ||
	   strings.Contains(response, "access denied") ||
	   strings.Contains(response, "invalid password") ||
	   strings.Contains(response, "login failed") {
		return fmt.Errorf("authentication failed")
	}

	// Check if we got a prompt (successful login indicator)
	if strings.Contains(buffer.String(), ">") ||
	   strings.Contains(buffer.String(), "#") ||
	   strings.Contains(buffer.String(), "$") {
		return nil
	}

	// If we reach here and have some response, assume success
	// (some devices may not show standard prompts)
	if buffer.Len() > 0 {
		return nil
	}

	return fmt.Errorf("unable to verify login success: no response received")
}
