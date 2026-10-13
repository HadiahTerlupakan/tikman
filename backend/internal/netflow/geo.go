package netflow

import (
	"bufio"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strings"
	"time"
)

// GeoMaxAge is how old the copy on disk may be before flowd downloads again.
// DB-IP publishes a new file each month; a week finds it soon enough.
const GeoMaxAge = 7 * 24 * time.Hour

// geoColumns is start, end, country.
const geoColumns = 3

// geoUnknown is DB-IP's code for space it cannot place.
const geoUnknown = "ZZ"

// maxGeoInflated bounds the decompressed dataset, about 30 MB today.
const maxGeoInflated = 256 << 20

var errGeoTooLarge = errors.New("dbip dataset inflates past its size limit")

// GeoDatasetURL is DB-IP's free IP-to-country file for the month holding t.
// It is CC BY 4.0, which is why the UI credits DB-IP beside the flags.
func GeoDatasetURL(t time.Time) string {
	return fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%s.csv.gz", t.UTC().Format("2006-01"))
}

type geoRange struct {
	start, end uint32
	country    string
}

// GeoTable maps an IPv4 address to the country it is located in, from
// DB-IP's country lite dataset: sorted ranges, searched by binary search.
type GeoTable struct {
	ranges []geoRange
}

// ParseGeoTable reads the uncompressed CSV. Its IPv6 lines are skipped, as
// flows are IPv4 only. Any malformed IPv4 line fails the whole table, so a
// truncated or substituted download never replaces a good one.
func ParseGeoTable(r io.Reader) (*GeoTable, error) {
	t := &GeoTable{}
	scanner := bufio.NewScanner(r)
	for line := 1; scanner.Scan(); line++ {
		if err := t.addLine(scanner.Text()); err != nil {
			return nil, fmt.Errorf("dbip line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(t.ranges) == 0 {
		return nil, errors.New("dbip: no IPv4 ranges")
	}
	sort.Slice(t.ranges, func(i, j int) bool { return t.ranges[i].start < t.ranges[j].start })
	for i := 1; i < len(t.ranges); i++ {
		if t.ranges[i].start <= t.ranges[i-1].end {
			return nil, fmt.Errorf("dbip: range starting %s overlaps the one ending %s",
				ipv4String(t.ranges[i].start), ipv4String(t.ranges[i-1].end))
		}
	}
	return t, nil
}

func (t *GeoTable) addLine(line string) error {
	cols := strings.Split(line, ",")
	if len(cols) != geoColumns {
		return fmt.Errorf("want %d comma-separated columns, got %d", geoColumns, len(cols))
	}
	if strings.Contains(cols[0], ":") {
		return nil
	}
	start, errStart := parseIPv4(cols[0])
	end, errEnd := parseIPv4(cols[1])
	if err := errors.Join(errStart, errEnd); err != nil {
		return err
	}
	if end < start {
		return errors.New("range ends before it starts")
	}
	if !isCountryCode(cols[2]) {
		return fmt.Errorf("%q is not a country code", cols[2])
	}
	if cols[2] == geoUnknown {
		return nil
	}
	t.ranges = append(t.ranges, geoRange{start: start, end: end, country: cols[2]})
	return nil
}

func isCountryCode(s string) bool {
	return len(s) == 2 && s[0] >= 'A' && s[0] <= 'Z' && s[1] >= 'A' && s[1] <= 'Z'
}

// Lookup returns the ISO 3166 code of the country ip is located in, or ""
// when the dataset cannot place it.
func (t *GeoTable) Lookup(ip netip.Addr) string {
	if !ip.Is4() {
		return ""
	}
	b := ip.As4()
	v := binary.BigEndian.Uint32(b[:])
	i := sort.Search(len(t.ranges), func(i int) bool { return t.ranges[i].start > v }) - 1
	if i < 0 || v > t.ranges[i].end {
		return ""
	}
	return t.ranges[i].country
}

// LoadGeoFile reads a gzipped dataset from disk.
func LoadGeoFile(path string) (*GeoTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("dbip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return ParseGeoTable(&cappedReader{r: gz, left: maxGeoInflated, err: errGeoTooLarge})
}

// GeoFetcher keeps the dataset on disk and loads it.
type GeoFetcher struct {
	Path   string
	Client *http.Client
	// URL names the file for a month; GeoDatasetURL outside tests.
	URL func(time.Time) string
}

// Stale reports whether the copy on disk is missing or older than GeoMaxAge.
func (f *GeoFetcher) Stale(now time.Time) bool {
	info, err := os.Stat(f.Path)
	return err != nil || now.Sub(info.ModTime()) > GeoMaxAge
}

// LoadLocal reads the copy on disk.
func (f *GeoFetcher) LoadLocal() (*GeoTable, error) {
	return LoadGeoFile(f.Path)
}

// Download fetches this month's file, or last month's while this month's is
// not yet published.
func (f *GeoFetcher) Download(now time.Time) (*GeoTable, error) {
	table, err := downloadDataset(f.Client, f.URL(now), f.Path, LoadGeoFile)
	if err == nil {
		return table, nil
	}
	previous := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	table, prevErr := downloadDataset(f.Client, f.URL(previous), f.Path, LoadGeoFile)
	if prevErr != nil {
		return nil, errors.Join(err, prevErr)
	}
	return table, nil
}
