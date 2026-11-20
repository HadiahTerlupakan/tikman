// Package shell holds an interactive CLI session on a switch, over SSH with a
// PTY or over Telnet, for vendors whose CLI has to be driven like a terminal.
package shell

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Protocol is how the CLI is reached.
type Protocol string

// The protocols Open accepts.
const (
	ProtocolSSH    Protocol = "ssh"
	ProtocolTelnet Protocol = "telnet"
)

// pendingQuiet is how long the device must stay silent before a command is
// sent, so the command never lands in the tail of earlier output.
const pendingQuiet = 50 * time.Millisecond

// Config says where a switch is and how to log in to it.
type Config struct {
	Protocol Protocol
	Host     string
	Port     int
	Username string
	Password string
	// PinnedFingerprint is the SSH host key an admin has trusted, in the
	// "SHA256:…" form ssh-keygen -l prints. Unused on Telnet.
	PinnedFingerprint string
	Dialect           Dialect
}

// Shell runs one command at a time on a switch CLI.
type Shell interface {
	Run(ctx context.Context, cmd string) (string, error)
	Close() error
}

var _ Shell = (*Session)(nil)

// Session is a logged-in CLI with paging switched off. It runs one command
// at a time and is not safe for concurrent use.
type Session struct {
	out    chan []byte
	done   chan struct{}
	err    error // why out was closed; written before close(out)
	write  func([]byte) error
	closer io.Closer
	eol    string
	prompt promptMatcher
	once   sync.Once
}

func newSession(write func([]byte) error, closer io.Closer, eol string) *Session {
	return &Session{
		out:    make(chan []byte, 64),
		done:   make(chan struct{}),
		write:  write,
		closer: closer,
		eol:    eol,
	}
}

// Open connects and logs in, learns the prompt and switches paging off. Over
// SSH it refuses an unpinned or changed host key before any credential is
// sent. ctx bounds the whole of it.
func Open(ctx context.Context, cfg Config) (*Session, error) {
	if _, ok := pagingOff[cfg.Dialect]; !ok {
		return nil, fmt.Errorf("shell: unknown dialect %q", cfg.Dialect)
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("shell: connect %s: %w", address, err)
	}
	// A handshake has no context of its own; closing the socket is what
	// stops one that ctx has given up on.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	session, err := start(ctx, conn, cfg)
	if err != nil {
		if session != nil {
			_ = session.Close()
		}
		_ = conn.Close()
		if ctx.Err() != nil {
			return nil, fmt.Errorf("shell: login to %s: %w", address, ctx.Err())
		}
		return nil, err
	}
	if _, err := session.Run(ctx, pagingOff[cfg.Dialect]); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

func start(ctx context.Context, conn net.Conn, cfg Config) (*Session, error) {
	switch cfg.Protocol {
	case ProtocolSSH:
		session, err := startSSH(conn, cfg)
		if err != nil {
			return nil, err
		}
		return session, session.awaitPrompt(ctx, cfg.Dialect)
	case ProtocolTelnet:
		session := startTelnet(conn)
		return session, session.telnetLogin(ctx, cfg)
	default:
		return nil, fmt.Errorf("shell: unknown protocol %q", cfg.Protocol)
	}
}

// Run sends cmd and returns what it printed, without the echoed command or
// the prompt, with pager breaks answered and removed.
func (s *Session) Run(ctx context.Context, cmd string) (string, error) {
	s.discardPending()
	if err := s.send(cmd); err != nil {
		return "", err
	}
	raw, err := s.readUntil(ctx, func(line string) bool { return s.prompt.matches(line) })
	if err != nil {
		return "", fmt.Errorf("shell: %q: %w", cmd, err)
	}
	return commandOutput(raw, cmd), nil
}

// Close ends the session.
func (s *Session) Close() error {
	s.once.Do(func() { close(s.done) })
	return s.closer.Close()
}

func (s *Session) send(line string) error {
	if err := s.write([]byte(line + s.eol)); err != nil {
		return fmt.Errorf("shell: send: %w", err)
	}
	return nil
}

// readUntil collects output until done accepts its last line, answering
// pagers and yes/no questions on the way.
func (s *Session) readUntil(ctx context.Context, done func(lastLine string) bool) (string, error) {
	var buffer strings.Builder
	answered := 0
	for {
		select {
		case <-ctx.Done():
			return buffer.String(), ctx.Err()
		case chunk, ok := <-s.out:
			if !ok {
				return buffer.String(), fmt.Errorf("connection closed: %w", s.err)
			}
			buffer.Write(chunk)
		}
		text := buffer.String()
		if reply := pendingReply(text[answered:]); reply != "" {
			answered = len(text)
			if err := s.answer(reply); err != nil {
				return text, err
			}
		}
		if done(renderLine(lastLine(text))) {
			return text, nil
		}
	}
}

func (s *Session) answer(reply string) error {
	if reply == " " {
		return s.write([]byte(reply))
	}
	return s.send(reply)
}

// discardPending throws away output still arriving from before.
func (s *Session) discardPending() {
	for {
		select {
		case _, ok := <-s.out:
			if !ok {
				return
			}
		case <-time.After(pendingQuiet):
			return
		}
	}
}

// emit hands a chunk read from the device to readUntil; false once closed.
func (s *Session) emit(chunk []byte) bool {
	select {
	case s.out <- chunk:
		return true
	case <-s.done:
		return false
	}
}

// finish records why the device stopped talking and wakes any reader.
func (s *Session) finish(err error) {
	s.err = err
	close(s.out)
}

// awaitPrompt waits for the first prompt and learns the switch name from it.
func (s *Session) awaitPrompt(ctx context.Context, dialect Dialect) error {
	text, err := s.readUntil(ctx, func(line string) bool { return learnHostname(dialect, line) != "" })
	if err != nil {
		return fmt.Errorf("shell: no prompt after login: %w", err)
	}
	s.prompt = newPromptMatcher(dialect, learnHostname(dialect, renderLine(lastLine(text))))
	return nil
}
