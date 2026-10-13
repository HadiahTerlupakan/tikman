package netflow

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const geoFixture = `0.0.0.0,0.255.255.255,ZZ
198.51.100.0,198.51.100.255,JP
192.0.2.0,192.0.2.255,DE
2001:db8::,2001:db8::ffff,US
`

func TestGeoLookupPlacesAnAddressInItsRange(t *testing.T) {
	table, err := ParseGeoTable(strings.NewReader(geoFixture))
	require.NoError(t, err)

	assert.Equal(t, "DE", table.Lookup(netip.MustParseAddr("192.0.2.200")))
	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.0")))
	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.255")))
	assert.Empty(t, table.Lookup(netip.MustParseAddr("198.51.101.0")), "between ranges")
	assert.Empty(t, table.Lookup(netip.MustParseAddr("0.0.0.1")), "ZZ is unknown")
	assert.Empty(t, table.Lookup(netip.MustParseAddr("2001:db8::1")), "IPv6 is not loaded")
}

func TestParseGeoTableRejectsABadFile(t *testing.T) {
	for name, body := range map[string]string{
		"bad address":    "192.0.2.0,not-an-address,DE\n",
		"bad country":    "192.0.2.0,192.0.2.255,Germany\n",
		"missing column": "192.0.2.0,192.0.2.255\n",
		"reversed range": "192.0.2.255,192.0.2.0,DE\n",
		"overlap":        "192.0.2.0,192.0.2.255,DE\n192.0.2.128,192.0.3.0,JP\n",
		"no IPv4":        "2001:db8::,2001:db8::ffff,US\n",
	} {
		_, err := ParseGeoTable(strings.NewReader(body))
		assert.Error(t, err, name)
	}
}

// DB-IP publishes each month's file some days into the month, so early on
// only last month's exists.
func TestGeoDownloadFallsBackToLastMonth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dbip-country-lite.csv.gz")
	body := gzipped(t, geoFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2026-09" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()
	fetcher := &GeoFetcher{Path: path, Client: server.Client(),
		URL: func(t time.Time) string { return server.URL + "/" + t.Format("2006-01") }}

	table, err := fetcher.Download(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.1")))
	stored, err := fetcher.LoadLocal()
	require.NoError(t, err)
	assert.Equal(t, "DE", stored.Lookup(netip.MustParseAddr("192.0.2.1")))
}

func TestGeoDownloadFailureLeavesThePreviousCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dbip-country-lite.csv.gz")
	previous := gzipped(t, geoFixture)
	require.NoError(t, os.WriteFile(path, previous, 0o644))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(gzipped(t, "192.0.2.0,192.0.2.255,Germany\n"))
	}))
	defer server.Close()
	fetcher := &GeoFetcher{Path: path, Client: server.Client(),
		URL: func(time.Time) string { return server.URL }}

	_, err := fetcher.Download(time.Now())

	assert.Error(t, err)
	kept, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, previous, kept)
}

func TestGeoDatasetURLNamesTheMonth(t *testing.T) {
	assert.Equal(t, "https://download.db-ip.com/free/dbip-country-lite-2026-10.csv.gz",
		GeoDatasetURL(time.Date(2026, 10, 13, 23, 0, 0, 0, time.UTC)))
}
