package netflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ASNDatasetURL is iptoasn.com's IPv4 dataset, rebuilt daily.
const ASNDatasetURL = "https://iptoasn.com/data/ip2asn-v4.tsv.gz"

// ASNMaxAge is how old the copy on disk may be before flowd downloads again.
const ASNMaxAge = 24 * time.Hour

const (
	// asnDownloadTimeout covers the whole transfer of a file of a few MB.
	asnDownloadTimeout = 2 * time.Minute
	// maxASNDownload bounds what a misbehaving server can make flowd write.
	maxASNDownload = 64 << 20
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

// Download fetches the dataset into a temporary file beside Path, parses it,
// and only then renames it over Path. A failed or invalid download leaves the
// previous copy, and the table built from it, as they were.
func (f *ASNFetcher) Download() (*ASNTable, time.Time, error) {
	tmp, err := os.CreateTemp(filepath.Dir(f.Path), ".ip2asn-*")
	if err != nil {
		return nil, time.Time{}, err
	}
	// After a successful rename this finds nothing to remove.
	defer func() { _ = os.Remove(tmp.Name()) }()

	fetchErr := f.fetchTo(tmp)
	if closeErr := tmp.Close(); fetchErr == nil {
		fetchErr = closeErr
	}
	if fetchErr != nil {
		return nil, time.Time{}, fetchErr
	}
	table, err := LoadASNFile(tmp.Name())
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("downloaded dataset is invalid: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.Path); err != nil {
		return nil, time.Time{}, err
	}
	return table, time.Now(), nil
}

func (f *ASNFetcher) fetchTo(w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), asnDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", f.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", f.URL, resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, maxASNDownload+1))
	if err != nil {
		return err
	}
	if n > maxASNDownload {
		return fmt.Errorf("dataset larger than %d bytes", maxASNDownload)
	}
	return nil
}
