package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func writeList(t *testing.T, path, text string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(text), 0o644))
	modified := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, modified, modified))
}

// listServer answers every request with status and body, counting requests.
func listServer(t *testing.T, status int, body string) (*httptest.Server, func() int) {
	t.Helper()
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, func() int { return hits }
}

func testAnonymitySource(t *testing.T, server *httptest.Server) *anonymitySource {
	t.Helper()
	source := newAnonymitySource(t.TempDir(), server.Client(), zap.NewNop())
	for _, l := range source.lists() {
		l.fetcher.URL = server.URL
	}
	return source
}

func TestAnonymityPreloadKeepsAFreshListWithoutDownloading(t *testing.T) {
	server, hits := listServer(t, http.StatusOK, "203.0.113.0/24\n")
	source := testAnonymitySource(t, server)
	writeList(t, source.tor.fetcher.Path, "198.51.100.7\n", time.Hour)
	writeList(t, source.vpn.fetcher.Path, "192.0.2.0/24\n", time.Hour)
	writeList(t, source.hosting.fetcher.Path, "198.51.100.0/24\n", time.Hour)

	source.preload()
	source.refresh(time.Now())

	assert.Zero(t, hits())
	assert.Equal(t, "tor", source.classify(probe))
	assert.Equal(t, "vpn", source.classify(netip.MustParseAddr("192.0.2.9")))
}

func TestAnonymityRefreshReplacesOnlyStaleLists(t *testing.T) {
	server, hits := listServer(t, http.StatusOK, "203.0.113.0/24\n")
	source := testAnonymitySource(t, server)
	writeList(t, source.tor.fetcher.Path, "198.51.100.7\n", time.Hour)
	writeList(t, source.vpn.fetcher.Path, "192.0.2.0/24\n", 7*time.Hour)

	source.preload()
	source.refresh(time.Now())

	assert.Equal(t, 2, hits(), "the stale VPN list and the missing hosting list")
	assert.Equal(t, "tor", source.classify(probe))
	assert.Equal(t, "vpn", source.classify(netip.MustParseAddr("203.0.113.1")))
	assert.Empty(t, source.classify(netip.MustParseAddr("192.0.2.9")), "the old VPN list is replaced")
}

func TestAnonymityRefreshFailureKeepsTheListInUse(t *testing.T) {
	server, _ := listServer(t, http.StatusTooManyRequests, "slow down")
	source := testAnonymitySource(t, server)
	writeList(t, source.tor.fetcher.Path, "198.51.100.7\n", 7*time.Hour)

	source.preload()
	source.refresh(time.Now())

	assert.Equal(t, "tor", source.classify(probe))
}

// typesRecorder records saved types, and fails with err when set.
type typesRecorder struct {
	saved map[uint32]string
	err   error
}

func (r *typesRecorder) SaveASNTypes(_ context.Context, types map[uint32]string, _ time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.saved = types
	return nil
}

func peeringDBBody(networks int) string {
	nets := make([]string, networks)
	for i := range nets {
		nets[i] = fmt.Sprintf(`{"asn":%d,"info_type":"Content"}`, 64496+i)
	}
	return `{"data":[` + strings.Join(nets, ",") + `]}`
}

func TestRefreshASNTypesSavesTheList(t *testing.T) {
	server, _ := listServer(t, http.StatusOK, peeringDBBody(2000))
	saver := &typesRecorder{}

	assert.True(t, refreshASNTypes(server.Client(), server.URL, saver, zap.NewNop()))
	assert.Len(t, saver.saved, 2000)
	assert.Equal(t, "Content", saver.saved[64496])
}

func TestRefreshASNTypesFailureSavesNothing(t *testing.T) {
	throttled, _ := listServer(t, http.StatusTooManyRequests, `{"meta":{"error":"throttled"}}`)
	saver := &typesRecorder{}
	assert.False(t, refreshASNTypes(throttled.Client(), throttled.URL, saver, zap.NewNop()))
	assert.Nil(t, saver.saved)

	ok, _ := listServer(t, http.StatusOK, peeringDBBody(2000))
	assert.False(t, refreshASNTypes(ok.Client(), ok.URL, &typesRecorder{err: errors.New("db down")}, zap.NewNop()),
		"a failed save is retried soon")
}
