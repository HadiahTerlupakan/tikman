package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

const (
	datasetB = "198.51.100.0\t198.51.100.255\t64497\tZZ\tEXAMPLE-NET-B\n"
	datasetC = "198.51.100.0\t198.51.100.255\t64498\tZZ\tEXAMPLE-NET-C\n"
)

// namesRecorder counts the names saved, and fails with err when set.
type namesRecorder struct {
	saved    int
	err      error
	deadline time.Duration
}

func (n *namesRecorder) SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, _ time.Time) error {
	if deadline, ok := ctx.Deadline(); ok {
		n.deadline = time.Until(deadline)
	}
	if n.err != nil {
		return n.err
	}
	n.saved += len(names)
	return nil
}

func gzipText(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(text))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func writeDataset(t *testing.T, path, text string, age time.Duration) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, gzipText(t, text), 0o644))
	modified := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(path, modified, modified))
}

// datasetServer answers every request with status and, on 200, text.
func datasetServer(t *testing.T, status int, text string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	body := gzipText(t, text)
	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func testSource(server *httptest.Server, path string) (*asnSource, *namesRecorder) {
	names := &namesRecorder{}
	return &asnSource{
		fetcher: &netflow.ASNFetcher{URL: server.URL, Path: path, Client: server.Client()},
		names:   names, logger: zap.NewNop(),
	}, names
}

var probe = netip.MustParseAddr("198.51.100.7")

func TestStartUsesAFreshCopyWithoutDownloading(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, hits := datasetServer(t, http.StatusOK, datasetC)
	source, names := testSource(server, path)

	source.start(time.Now())

	assert.Zero(t, hits.Load())
	assert.EqualValues(t, 64497, source.lookup(probe))
	assert.Equal(t, 1, names.saved)
}

func TestStartDownloadsOverADayOldCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, 25*time.Hour)
	server, _ := datasetServer(t, http.StatusOK, datasetC)
	source, _ := testSource(server, path)

	source.start(time.Now())

	assert.EqualValues(t, 64498, source.lookup(probe))
}

// An old table beats none: destinations a day stale are still mostly right.
func TestStartFallsBackToAStaleCopyWhenTheDownloadFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, 25*time.Hour)
	server, _ := datasetServer(t, http.StatusBadGateway, datasetC)
	source, _ := testSource(server, path)

	source.start(time.Now())

	assert.EqualValues(t, 64497, source.lookup(probe))
}

func TestRefreshKeepsTheTableInUseWhenTheDownloadFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, _ := datasetServer(t, http.StatusBadGateway, datasetC)
	source, _ := testSource(server, path)
	source.start(time.Now())

	source.refresh()

	assert.EqualValues(t, 64497, source.lookup(probe))
}

func TestLookupBeforeAnyDatasetIsOther(t *testing.T) {
	source := &asnSource{logger: zap.NewNop()}

	assert.EqualValues(t, netflow.OtherASN, source.lookup(probe))
}

// Names are for display; failing to store them must not keep the table out
// of the lookups that classify every flow.
func TestAFailedNameSaveStillInstallsTheTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, _ := datasetServer(t, http.StatusOK, datasetC)
	source, names := testSource(server, path)
	names.err = errDown

	source.start(time.Now())

	assert.EqualValues(t, 64497, source.lookup(probe))
}

// The full dataset's names are hundreds of thousands of rows: the upsert gets
// a deadline of its own, far beyond the one a minute's write gets.
func TestNameSaveGetsTheDatasetDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), asnFileName)
	writeDataset(t, path, datasetB, time.Hour)
	server, _ := datasetServer(t, http.StatusOK, datasetC)
	source, names := testSource(server, path)

	source.start(time.Now())

	assert.Greater(t, names.deadline, asnSaveTimeout-time.Minute)
	assert.Greater(t, names.deadline, 10*flowWriteTimeout)
}
