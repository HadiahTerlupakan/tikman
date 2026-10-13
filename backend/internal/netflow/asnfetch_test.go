package netflow

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadStoresAValidDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	body := gzipped(t, asnFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	table, _, err := (&ASNFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()

	require.NoError(t, err)
	assert.Equal(t, uint32(64497), table.Lookup(netip.MustParseAddr("198.51.100.1")))
	stored, err := LoadASNFile(path)
	require.NoError(t, err)
	assert.Equal(t, uint32(64496), stored.Lookup(netip.MustParseAddr("192.0.2.1")))
}

func TestDownloadFailureLeavesThePreviousCopy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ip2asn-v4.tsv.gz")
	previous := gzipped(t, asnFixture)
	require.NoError(t, os.WriteFile(path, previous, 0o644))
	malformed := gzipped(t, "192.0.2.0\tnot-an-address\t64496\tZZ\tX\n")

	handlers := map[string]http.HandlerFunc{
		"server error":      func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
		"not gzip":          func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>maintenance</html>")) },
		"malformed dataset": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(malformed) },
	}
	for name, handler := range handlers {
		server := httptest.NewServer(handler)
		_, _, err := (&ASNFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()
		server.Close()

		assert.Error(t, err, name)
		kept, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, previous, kept, name)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".dataset-*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "a failed download leaves no temporary file")
}

func TestStaleFollowsTheCopysAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ip2asn-v4.tsv.gz")
	fetcher := &ASNFetcher{Path: path}
	now := time.Now()
	assert.True(t, fetcher.Stale(now), "no copy yet")

	require.NoError(t, os.WriteFile(path, gzipped(t, asnFixture), 0o644))
	assert.False(t, fetcher.Stale(now))

	old := now.Add(-ASNMaxAge - time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	assert.True(t, fetcher.Stale(now))
}
