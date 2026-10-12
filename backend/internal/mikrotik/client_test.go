package mikrotik

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A trimmed, documentation-range copy of what /rest/routing/bgp/session
// returns. Every value is a string, as RouterOS sends it.
const sessionsJSON = `[
 {".id":"*1","name":"peer-transit-1","remote.address":"198.51.100.1","remote.as":"64500",
  "local.address":"198.51.100.0","established":"true","uptime":"15h53m9s990ms",
  "prefix-count":"1093192","last-started":"2026-10-11 16:06:43","last-stopped":"2026-10-11 16:06:39"},
 {".id":"*2","name":"peer-ix-rs1-new-1","remote.address":"203.0.113.166","remote.as":"0",
  "local.address":"203.0.113.140","prefix-count":"0"}
]`

func tlsRouter(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string, int) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	return server, host, port
}

func TestClientReadsSessionsFromAPinnedRouter(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "tikman" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "/rest/routing/bgp/session", r.URL.Path)
		_, _ = w.Write([]byte(sessionsJSON))
	})

	client := openREST(t, host, port, "tikman", "secret", Fingerprint(server.Certificate().Raw))
	sessions, err := client.BGPSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 2)
	assert.Equal(t, "peer-transit-1", sessions[0].Name)
	assert.Equal(t, "198.51.100.1", sessions[0].RemoteAddress)
	assert.Equal(t, "true", sessions[0].Established)
	assert.Equal(t, "1093192", sessions[0].PrefixCount)
	assert.Equal(t, "2026-10-11 16:06:39", sessions[0].LastStopped)
	assert.Empty(t, sessions[1].Established)
}

// The router's certificate is self-signed, so the pin is the only thing
// standing between the stored password and whoever answers on that address.
func TestClientRefusesARouterWhoseCertificateChanged(t *testing.T) {
	called := false
	_, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	})

	pinned := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := openREST(t, host, port, "tikman", "secret", pinned).SystemResource()

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.False(t, called, "credentials must not reach a router that failed the pin")
}

// A client lives for one poll. A kept-alive connection would outlive it with
// nothing left to close it, so every request must ask the router to close.
func TestClientDoesNotKeepConnectionsAlive(t *testing.T) {
	closes := make(chan bool, 1)
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		closes <- r.Close
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := openREST(t, host, port, "tikman", "secret", Fingerprint(server.Certificate().Raw)).SystemResource()

	require.NoError(t, err)
	assert.True(t, <-closes, "request must carry Connection: close")
}

func TestClientReportsRejectedCredentials(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := openREST(t, host, port, "tikman", "wrong", Fingerprint(server.Certificate().Raw)).SystemResource()

	assert.True(t, errors.Is(err, ErrUnauthorized), "got %v", err)
}

func TestClientReportsAnHTTPErrorWithItsBody(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":400,"message":"no such command"}`))
	})

	_, err := openREST(t, host, port, "u", "p", Fingerprint(server.Certificate().Raw)).Clock()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 400")
	assert.Contains(t, err.Error(), "no such command")
}

func TestFetchFingerprintReturnsWhatTheRouterPresents(t *testing.T) {
	server, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {})

	got, err := FetchFingerprint(host, port)

	require.NoError(t, err)
	assert.Equal(t, Fingerprint(server.Certificate().Raw), got)
	assert.Len(t, got, 64)
}

func TestFetchFingerprintFailsWhenNothingListens(t *testing.T) {
	_, err := FetchFingerprint("127.0.0.1", 1)
	assert.Error(t, err)
}

func openREST(t *testing.T, host string, port int, username, password, fingerprint string) *Client {
	t.Helper()
	client, err := Open(Config{Protocol: ProtocolREST, Host: host, Port: port,
		Username: username, Password: password, Fingerprint: fingerprint})
	require.NoError(t, err)
	return client
}

func TestOpenRejectsAnUnknownProtocol(t *testing.T) {
	_, err := Open(Config{Protocol: "telnet", Host: "127.0.0.1", Port: 1})
	assert.Error(t, err)
}

// The typed reads are shared by every transport: what works over REST works
// over the API without a second implementation.
func TestOpenWithAPIReadsThroughTheSameMethods(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/session/print": {{"!re", "=name=peer-transit-1", "=prefix-count=1093192"}, {"!done"}},
	}}
	client, err := Open(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sessions, err := client.BGPSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "1093192", sessions[0].PrefixCount)
}

// Review Focus 4: a router edited from api to rest keeps an empty pin. It
// must be refused as a changed certificate, which hands it to the existing
// accept-certificate flow, and never be read unpinned.
func TestOpenRESTWithNoPinnedFingerprintRefusesTheRouter(t *testing.T) {
	called := false
	_, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := openREST(t, host, port, "tikman", "secret", "").SystemResource()

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.False(t, called)
}
