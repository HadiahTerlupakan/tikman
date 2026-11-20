package shell

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	testUser     = "reader"
	testPassword = "s3cret"
)

type fakeSSHServer struct {
	addr        string
	fingerprint string
	cli         *fakeCLI

	mu               sync.Mutex
	passwordAttempts int
	ptyWidth         int
}

func (s *fakeSSHServer) attempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.passwordAttempts
}

func startFakeSSH(t *testing.T, cli *fakeCLI) *fakeSSHServer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	server := &fakeSSHServer{
		addr:        listener.Addr().String(),
		fingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
		cli:         cli,
	}
	config := &ssh.ServerConfig{PasswordCallback: server.checkPassword}
	config.AddHostKey(signer)
	go server.accept(listener, config)
	return server
}

func (s *fakeSSHServer) checkPassword(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	s.mu.Lock()
	s.passwordAttempts++
	s.mu.Unlock()
	if conn.User() == testUser && string(password) == testPassword {
		return nil, nil
	}
	return nil, errors.New("wrong password")
}

func (s *fakeSSHServer) accept(listener net.Listener, config *ssh.ServerConfig) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn, config)
	}
}

func (s *fakeSSHServer) handle(conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()
	_, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			return
		}
		go s.session(channel, channelRequests)
	}
}

func (s *fakeSSHServer) session(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()
	for request := range requests {
		switch request.Type {
		case "pty-req":
			s.recordPty(request.Payload)
			_ = request.Reply(true, nil)
		case "shell":
			_ = request.Reply(true, nil)
			s.cli.serve(channel, &fakeReader{r: bufio.NewReader(channel), cli: s.cli})
			return
		default:
			_ = request.Reply(false, nil)
		}
	}
}

// recordPty reads the column count from a pty-req: a string term, then cols.
func (s *fakeSSHServer) recordPty(payload []byte) {
	if len(payload) < 4 {
		return
	}
	termLength := int(binary.BigEndian.Uint32(payload))
	if len(payload) < 8+termLength {
		return
	}
	s.mu.Lock()
	s.ptyWidth = int(binary.BigEndian.Uint32(payload[4+termLength:]))
	s.mu.Unlock()
}

func sshConfig(t *testing.T, server *fakeSSHServer, pinned string) Config {
	host, port := splitHostPort(t, server.addr)
	return Config{
		Protocol: ProtocolSSH, Host: host, Port: port,
		Username: testUser, Password: testPassword,
		PinnedFingerprint: pinned, Dialect: DialectVRP,
	}
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func openTest(t *testing.T, cfg Config) *Session {
	t.Helper()
	session, err := Open(testContext(t), cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestOpenSSHUnpinnedRefusesBeforePassword(t *testing.T) {
	server := startFakeSSH(t, versionCLI())

	_, err := Open(testContext(t), sshConfig(t, server, ""))

	var unpinned ErrHostKeyUnpinned
	if !errors.As(err, &unpinned) {
		t.Fatalf("err = %v, want ErrHostKeyUnpinned", err)
	}
	if unpinned.Presented != server.fingerprint {
		t.Errorf("Presented = %q, want %q", unpinned.Presented, server.fingerprint)
	}
	if !strings.HasPrefix(unpinned.Presented, "SHA256:") {
		t.Errorf("Presented = %q, want the ssh-keygen -l form", unpinned.Presented)
	}
	if n := server.attempts(); n != 0 {
		t.Errorf("server saw %d password attempts, want 0", n)
	}
}

func TestOpenSSHChangedKeyRefusesBeforePassword(t *testing.T) {
	server := startFakeSSH(t, versionCLI())

	_, err := Open(testContext(t), sshConfig(t, server, "SHA256:bm90LXRoaXMtc3dpdGNoLXNob3N0LWtleQ"))

	var changed ErrHostKeyChanged
	if !errors.As(err, &changed) {
		t.Fatalf("err = %v, want ErrHostKeyChanged", err)
	}
	if changed.Presented != server.fingerprint {
		t.Errorf("Presented = %q, want %q", changed.Presented, server.fingerprint)
	}
	if n := server.attempts(); n != 0 {
		t.Errorf("server saw %d password attempts, want 0", n)
	}
}

func TestOpenSSHPinnedDisablesPagingAndRuns(t *testing.T) {
	cli := versionCLI()
	server := startFakeSSH(t, cli)
	session := openTest(t, sshConfig(t, server, server.fingerprint))

	out, err := session.Run(testContext(t), "display version")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != wantVersion {
		t.Errorf("output = %q, want %q", out, wantVersion)
	}
	if got := cli.lines(); len(got) != 2 || got[0] != "screen-length 0 temporary" {
		t.Errorf("sent lines = %q, want paging disabled before the command", got)
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.ptyWidth != terminalWidth {
		t.Errorf("pty width = %d, want %d", server.ptyWidth, terminalWidth)
	}
}

func TestRunAnswersPagerAndStripsIt(t *testing.T) {
	cli := versionCLI()
	server := startFakeSSH(t, cli)
	session := openTest(t, sshConfig(t, server, server.fingerprint))

	out, err := session.Run(testContext(t), "display current-configuration")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != wantConfig {
		t.Errorf("output = %q, want %q", out, wantConfig)
	}
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if string(cli.pagerAnswers) != "  " {
		t.Errorf("pager answers = %q, want two spaces", cli.pagerAnswers)
	}
}

func TestRunWithoutPromptReturnsDeadline(t *testing.T) {
	server := startFakeSSH(t, versionCLI())
	session := openTest(t, sshConfig(t, server, server.fingerprint))

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := session.Run(ctx, "display hang")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Errorf("Run took %s after a 300ms deadline", elapsed)
	}
}

func TestOpenAnswersYesNoQuestionWithNo(t *testing.T) {
	cli := versionCLI()
	cli.askYesNo = true
	server := startFakeSSH(t, cli)
	openTest(t, sshConfig(t, server, server.fingerprint))

	if got := cli.lines(); len(got) == 0 || got[0] != "N" {
		t.Errorf("sent lines = %q, want the question answered N", got)
	}
}
