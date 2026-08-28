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

// loginPromptWait bounds each step of the login handshake.
//
// An idle C300 negotiates options and prints its banner in about fifteen
// milliseconds, so this is not a delay anyone waits out: the read returns the
// moment the prompt arrives. It is a ceiling for a busy one. While the
// discovery poll has the OLT's management plane saturated its SNMP walks time
// out too, and a provisioning request that happened to land in that window was
// refused outright rather than waiting a few seconds longer.
const loginPromptWait = 30 * time.Second

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
	hostname string // Fixed part of the CLI prompt, learned at login
	// loginWait bounds each handshake step. Held per commander rather than read
	// from the constant so a test can use a budget it does not have to wait out.
	loginWait time.Duration
}

// NewTelnetCommander creates a new TelnetCommander and establishes connection
func NewTelnetCommander(host string, port int, username, password string, timeout time.Duration) (*TelnetCommander, error) {
	return newTelnetCommander(host, port, username, password, timeout, loginPromptWait)
}

func newTelnetCommander(host string, port int, username, password string, timeout, loginWait time.Duration) (*TelnetCommander, error) {
	tc := &TelnetCommander{
		loginWait: loginWait,
		host:      host,
		port:      port,
		timeout:   timeout,
		username:  username,
		password:  password,
		prompt:    ">", // Default ZTE-style prompt
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
	// Sized to the handshake rather than to one command. The read loop sets its
	// own deadline each pass so this is not what bounds a login today, but a
	// connect-time deadline shorter than the handshake it covers is a trap
	// waiting for the first step that does not poll.
	_ = tc.conn.SetDeadline(time.Now().Add(3 * tc.loginWait))

	// Read banner and wait for username prompt. The prompt has to actually
	// appear: treating any bytes as the prompt sent the username into a session
	// still negotiating options, and the login never recovered.
	banner, err := tc.readUntilPatterns([]string{"username:", "login:", "user:"}, tc.loginWait)
	if err != nil {
		return fmt.Errorf("failed to read banner: %w", err)
	}
	if !containsAnyFold(banner, []string{"username:", "login:", "user:"}) {
		return fmt.Errorf("no username prompt after %s", tc.loginWait)
	}

	// Send username
	if _, err := tc.writeLine(tc.username); err != nil {
		return fmt.Errorf("failed to send username: %w", err)
	}

	// Wait for password prompt
	pwPrompt, err := tc.readUntilPatterns([]string{"password:"}, tc.loginWait)
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

	// Learned once so later reads can tell the prompt from the same characters
	// appearing inside command output.
	tc.hostname = deviceHostname(response)

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

	tc.discardPending()

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
	// Stops at the first failure. Carrying on used to send the closing "exit"
	// and "commit" after a command the OLT had refused, which is how a rejected
	// registration still left a half-written ONU behind.
	executed := make([]*CommandResult, 0, len(cmds))
	for _, cmd := range cmds {
		result, err := tc.ExecuteCommand(ctx, cmd)
		if err != nil {
			result = &CommandResult{Success: false, Error: err.Error()}
		}
		executed = append(executed, result)
		if !result.Success {
			break
		}
	}

	return executed, nil
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

// readUntilPatterns reads until one of the patterns appears anywhere in the
// stream. Used for the login handshake, where there is no prompt yet.
func (tc *TelnetCommander) readUntilPatterns(patterns []string, maxWait time.Duration) (string, error) {
	return tc.readUntil(func(buffer string) bool {
		return containsAnyFold(buffer, patterns)
	}, maxWait)
}

// readUntil reads bytes, answering Telnet negotiation as it goes, until done
// reports that enough has arrived or maxWait expires.
func (tc *TelnetCommander) readUntil(done func(string) bool, maxWait time.Duration) (string, error) {
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

		if done(buffer.String()) {
			return buffer.String(), nil
		}
	}

	return buffer.String(), nil
}

// ExecuteBulk reads output too large for the prompt-bounded read: the running
// config runs to thousands of lines and takes tens of seconds, and its body
// contains characters the prompt match would stop on. It instead returns once
// the OLT has been quiet for quiet, or when max expires.
func (tc *TelnetCommander) ExecuteBulk(ctx context.Context, cmd string, quiet, max time.Duration) (string, error) {
	if _, err := tc.writeLine(cmd); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	var buffer strings.Builder
	deadline := time.Now().Add(max)
	lastByte := time.Now()

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return buffer.String(), ctx.Err()
		}

		_ = tc.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		b, err := tc.reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if buffer.Len() > 0 && time.Since(lastByte) > quiet {
					return buffer.String(), nil
				}
				continue
			}
			return buffer.String(), err
		}

		lastByte = time.Now()

		if b == telnetIAC {
			_ = tc.conn.SetWriteDeadline(time.Now().Add(tc.timeout))
			data, keep, negErr := negotiateTelnetOption(tc.reader, tc.conn)
			if negErr != nil {
				continue
			}
			if !keep {
				continue
			}
			b = data
		}

		buffer.WriteByte(b)
	}

	return buffer.String(), nil
}

// waitForPrompt reads until the CLI has finished and printed its prompt again.
func (tc *TelnetCommander) waitForPrompt(maxWait time.Duration) (string, error) {
	return tc.readUntil(func(buffer string) bool {
		return endsWithDevicePrompt(buffer, tc.hostname)
	}, maxWait)
}

// discardPending throws away anything the OLT is still writing, so a command
// is never sent into the tail of the previous one's output.
func (tc *TelnetCommander) discardPending() {
	for {
		_ = tc.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		if _, err := tc.reader.ReadByte(); err != nil {
			return
		}
	}
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
