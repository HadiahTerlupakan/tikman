package netflow

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const geoHeader = "network,country,country_code,continent,continent_code,asn,as_name,as_domain\n"

const geoFixture = geoHeader + `198.51.100.0/24,Japan,JP,Asia,AS,AS64497,"Example, Inc.",example.net
192.0.2.0/25,Germany,DE,Europe,EU,,,
192.0.2.128/25,,,,,,,
203.0.113.9,Singapore,SG,Asia,AS,,,
2001:db8::/32,United States,US,North America,NA,AS64496,Example,example.com
`

func TestGeoLookupPlacesAnAddressInItsNetwork(t *testing.T) {
	table, err := ParseGeoTable(strings.NewReader(geoFixture))
	require.NoError(t, err)

	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.0")))
	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.255")), "quoted AS name with a comma")
	assert.Equal(t, "DE", table.Lookup(netip.MustParseAddr("192.0.2.127")))
	assert.Empty(t, table.Lookup(netip.MustParseAddr("192.0.2.128")), "a network with no country")
	assert.Empty(t, table.Lookup(netip.MustParseAddr("198.51.101.0")), "between networks")
	assert.Equal(t, "SG", table.Lookup(netip.MustParseAddr("203.0.113.9")), "a bare address is its /32")
	assert.Empty(t, table.Lookup(netip.MustParseAddr("203.0.113.10")), "and nothing past it")
	assert.Empty(t, table.Lookup(netip.MustParseAddr("2001:db8::1")), "IPv6 is not loaded")
}

func TestParseGeoTableRejectsABadFile(t *testing.T) {
	for name, body := range map[string]string{
		"no header":      "",
		"bad network":    geoHeader + "198.51.100.0/33,Japan,JP,Asia,AS,,,\n",
		"bad country":    geoHeader + "198.51.100.0/24,Japan,Japan,Asia,AS,,,\n",
		"missing column": geoHeader + "198.51.100.0/24,Japan,JP\n",
		"overlap":        geoHeader + "198.51.100.0/24,Japan,JP,Asia,AS,,,\n198.51.100.128/25,Germany,DE,Europe,EU,,,\n",
		"no IPv4":        geoHeader + "2001:db8::/32,United States,US,North America,NA,,,\n",
	} {
		_, err := ParseGeoTable(strings.NewReader(body))
		assert.Error(t, err, name)
	}
}

func TestGeoDownloadStoresAValidDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipinfo_lite.csv.gz")
	body := gzipped(t, geoFixture)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	fetcher := &GeoFetcher{URL: server.URL + "/data?token=secret", Path: path, Client: server.Client()}

	table, err := fetcher.Download()

	require.NoError(t, err)
	assert.Equal(t, "JP", table.Lookup(netip.MustParseAddr("198.51.100.1")))
	stored, err := fetcher.LoadLocal()
	require.NoError(t, err)
	assert.Equal(t, "DE", stored.Lookup(netip.MustParseAddr("192.0.2.1")))
}

func TestGeoDownloadFailureLeavesThePreviousCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipinfo_lite.csv.gz")
	previous := gzipped(t, geoFixture)
	require.NoError(t, os.WriteFile(path, previous, 0o644))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(gzipped(t, geoHeader+"198.51.100.0/24,Japan,Japan,Asia,AS,,,\n"))
	}))
	defer server.Close()

	_, err := (&GeoFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()

	assert.Error(t, err)
	kept, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, previous, kept)
}

// The token rides in the query string, and download errors are logged.
func TestGeoDownloadErrorsNeverCarryTheToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipinfo_lite.csv.gz")
	refused := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer refused.Close()
	unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	unreachable.Close()

	for name, base := range map[string]string{"HTTP error": refused.URL, "connection error": unreachable.URL} {
		_, err := (&GeoFetcher{URL: base + "/data?token=s3cr3t-token", Path: path, Client: http.DefaultClient}).Download()

		require.Error(t, err, name)
		assert.NotContains(t, err.Error(), "s3cr3t-token", name)
	}
}
