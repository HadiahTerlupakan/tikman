package shell

import (
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// sshEOL is the Enter key as a terminal sends it in raw mode.
const sshEOL = "\r"

const sshReadSize = 4096

// ErrHostKeyUnpinned means no host key has been trusted for this switch yet.
// Presented is the key it offered, for an admin to confirm.
type ErrHostKeyUnpinned struct{ Presented string }

func (e ErrHostKeyUnpinned) Error() string {
	return "shell: host key " + e.Presented + " is not trusted yet"
}

// ErrHostKeyChanged means the switch offered a key other than the pinned one.
type ErrHostKeyChanged struct{ Presented string }

func (e ErrHostKeyChanged) Error() string {
	return "shell: host key changed to " + e.Presented
}

// pinnedHostKey accepts only the pinned key. The key exchange runs before
// authentication, so refusing here means the password is never sent.
func pinnedHostKey(pinned string, refused *error) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		presented := ssh.FingerprintSHA256(key)
		switch {
		case pinned == "":
			*refused = ErrHostKeyUnpinned{Presented: presented}
		case pinned != presented:
			*refused = ErrHostKeyChanged{Presented: presented}
		default:
			return nil
		}
		return *refused
	}
}

func startSSH(conn net.Conn, cfg Config) (*Session, error) {
	var refused error
	config := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(cfg.Password)},
		HostKeyCallback: pinnedHostKey(cfg.PinnedFingerprint, &refused),
	}
	clientConn, channels, requests, err := ssh.NewClientConn(conn, conn.RemoteAddr().String(), config)
	if refused != nil {
		return nil, refused
	}
	if err != nil {
		return nil, fmt.Errorf("shell: ssh login: %w", err)
	}
	client := ssh.NewClient(clientConn, channels, requests)
	session, err := openPTYShell(client)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return session, nil
}

func openPTYShell(client *ssh.Client) (*Session, error) {
	channel, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("shell: ssh session: %w", err)
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1}
	if err := channel.RequestPty("vt100", terminalHeight, terminalWidth, modes); err != nil {
		return nil, fmt.Errorf("shell: ssh pty: %w", err)
	}
	stdin, err := channel.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("shell: ssh stdin: %w", err)
	}
	stdout, err := channel.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("shell: ssh stdout: %w", err)
	}
	if err := channel.Shell(); err != nil {
		return nil, fmt.Errorf("shell: ssh shell: %w", err)
	}
	var mu sync.Mutex
	write := func(b []byte) error {
		mu.Lock()
		defer mu.Unlock()
		_, err := stdin.Write(b)
		return err
	}
	session := newSession(write, client, sshEOL)
	go pumpStream(session, stdout)
	return session, nil
}

// pumpStream feeds everything r yields to the session until it fails.
func pumpStream(s *Session, r io.Reader) {
	for {
		buf := make([]byte, sshReadSize)
		n, err := r.Read(buf)
		if n > 0 && !s.emit(buf[:n]) {
			return
		}
		if err != nil {
			s.finish(err)
			return
		}
	}
}
