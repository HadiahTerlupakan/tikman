package connectivity

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// CommandResult represents the result of a single command execution
type CommandResult struct {
	Success  bool
	Output   string
	Error    string
	Duration time.Duration
}

// OLTCommander provides vendor-specific configuration commands
type OLTCommander interface {
	// ExecuteCommand sends a single configuration command and returns the result
	ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error)
	// BatchExecute sends multiple commands sequentially and returns results
	BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error)
}

// loginPromptWait bounds each step of the login handshake. A C300 answers
// option negotiation and prints its banner well inside this.
const loginPromptWait = 8 * time.Second

// TelnetCommander implements OLTCommander via Telnet session
type TelnetCommander struct {
	conn     net.Conn
	reader   *bufio.Reader
	host     string
	port     int
	timeout  time.Duration
	username string
	password string
	prompt   string // Expected prompt after login (> or #)
}

// NewTelnetCommander creates a new TelnetCommander and establishes connection
func NewTelnetCommander(host string, port int, username, password string, timeout time.Duration) (*TelnetCommander, error) {
	tc := &TelnetCommander{
		host:     host,
		port:     port,
		timeout:  timeout,
		username: username,
		password: password,
		prompt:   ">", // Default ZTE-style prompt
	}

	if err := tc.connect(); err != nil {
		return nil, err
	}

	return tc, nil
}

// connect establishes TCP connection and authenticates
func (tc *TelnetCommander) connect() error {
	address := net.JoinHostPort(tc.host, fmt.Sprintf("%d", tc.port))
	conn, err := net.DialTimeout("tcp", address, tc.timeout)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	tc.conn = conn
	tc.reader = bufio.NewReader(conn)
	_ = tc.conn.SetDeadline(time.Now().Add(tc.timeout))

	// Read banner and wait for username prompt. The prompt has to actually
	// appear: treating any bytes as the prompt sent the username into a session
	// still negotiating options, and the login never recovered.
	banner, err := tc.readUntilPatterns([]string{"username:", "login:", "user:"}, loginPromptWait)
	if err != nil {
		return fmt.Errorf("failed to read banner: %w", err)
	}
	if !containsAnyFold(banner, []string{"username:", "login:", "user:"}) {
		return fmt.Errorf("no username prompt after %s", loginPromptWait)
	}

	// Send username
	if _, err := tc.writeLine(tc.username); err != nil {
		return fmt.Errorf("failed to send username: %w", err)
	}

	// Wait for password prompt
	pwPrompt, err := tc.readUntilPatterns([]string{"password:"}, loginPromptWait)
	if err != nil || !strings.Contains(strings.ToLower(pwPrompt), "password") {
		return fmt.Errorf("expected password prompt")
	}

	// Send password
	if _, err := tc.writeLine(tc.password); err != nil {
		return fmt.Errorf("failed to send password: %w", err)
	}

	// Verify successful login
	response, err := tc.readUntilPatterns([]string{">", "#", "$"}, 3*time.Second)
	if err != nil {
		return fmt.Errorf("failed to verify login")
	}

	// Check for failure indicators
	respLower := strings.ToLower(response)
	failureIndicators := []string{"error", "incorrect", "denied", "invalid"}
	for _, indicator := range failureIndicators {
		if strings.Contains(respLower, indicator) {
			return fmt.Errorf("authentication failed")
		}
	}

	return nil
}

// Close terminates the Telnet connection and implements io.Closer.
func (tc *TelnetCommander) Close() error {
	return tc.close()
}

// close terminates the Telnet connection
func (tc *TelnetCommander) close() error {
	if tc.conn != nil {
		return tc.conn.Close()
	}
	return nil
}

// ExecuteCommand sends a single command and waits for response
func (tc *TelnetCommander) ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error) {
	startTime := time.Now()

	// Send command
	if _, err := tc.writeLine(cmd); err != nil {
		return nil, fmt.Errorf("failed to send command '%s': %w", cmd, err)
	}

	// Wait for response (look for prompt or EOF)
	output, err := tc.waitForPrompt(5 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for response: %w", err)
	}

	duration := time.Since(startTime)

	return &CommandResult{
		Success:  commandOutputError(output) == "",
		Output:   output,
		Error:    commandOutputError(output),
		Duration: duration,
	}, nil
}

