package mikrotik

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPIRouter answers the RouterOS API on a local port: one login, then the
// sentences in replies for each command word it receives.
type fakeAPIRouter struct {
	username, password string
	replies            map[string][][]string
	silent             bool
	received           chan []string
}

func (f *fakeAPIRouter) start(t *testing.T, tlsConfig *tls.Config) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}
	t.Cleanup(func() { _ = listener.Close() })
	f.received = make(chan []string, 16)
	go f.serve(listener)
	return listener.Addr().(*net.TCPAddr).Port
}

func (f *fakeAPIRouter) serve(listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for {
		words, err := readSentence(r)
		if err != nil {
			return
		}
		f.received <- words
		if f.silent {
			continue
		}
		for _, sentence := range f.answer(words) {
			if writeSentence(conn, sentence...) != nil {
				return
			}
		}
	}
}

func (f *fakeAPIRouter) answer(words []string) [][]string {
	if words[0] != "/login" {
		return f.replies[words[0]]
	}
	if slices.Contains(words, "=name="+f.username) && slices.Contains(words, "=password="+f.password) {
		return [][]string{{"!done"}}
	}
	return [][]string{{"!trap", "=message=invalid user name or password (6)"}, {"!done"}}
}

// testTLSCertificate borrows httptest's self-signed certificate, the same kind
// a router serves, and returns it with its fingerprint.
func testTLSCertificate(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	server := httptest.NewTLSServer(nil)
	t.Cleanup(server.Close)
	cert := server.TLS.Certificates[0]
	return cert, Fingerprint(cert.Certificate[0])
}

func apiConfig(port int) Config {
	return Config{Protocol: ProtocolAPI, Host: "127.0.0.1", Port: port, Username: "tikman", Password: "secret"}
}

func shortenAPITimeout(t *testing.T) {
	t.Helper()
	saved := apiCallTimeout
	apiCallTimeout = 300 * time.Millisecond
	t.Cleanup(func() { apiCallTimeout = saved })
}

func TestAPIReadsRowsAfterLogin(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/session/print": {
			{"!re", "=.id=*1", "=name=peer-transit-1", "=remote.address=198.51.100.1", "=established=true"},
			{"!re", "=.id=*2", "=name=peer-ix-1", "=remote.address=203.0.113.166"},
			{"!done"},
		},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	defer func() { _ = transport.Close() }()

	var sessions []BGPSession
	require.NoError(t, transport.List("/routing/bgp/session", &sessions))

	require.Len(t, sessions, 2)
	assert.Equal(t, "peer-transit-1", sessions[0].Name)
	assert.Equal(t, "198.51.100.1", sessions[0].RemoteAddress)
	assert.Equal(t, "true", sessions[0].Established)
	assert.Empty(t, sessions[1].Established)
	assert.Equal(t, []string{"/login", "=name=tikman", "=password=secret"}, <-router.received)
	assert.Equal(t, []string{"/routing/bgp/session/print"}, <-router.received)
}

func TestAPIFillsAStructFromASingleRow(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/system/resource/print": {{"!re", "=board-name=CCR2116-12G-4S+", "=version=7.24.2 (stable)"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var resource SystemResource
	require.NoError(t, transport.List("/system/resource", &resource))

	assert.Equal(t, "CCR2116-12G-4S+", resource.BoardName)
	assert.Equal(t, "7.24.2 (stable)", resource.Version)
}

func TestAPIRefusesWrongCredentials(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "other"}

	_, err := dialAPI(apiConfig(router.start(t, nil)))

	assert.True(t, errors.Is(err, ErrUnauthorized), "got %v", err)
}

func TestAPIReportsATrapWithItsMessage(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/interface/vlan/print": {{"!trap", "=message=no such command prefix"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var vlans []VLAN
	err = transport.List("/interface/vlan", &vlans)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such command prefix")
	assert.False(t, errors.Is(err, ErrUnauthorized), "a refused command is not a refused login")
}

