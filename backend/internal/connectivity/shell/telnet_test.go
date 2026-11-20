package shell

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A VRP VTY opens with WILL ECHO, WILL SUPPRESS-GO-AHEAD and DO NAWS.
var vrpGreeting = []byte{255, 251, 1, 255, 251, 3, 255, 253, 31}

func startFakeTelnet(t *testing.T, cli *fakeCLI) Config {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serveTelnetLogin(conn, cli)
	}()
	host, port := splitHostPort(t, listener.Addr().String())
	return Config{
		Protocol: ProtocolTelnet, Host: host, Port: port,
		Username: testUser, Password: testPassword, Dialect: DialectVRP,
	}
}

func serveTelnetLogin(conn net.Conn, cli *fakeCLI) {
	reader := &fakeReader{r: bufio.NewReader(conn), cli: cli}
	_, _ = conn.Write(vrpGreeting)
	_, _ = io.WriteString(conn, "\r\n\r\nLogin authentication\r\n\r\n\r\nUsername:")
	for {
		user, err := reader.line()
		if err != nil {
			return
		}
		_, _ = io.WriteString(conn, user+"\r\nPassword:")
		password, err := reader.line()
		if err != nil {
			return
		}
		if user == testUser && password == testPassword {
			cli.serve(conn, reader)
			return
		}
		_, _ = io.WriteString(conn, "\r\nError: Failed to verify the username or password.\r\n\r\nUsername:")
	}
}

func TestOpenTelnetRunsLikeSSH(t *testing.T) {
	cli := versionCLI()
	session := openTest(t, startFakeTelnet(t, cli))

	out, err := session.Run(testContext(t), "display version")
	if err != nil {
		t.Fatalf("Run version: %v", err)
	}
	if out != wantVersion {
		t.Errorf("version = %q, want %q", out, wantVersion)
	}
	out, err = session.Run(testContext(t), "display current-configuration")
	if err != nil {
		t.Fatalf("Run config: %v", err)
	}
	if out != wantConfig {
		t.Errorf("config = %q, want %q", out, wantConfig)
	}
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.nawsWidth != terminalWidth {
		t.Errorf("NAWS width = %d, want %d", cli.nawsWidth, terminalWidth)
	}
}

func TestOpenTelnetWrongPasswordFails(t *testing.T) {
	cfg := startFakeTelnet(t, versionCLI())
	cfg.Password = "wrong"

	_, err := Open(testContext(t), cfg)

	if err == nil || !strings.Contains(err.Error(), "login refused") {
		t.Fatalf("err = %v, want login refused", err)
	}
}

// RFC 854 sends a bare carriage return as CR NUL. PostgreSQL text refuses a
// NUL byte, so one left in the output fails every insert of it.
func TestTelnetDropsTheNULAfterABareCarriageReturn(t *testing.T) {
	cli := versionCLI()
	cli.responses["display vlan"] = []string{"VID  Type\r\x00\r\n10   common"}
	session := openTest(t, startFakeTelnet(t, cli))

	out, err := session.Run(testContext(t), "display vlan")

	require.NoError(t, err)
	assert.NotContains(t, out, "\x00")
	assert.Contains(t, out, "10   common")
}
