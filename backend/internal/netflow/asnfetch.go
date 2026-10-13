package netflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ASNDatasetURL is iptoasn.com's IPv4 dataset, rebuilt daily.
const ASNDatasetURL = "https://iptoasn.com/data/ip2asn-v4.tsv.gz"

// ASNMaxAge is how old the copy on disk may be before flowd downloads again.
const ASNMaxAge = 24 * time.Hour

const (
	// datasetDownloadTimeout covers the whole transfer of a file of a few MB.
	datasetDownloadTimeout = 2 * time.Minute
	// maxDatasetDownload bounds what a misbehaving server can make flowd write.
	maxDatasetDownload = 64 << 20
)

// ASNFetcher keeps the dataset on disk and loads it.
type ASNFetcher struct {
	URL    string
	Path   string
	Client *http.Client
}

// Stale reports whether the copy on disk is missing or older than ASNMaxAge.
func (f *ASNFetcher) Stale(now time.Time) bool {
	info, err := os.Stat(f.Path)
	return err != nil || now.Sub(info.ModTime()) > ASNMaxAge
}

// LoadLocal reads the copy on disk and reports when it was downloaded.
func (f *ASNFetcher) LoadLocal() (*ASNTable, time.Time, error) {
	info, err := os.Stat(f.Path)
	if err != nil {
		return nil, time.Time{}, err
	}
	table, err := LoadASNFile(f.Path)
	return table, info.ModTime(), err
}

// Download fetches the dataset and loads it; see downloadDataset.
func (f *ASNFetcher) Download() (*ASNTable, time.Time, error) {
	table, err := downloadDataset(f.Client, f.URL, f.Path, LoadASNFile)
	return table, time.Now(), err
}

// downloadDataset fetches url into a temporary file beside path, loads it,
// and only then renames it over path. A failed or invalid download leaves the
// previous copy, and the table built from it, as they were.
func downloadDataset[T any](client *http.Client, url, path string, load func(string) (T, error)) (T, error) {
	var none T
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dataset-*")
	if err != nil {
		return none, err
	}
	// After a successful rename this finds nothing to remove.
	defer func() { _ = os.Remove(tmp.Name()) }()

	fetchErr := fetchTo(client, url, nil, tmp)
	if closeErr := tmp.Close(); fetchErr == nil {
		fetchErr = closeErr
	}
	if fetchErr != nil {
		return none, fetchErr
	}
	table, err := load(tmp.Name())
	if err != nil {
		return none, fmt.Errorf("downloaded dataset is invalid: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return none, err
	}
	return table, nil
}

// fetchTo GETs rawURL into w, sending header (nil for none) with it.
func fetchTo(client *http.Client, rawURL string, header http.Header, w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), datasetDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("GET %s: invalid URL", redactURL(rawURL))
	}
	for name, values := range header {
		req.Header[name] = values
	}
	resp, err := client.Do(req)
	if err != nil {
		// A *url.Error repeats the whole URL, token included; keep only its cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("GET %s: %w", redactURL(rawURL), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", redactURL(rawURL), resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxDatasetDownload+1))
	if err != nil {
		return err
	}
	if n > maxDatasetDownload {
		return fmt.Errorf("dataset larger than %d bytes", maxDatasetDownload)
	}
	return nil
}

// redactURL drops the query, where a dataset's access token travels, so an
// error can name the URL in a log.
func redactURL(rawURL string) string {
	if i := strings.IndexByte(rawURL, '?'); i >= 0 {
		return rawURL[:i] + "?…"
	}
	return rawURL
}