// RouterOS 7.18+ answers an empty listing with !empty before !done.
func TestAPITreatsEmptyAsNoRows(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/connection/print": {{"!empty"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var connections []BGPConnection
	require.NoError(t, transport.List("/routing/bgp/connection", &connections))
	assert.Empty(t, connections)
}

func TestAPIReportsFatal(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/interface/print": {{"!fatal", "session terminated on request"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var ifaces []Interface
	err = transport.List("/interface", &ifaces)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "session terminated on request")
}

func TestAPIRefusesAReplyOverTheBudget(t *testing.T) {
	row := []string{"!re", "=comment=" + strings.Repeat("x", 600<<10)}
	var reply [][]string
	for i := 0; i <= maxReplyBytes/len(row[1]); i++ {
		reply = append(reply, row)
	}
	reply = append(reply, []string{"!done"})
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/interface/print": reply,
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var ifaces []Interface
	err = transport.List("/interface", &ifaces)

	assert.True(t, errors.Is(err, errBadReply), "got %v", err)
}

func TestAPIGivesUpOnASilentRouter(t *testing.T) {
	shortenAPITimeout(t)
	router := &fakeAPIRouter{silent: true}
	port := router.start(t, nil)

	started := time.Now()
	_, err := dialAPI(apiConfig(port))

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}

// API chosen, but the port is a TLS service. The server reads the login
// sentence as a broken ClientHello and answers with an alert; the client must
// turn that into an error promptly, not hang or trust TLS bytes as a word
// length.
func TestAPIFailsFastAgainstATLSPort(t *testing.T) {
	shortenAPITimeout(t)
	cert, _ := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret"}
	port := router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}})

	started := time.Now()
	_, err := dialAPI(apiConfig(port))

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}

func TestAPISSLReadsThroughThePinnedCertificate(t *testing.T) {
	cert, fingerprint := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/system/clock/print": {{"!re", "=gmt-offset=+07:00"}, {"!done"}},
	}}
	cfg := apiConfig(router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}}))
	cfg.Protocol, cfg.Fingerprint = ProtocolAPISSL, fingerprint

	transport, err := dialAPI(cfg)
	require.NoError(t, err)

	var clock Clock
	require.NoError(t, transport.List("/system/clock", &clock))
	assert.Equal(t, "+07:00", clock.GMTOffset)
}

func TestAPISSLRefusesAnotherCertificateBeforeLoggingIn(t *testing.T) {
	cert, _ := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret"}
	cfg := apiConfig(router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}}))
	cfg.Protocol, cfg.Fingerprint = ProtocolAPISSL, "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := dialAPI(cfg)

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.Empty(t, router.received, "the password must not reach a router that failed the pin")
}

func TestProtocolDefaults(t *testing.T) {
	assert.Equal(t, 8443, ProtocolREST.DefaultPort())
	assert.Equal(t, 8729, ProtocolAPISSL.DefaultPort())
	assert.Equal(t, 8728, ProtocolAPI.DefaultPort())
	assert.True(t, ProtocolREST.UsesTLS())
	assert.True(t, ProtocolAPISSL.UsesTLS())
	assert.False(t, ProtocolAPI.UsesTLS())
	assert.False(t, Protocol("telnet").Valid())
	assert.False(t, Protocol("").Valid())
}

// dialAPI must not depend on its caller for this: a protocol that is not an
// API protocol would otherwise fall through to cleartext with the password.
func TestDialAPIRefusesNonAPIProtocolsBeforeDialing(t *testing.T) {
	for _, protocol := range []Protocol{ProtocolREST, ""} {
		router := &fakeAPIRouter{username: "tikman", password: "secret"}
		cfg := apiConfig(router.start(t, nil))
		cfg.Protocol = protocol

		_, err := dialAPI(cfg)

		assert.Error(t, err, "protocol %q", protocol)
		select {
		case words := <-router.received:
			t.Fatalf("protocol %q sent %v", protocol, words)
		case <-time.After(100 * time.Millisecond):
		}
	}
}
