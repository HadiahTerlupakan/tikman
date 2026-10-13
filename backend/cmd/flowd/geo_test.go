package main

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

const geoCSVHeader = "network,country,country_code,continent,continent_code,asn,as_name,as_domain\n"

func testGeoSource(t *testing.T, age time.Duration, served string) (*geoSource, func() int32) {
	t.Helper()
	path := filepath.Join(t.TempDir(), geoFileName)
	writeDataset(t, path, geoCSVHeader+"198.51.100.0/24,Japan,JP,Asia,AS,,,\n", age)
	server, hits := datasetServer(t, http.StatusOK, geoCSVHeader+served)
	return &geoSource{
		fetcher: &netflow.GeoFetcher{URL: server.URL, Path: path, Client: server.Client()},
		logger:  zap.NewNop(),
	}, hits.Load
}

// main installs the copy on disk before the socket opens; start must not
// read the same fresh file a second time.
func TestGeoStartKeepsAFreshPreloadedCopy(t *testing.T) {
	source, hits := testGeoSource(t, time.Hour, "198.51.100.0/24,Singapore,SG,Asia,AS,,,\n")

	source.installLocal()
	source.start(time.Now())

	assert.Equal(t, "JP", source.lookup(probe))
	assert.Zero(t, hits())
}

func TestGeoStartReplacesAStalePreloadedCopy(t *testing.T) {
	source, hits := testGeoSource(t, 2*netflow.GeoMaxAge, "198.51.100.0/24,Singapore,SG,Asia,AS,,,\n")

	source.installLocal()
	assert.Equal(t, "JP", source.lookup(probe), "the stale copy serves until the download lands")
	source.start(time.Now())

	assert.Equal(t, "SG", source.lookup(probe))
	assert.EqualValues(t, 1, hits())
}
