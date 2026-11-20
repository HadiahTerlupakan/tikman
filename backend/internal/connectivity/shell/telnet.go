package shell

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/tikman/olt-provisioning/internal/connectivity"
)

// telnetEOL is the Telnet newline, RFC 854.
const telnetEOL = "\r\n"

// Telnet bytes this file answers itself, RFC 854 and RFC 1073 (NAWS).
const (
	telnetSE   = 240
	telnetSB   = 250
	telnetWILL = 251
	telnetDO   = 253
	telnetNAWS = 31
)

// The VRP VTY login prompts, exactly "Username:" and "Password:".
const (
	usernamePrompt = "username:"
	passwordPrompt = "password:"
)

func startTelnet(conn net.Conn) *Session {
	var mu sync.Mutex
	write := func(b []byte) error {
		mu.Lock()
		defer mu.Unlock()
		_, err := conn.Write(b)
		return err
	}
	session := newSession(write, conn, telnetEOL)
	go pumpTelnet(session, bufio.NewReader(conn), write)
	return session
}

// pumpTelnet strips and answers negotiation, feeding the data to the session.
func pumpTelnet(s *Session, r *bufio.Reader, write func([]byte) error) {
	var chunk []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			s.finish(err)
			return
		}
		keep := true
		if b == connectivity.TelnetIAC {
			b, keep, err = negotiate(r, write)
			if err != nil {
				s.finish(err)
				return
			}
		}
		if keep {
			chunk = append(chunk, b)
		}
		if len(chunk) > 0 && r.Buffered() == 0 {
			if !s.emit(chunk) {
				return
			}
			chunk = nil
		}
	}
}

// negotiate agrees to report a window size, so the switch wraps at the same
// width the SSH PTY asks for; every other option is answered by the OLT
// client's negotiation.
func negotiate(r *bufio.Reader, write func([]byte) error) (byte, bool, error) {
	if next, _ := r.Peek(2); len(next) == 2 && next[0] == telnetDO && next[1] == telnetNAWS {
		_, _ = r.Discard(2)
		return 0, false, write([]byte{
			connectivity.TelnetIAC, telnetWILL, telnetNAWS,
			connectivity.TelnetIAC, telnetSB, telnetNAWS,
			terminalWidth >> 8, terminalWidth & 0xff, 0, terminalHeight,
			connectivity.TelnetIAC, telnetSE,
		})
	}
	return connectivity.NegotiateTelnetOption(r, writerFunc(write))
}

type writerFunc func([]byte) error

func (f writerFunc) Write(b []byte) (int, error) {
	if err := f(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// telnetLogin answers the VTY prompts and waits for the CLI prompt.
func (s *Session) telnetLogin(ctx context.Context, cfg Config) error {
	steps := []struct{ prompt, answer string }{
		{usernamePrompt, cfg.Username},
		{passwordPrompt, cfg.Password},
	}
	for _, step := range steps {
		if _, err := s.readUntil(ctx, endsWith(step.prompt)); err != nil {
			return fmt.Errorf("shell: no %q prompt: %w", step.prompt, err)
		}
		if err := s.send(step.answer); err != nil {
			return err
		}
	}
	refused := func(line string) bool {
		return endsWith(usernamePrompt)(line) || endsWith(passwordPrompt)(line)
	}
	text, err := s.readUntil(ctx, func(line string) bool {
		return refused(line) || learnHostname(cfg.Dialect, line) != ""
	})
	if err != nil {
		return fmt.Errorf("shell: no prompt after login: %w", err)
	}
	last := renderLine(lastLine(text))
	if refused(last) {
		return fmt.Errorf("shell: login refused for %q", cfg.Username)
	}
	s.prompt = newPromptMatcher(cfg.Dialect, learnHostname(cfg.Dialect, last))
	return nil
}

func endsWith(prompt string) func(string) bool {
	return func(line string) bool {
		return strings.HasSuffix(strings.ToLower(strings.TrimSpace(line)), prompt)
	}
}