// BatchExecute sends multiple commands sequentially
func (tc *TelnetCommander) BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error) {
	results := make([]*CommandResult, len(cmds))

	for i, cmd := range cmds {
		result, err := tc.ExecuteCommand(ctx, cmd)
		if err != nil {
			results[i] = &CommandResult{
				Success:  false,
				Error:    err.Error(),
				Duration: 0,
			}
			// Continue trying other commands (don't abort batch)
			continue
		}
		results[i] = result
	}

	return results, nil
}

// writeLine writes a line terminated with \r\n
func (tc *TelnetCommander) writeLine(line string) (int, error) {
	// connect set one deadline for the whole connection; refresh it per write so
	// a session held open past that timeout can still send commands.
	_ = tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout))
	return tc.conn.Write([]byte(line + "\r\n"))
}

// containsAnyFold reports whether s contains any of the substrings, ignoring case.
func containsAnyFold(s string, substrings []string) bool {
	lower := strings.ToLower(s)
	for _, substring := range substrings {
		if strings.Contains(lower, strings.ToLower(substring)) {
			return true
		}
	}
	return false
}

// readUntilPatterns reads until one of the patterns is found
func (tc *TelnetCommander) readUntilPatterns(patterns []string, maxWait time.Duration) (string, error) {
	var buffer strings.Builder
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		_ = tc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		b, err := tc.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return buffer.String(), err
		}

		if b == telnetIAC {
			_ = tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout))
			data, keep, negErr := negotiateTelnetOption(tc.reader, tc.conn)
			if negErr != nil {
				if netErr, ok := negErr.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return buffer.String(), negErr
			}
			if !keep {
				continue
			}
			b = data
		}

		buffer.WriteByte(b)

		current := strings.ToLower(buffer.String())
		for _, pattern := range patterns {
			if strings.Contains(current, strings.ToLower(pattern)) {
				return buffer.String(), nil
			}
		}
	}

	return buffer.String(), nil
}

// waitForPrompt waits for the prompt character
func (tc *TelnetCommander) waitForPrompt(maxWait time.Duration) (string, error) {
	return tc.readUntilPatterns([]string{">", "#", "$"}, maxWait)
}

// HSGQCommander implements OLTCommander for HSGQ OLTs
type HSGQCommander struct {
	conn    net.Conn
	reader  *bufio.Reader
	timeout time.Duration
}

// NewHSGQCommander creates an HSGQ commander from an existing connection
func NewHSGQCommander(conn net.Conn, reader *bufio.Reader, timeout time.Duration) *HSGQCommander {
	return &HSGQCommander{
		conn:    conn,
		reader:  reader,
		timeout: timeout,
	}
}

// ExecuteCommand sends a CLI command to HSGQ OLT
func (hc *HSGQCommander) ExecuteCommand(ctx context.Context, cmd string) (*CommandResult, error) {
	startTime := time.Now()

	if _, err := hc.conn.Write([]byte(cmd + "\r\n")); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	output, err := hc.waitForPrompt(5 * time.Second)
	duration := time.Since(startTime)

	if err != nil {
		return &CommandResult{Success: false, Output: output, Error: err.Error(), Duration: duration}, nil
	}
	return &CommandResult{
		Success:  commandOutputError(output) == "",
		Output:   output,
		Error:    commandOutputError(output),
		Duration: duration,
	}, nil
}

// BatchExecute sends multiple commands
func (hc *HSGQCommander) BatchExecute(ctx context.Context, cmds []string) ([]*CommandResult, error) {
	results := make([]*CommandResult, len(cmds))

	for i, cmd := range cmds {
		result, err := hc.ExecuteCommand(ctx, cmd)
		results[i] = result
		if err != nil {
			if result == nil {
				result = &CommandResult{}
				results[i] = result
			}
			result.Success = false
			result.Error = err.Error()
		}
	}

	return results, nil
}

// waitForPrompt waits for HSGQ prompt
func (hc *HSGQCommander) waitForPrompt(maxWait time.Duration) (string, error) {
	var buffer strings.Builder
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		_ = hc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		b, err := hc.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return buffer.String(), err
		}

		buffer.WriteByte(b)

		// HSGQ prompts are typically "> " or "# "
		str := buffer.String()
		if strings.HasSuffix(strings.TrimSpace(str), "> ") ||
			strings.HasSuffix(strings.TrimSpace(str), "# ") {
			return str, nil
		}
	}

	return buffer.String(), nil
}
