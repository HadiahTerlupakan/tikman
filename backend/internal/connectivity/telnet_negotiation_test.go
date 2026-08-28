package connectivity

import (
	"bufio"
	"bytes"
	"context"
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
		// Held open past the budget below, so the login gives up on the missing
		// prompt rather than on the server hanging up.
		time.Sleep(2 * time.Second)
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	// A short budget: the real one is thirty seconds, which this would otherwise
	// spend waiting for a prompt the fake server never sends.
	commander, err := newTelnetCommander("127.0.0.1", port, "admin", "secret", 5*time.Second, 500*time.Millisecond)
	if err == nil {
		_ = commander.Close()
		t.Fatal("login reported success without ever seeing a username prompt")
	}
	if !strings.Contains(err.Error(), "no username prompt") {
		t.Errorf("error = %v, want it to name the missing prompt", err)
	}
}

// serveCommands logs in a client and then answers each command it receives from
// the script, so a test can drive the commander end to end.
func serveCommands(t *testing.T, listener net.Listener, answers map[string]string, received *[]string) {
	t.Helper()

	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	if _, err := conn.Write(c300Greeting); err != nil {
		return
	}
	if _, err := io.ReadFull(conn, make([]byte, len(c300Greeting))); err != nil {
		return
	}

	reader := bufio.NewReader(conn)
	_, _ = conn.Write([]byte("\r\nUsername:"))
	_, _ = reader.ReadString('\n')
	_, _ = conn.Write([]byte("\r\nPassword:"))
	_, _ = reader.ReadString('\n')
	_, _ = conn.Write([]byte("\r\nBRAS-PANCORANMAS-DPK#"))

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)
		*received = append(*received, command)
		answer := answers[command]
		// Echo as a real CLI does, then the answer, then the prompt back.
		_, _ = conn.Write([]byte(command + "\r\n" + answer + "\r\nBRAS-PANCORANMAS-DPK(config-if)#"))
	}
}

// A long command echoes back wrapped, and the C300 marks the wrap with '$'.
// Reading that as the prompt returned mid-echo and the next command went out
// on top of the previous one's output.
func TestExecuteCommandReadsPastAWrappedEcho(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	long := "onu 15 type HG8245H5 sn HWTCB403E8A0"
	var received []string
	go serveCommands(t, listener, map[string]string{
		long: "wrapped output ending in a marker $\r\nsecond line of output",
	}, &received)

	commander, err := NewTelnetCommander("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "admin", "secret", 5*time.Second)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer func() { _ = commander.Close() }()

	result, err := commander.ExecuteCommand(context.Background(), long)
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}

	if !strings.Contains(result.Output, "second line of output") {
		t.Errorf("read stopped early, got %q", result.Output)
	}
}

// Carrying on after a refusal sent the closing exit and commit anyway, which
// is how a rejected registration still left a half-written ONU on the OLT.
func TestBatchExecuteStopsAtTheFirstRefusal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	var received []string
	go serveCommands(t, listener, map[string]string{
		"service-port 1 vport 1 user-vlan 214 vlan 214": ".[Failed]",
	}, &received)

	commander, err := NewTelnetCommander("127.0.0.1", listener.Addr().(*net.TCPAddr).Port, "admin", "secret", 5*time.Second)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer func() { _ = commander.Close() }()

	results, err := commander.BatchExecute(context.Background(), []string{
		"configure terminal",
		"service-port 1 vport 1 user-vlan 214 vlan 214",
		"exit",
		"commit",
	})
	if err != nil {
		t.Fatalf("BatchExecute: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("ran %d commands, want to stop after the refusal: %v", len(results), received)
	}
	if results[1].Success {
		t.Error("the refused command was reported as a success")
	}
	for _, command := range received {
		if command == "commit" {
			t.Fatal("commit was sent after a refusal")
		}
	}
}
