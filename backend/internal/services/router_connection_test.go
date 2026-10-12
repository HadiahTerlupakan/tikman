package services

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// fakeRouterTLSServer serves a RouterOS-like REST endpoint over TLS and counts the
// requests it receives, so a test can prove that no credentials were sent.
func fakeRouterTLSServer(t *testing.T) (*httptest.Server, *atomic.Int32, int) {
	t.Helper()
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"board-name":"CCR2116-12G-4S+","version":"7.24.2 (stable)"}`))
	}))
	t.Cleanup(server.Close)
	_, portText, _ := net.SplitHostPort(server.Listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	return server, &hits, port
}

func TestConnectionTestSendsNoCredentialsBeforeTheFingerprintIsConfirmed(t *testing.T) {
	server, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIPort: port, APIUsername: "u", APIPassword: "p",
	})

	assert.Equal(t, mikrotik.Fingerprint(server.Certificate().Raw), result.Fingerprint)
	assert.Empty(t, result.BoardName)
	assert.Empty(t, result.APIError)
	assert.Zero(t, hits.Load(), "the password must not reach a certificate nobody has confirmed")
}

func TestConnectionTestReadsTheBoardWithTheConfirmedFingerprint(t *testing.T) {
	server, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIPort: port, APIUsername: "u", APIPassword: "p",
		TLSFingerprint: mikrotik.Fingerprint(server.Certificate().Raw),
	})

	assert.Empty(t, result.APIError)
	assert.Equal(t, "CCR2116-12G-4S+", result.BoardName)
	assert.Equal(t, "7.24.2 (stable)", result.Version)
	assert.Equal(t, int32(1), hits.Load())
	assert.NotEmpty(t, result.SNMPError, "nothing answers SNMP on port 1")
}

func TestConnectionTestRefusesAWrongFingerprintWithoutSendingCredentials(t *testing.T) {
	_, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, APIPort: port, APIUsername: "u", APIPassword: "p",
		TLSFingerprint: testFingerprint,
	})

	assert.NotEmpty(t, result.APIError)
	assert.Empty(t, result.BoardName)
	assert.Zero(t, hits.Load())
}

func TestConnectionTestRejectsAMalformedFingerprint(t *testing.T) {
	_, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, APIPort: port, APIUsername: "u", APIPassword: "p",
		TLSFingerprint: "abcd",
	})

	assert.NotEmpty(t, result.APIError)
	assert.Zero(t, hits.Load())
}

func TestConnectionTestReportsAnUnreachableAPI(t *testing.T) {
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{Host: "127.0.0.1", SNMPPort: 1, APIPort: 1})

	assert.NotEmpty(t, result.APIError)
	assert.Empty(t, result.Fingerprint)
}

func TestTrustCertificateAcceptsOnlyWhatTheRouterPresented(t *testing.T) {
	service, db := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	presented := "cd" + testFingerprint[2:]
	require.NoError(t, db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Updates(map[string]interface{}{
		"api_status": models.RouterAPIStatusCertificateChanged, "presented_fingerprint": presented,
	}).Error)

	_, err = service.TrustCertificate(router.ID, testFingerprint)
	assert.True(t, errors.Is(err, ErrFingerprintMismatch), "a fingerprint the router never showed is refused")

	trusted, err := service.TrustCertificate(router.ID, presented)
	require.NoError(t, err)
	assert.Equal(t, presented, trusted.TLSFingerprint)
	assert.Nil(t, trusted.PresentedFingerprint)
	assert.Equal(t, models.RouterAPIStatusUnknown, trusted.APIStatus)
}

func TestTrustCertificateRefusesWhenNothingChanged(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	_, err = service.TrustCertificate(router.ID, testFingerprint)
	assert.True(t, errors.Is(err, ErrFingerprintMismatch))
}

// plainAPIListener records the first bytes a client sends, then hangs up.
func plainAPIListener(t *testing.T) (int, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	first := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 64)
		n, _ := io.ReadAtLeast(conn, buf, 6)
		first <- string(buf[:n])
	}()
	return listener.Addr().(*net.TCPAddr).Port, first
}

// Plain api has no certificate to confirm, so the test logs in at once and
// comes back without a fingerprint.
func TestConnectionTestOverPlainAPILogsInWithoutAFingerprint(t *testing.T) {
	port, first := plainAPIListener(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIProtocol: "api", APIPort: port,
		APIUsername: "u", APIPassword: "p",
	})

	assert.Contains(t, <-first, "/login")
	assert.Empty(t, result.Fingerprint)
	assert.NotEmpty(t, result.APIError, "the listener hangs up instead of answering")
}

func TestConnectionTestOverAPISSLStartsWithTheFingerprint(t *testing.T) {
	server, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIProtocol: "api-ssl", APIPort: port,
		APIUsername: "u", APIPassword: "p",
	})

	assert.Equal(t, mikrotik.Fingerprint(server.Certificate().Raw), result.Fingerprint)
	assert.Empty(t, result.APIError)
	assert.Zero(t, hits.Load())
}

func TestConnectionTestRejectsAnUnknownProtocol(t *testing.T) {
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{Host: "127.0.0.1", SNMPPort: 1, APIProtocol: "telnet"})

	assert.Contains(t, result.APIError, "api_protocol")
}
