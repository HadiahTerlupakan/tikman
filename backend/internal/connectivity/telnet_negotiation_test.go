package connectivity

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// The opening sequence a ZTE C300 sends before it will print anything:
// WILL ECHO, DO TERMINAL-TYPE, DO NAWS, DO TERMINAL-SPEED, WILL SUPPRESS-GO-AHEAD.
var c300Greeting = []byte{255, 251, 1, 255, 253, 24, 255, 253, 31, 255, 253, 32, 255, 251, 3}

type fakeOLT struct {
	mu          sync.Mutex
	negotiation []byte
	username    string
	password    string
}

func (f *fakeOLT) serve(t *testing.T, listener net.Listener) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, err := conn.Write(c300Greeting); err != nil {
		return
	}

	answers := make([]byte, len(c300Greeting))
	if _, err := io.ReadFull(conn, answers); err != nil {
		return
	}

	reader := bufio.NewReader(conn)
	if _, err := conn.Write([]byte("\r\nUsername:")); err != nil {
		return
	}
	username, _ := reader.ReadString('\n')
	if _, err := conn.Write([]byte("\r\nPassword:")); err != nil {
		return
	}
	password, _ := reader.ReadString('\n')
	_, _ = conn.Write([]byte("\r\nBRAS-PANCORANMAS-DPK#"))

	f.mu.Lock()
	f.negotiation = answers
	f.username = strings.TrimSpace(username)
	f.password = strings.TrimSpace(password)
	f.mu.Unlock()
}

// Before this was handled the client read the negotiation bytes as though they
// were the username prompt and sent credentials into a session that had not
// started, so every CLI login to a C300 failed.
func TestTelnetLoginAnswersOptionNegotiation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	olt := &fakeOLT{}
	go olt.serve(t, listener)

	port := listener.Addr().(*net.TCPAddr).Port
	commander, err := NewTelnetCommander("127.0.0.1", port, "admin", "secret", 5*time.Second)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer func() { _ = commander.Close() }()

	olt.mu.Lock()
	defer olt.mu.Unlock()

	// Echo and suppress-go-ahead are agreed to; the terminal options are refused
	// because this is a line-mode client with no terminal to describe.
	want := []byte{255, 253, 1, 255, 252, 24, 255, 252, 31, 255, 252, 32, 255, 253, 3}
	if !bytes.Equal(olt.negotiation, want) {
		t.Errorf("negotiation answers = %v, want %v", olt.negotiation, want)
	}
	if olt.username != "admin" {
		t.Errorf("username = %q, want admin", olt.username)
	}
	if olt.password != "secret" {
		t.Errorf("password reached the OLT as %q", olt.password)
	}
}

// An OLT that negotiates and then never prompts must fail the login rather than
// push credentials into the stream and wait.
func TestTelnetLoginFailsWithoutAUsernamePrompt(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write(c300Greeting)
		time.Sleep(9 * time.Second)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	commander, err := NewTelnetCommander("127.0.0.1", port, "admin", "secret", 5*time.Second)
	if err == nil {
		_ = commander.Close()
		t.Fatal("login reported success without ever seeing a username prompt")
	}
	if !strings.Contains(err.Error(), "no username prompt") {
		t.Errorf("error = %v, want it to name the missing prompt", err)
	}
}
