package connectivity

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// mockTelnetServer simulates a simple Telnet server for testing
type mockTelnetServer struct {
	listener net.Listener
	addr     string
}

func startMockTelnetServer(t *testing.T) *mockTelnetServer {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}

	server := &mockTelnetServer{listener: l, addr: l.Addr().String()}

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		reader := bufio.NewReader(conn)

		// Send username prompt
		_, _ = conn.Write([]byte("Username: "))

		// Read username
		username, _ := reader.ReadString('\n')
		if !strings.Contains(strings.ToLower(strings.TrimSpace(username)), "admin") {
			_, _ = conn.Write([]byte("Login incorrect\n"))
			return
		}

		// Send password prompt
		_, _ = conn.Write([]byte("Password: "))

		// Read password
		password, _ := reader.ReadString('\n')
		if !strings.Contains(strings.ToLower(strings.TrimSpace(password)), "admin123") {
			_, _ = conn.Write([]byte("Login incorrect\n"))
			return
		}

		// Send prompt
		_, _ = conn.Write([]byte("\nRouter# "))

		// Echo commands
		for {
			cmd, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			response := fmt.Sprintf("%s executed\nRouter# ", cmd)
			_, _ = conn.Write([]byte(response))
		}
	}()

	return server
}

func TestNewTelnetCommander_Success(t *testing.T) {
	server := startMockTelnetServer(t)
	host, portStr, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	cmdr, err := NewTelnetCommander(host, port, "admin", "admin123", 5*time.Second)
	if err != nil {
		t.Fatalf("expected commander creation to succeed, got error: %v", err)
	}
	defer func() { _ = cmdr.close() }()

	result, err := cmdr.ExecuteCommand(context.Background(), "show version")
	if err != nil {
		t.Fatalf("expected command to succeed, got error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected command to be successful")
	}
	if !strings.Contains(result.Output, "show version executed") {
		t.Errorf("expected output to contain command execution result, got: %s", result.Output)
	}
}

func TestNewTelnetCommander_AuthFailure(t *testing.T) {
	server := startMockTelnetServer(t)
	host, portStr, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	_, err = NewTelnetCommander(host, port, "wrong", "wrong", 5*time.Second)
	if err == nil {
		t.Fatalf("expected authentication failure, got no error")
	}
}

func TestTelnetCommander_BatchExecute(t *testing.T) {
	server := startMockTelnetServer(t)
	host, portStr, err := net.SplitHostPort(server.addr)
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	cmdr, err := NewTelnetCommander(host, port, "admin", "admin123", 5*time.Second)
	if err != nil {
		t.Fatalf("expected commander creation to succeed, got error: %v", err)
	}
	defer func() { _ = cmdr.close() }()

	cmds := []string{"cmd1", "cmd2", "cmd3"}
	results, err := cmdr.BatchExecute(context.Background(), cmds)
	if err != nil {
		t.Fatalf("expected batch to succeed, got error: %v", err)
	}
	if len(results) != len(cmds) {
		t.Fatalf("expected %d results, got %d", len(cmds), len(results))
	}
	for i, result := range results {
		if !result.Success {
			t.Errorf("expected command %d to be successful", i)
		}
		expected := fmt.Sprintf("%s executed", cmds[i])
		if !strings.Contains(result.Output, expected) {
			t.Errorf("expected result %d to contain '%s', got: %s", i, expected, result.Output)
		}
	}
}
