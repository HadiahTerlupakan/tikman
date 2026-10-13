package netflow

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/csv"
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

// GeoMaxAge is how old the copy on disk may be before flowd downloads again;
// IPinfo rebuilds the file daily.
const GeoMaxAge = 24 * time.Hour

// GeoDatasetURL is IPinfo's free Lite dataset. The token is the account's
// own, so the URL must never reach a log; see redactURL.
const GeoDatasetURL = "https://ipinfo.io/data/ipinfo_lite.csv.gz?token="

// geoNetworkColumn and geoCountryColumn index IPinfo Lite's header:
// network, country, country_code, continent, continent_code, asn, as_name,
// as_domain.
const (
	geoNetworkColumn = 0
	geoCountryColumn = 2
	geoColumns       = 8
)

// maxGeoInflated bounds the decompressed dataset, about 330 MB today.
const maxGeoInflated = 1 << 30

var errGeoTooLarge = errors.New("ipinfo dataset inflates past its size limit")

type geoRange struct {
	start, end uint32
	country    [2]byte
}

// GeoTable maps an IPv4 address to the country it is located in, from
// IPinfo's Lite dataset: sorted ranges, searched by binary search.
type GeoTable struct {
	ranges []geoRange
}

// ParseGeoTable reads the uncompressed CSV. Its IPv6 networks are skipped, as
// flows are IPv4 only. Any malformed IPv4 line fails the whole table, so a
// truncated or substituted download never replaces a good one.
func ParseGeoTable(r io.Reader) (*GeoTable, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = geoColumns
	reader.ReuseRecord = true
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("ipinfo header: %w", err)
	}
	t := &GeoTable{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("ipinfo: %w", err)
		}
		if err := t.add(record[geoNetworkColumn], record[geoCountryColumn]); err != nil {
			line, _ := reader.FieldPos(0)
			return nil, fmt.Errorf("ipinfo line %d: %w", line, err)
		}
	}
	if len(t.ranges) == 0 {
		return nil, errors.New("ipinfo: no IPv4 networks")
	}
	sort.Slice(t.ranges, func(i, j int) bool { return t.ranges[i].start < t.ranges[j].start })
	for i := 1; i < len(t.ranges); i++ {
		if t.ranges[i].start <= t.ranges[i-1].end {
			return nil, fmt.Errorf("ipinfo: network starting %s overlaps the one ending %s",
				ipv4String(t.ranges[i].start), ipv4String(t.ranges[i-1].end))
		}
	}
	return t, nil
}

func (t *GeoTable) add(network, country string) error {
	if strings.Contains(network, ":") {
		return nil
	}
	prefix, err := parseNetwork(network)
	if err != nil || !prefix.Addr().Is4() {
		return fmt.Errorf("%q is not an IPv4 network", network)
	}
	if country == "" {
		return nil
	}
	if !isCountryCode(country) {
		return fmt.Errorf("%q is not a country code", country)
	}
	b := prefix.Masked().Addr().As4()
	start := binary.BigEndian.Uint32(b[:])
	end := start | (1<<(32-prefix.Bits()) - 1)
	t.ranges = append(t.ranges, geoRange{start: start, end: end, country: [2]byte{country[0], country[1]}})
	return nil
}

// parseNetwork reads a CIDR network or, as IPinfo writes some, a bare
// address standing for its /32.
func parseNetwork(network string) (netip.Prefix, error) {
	if strings.Contains(network, "/") {
		return netip.ParsePrefix(network)
	}
	addr, err := netip.ParseAddr(network)
	if err != nil {
		return netip.Prefix{}, err
	}
	return addr.Prefix(addr.BitLen())
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
	return string(t.ranges[i].country[:])
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
		return nil, fmt.Errorf("ipinfo: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return ParseGeoTable(&cappedReader{r: gz, left: maxGeoInflated, err: errGeoTooLarge})
}

// GeoFetcher keeps the dataset on disk and loads it.
type GeoFetcher struct {
	URL    string
	Path   string
	Client *http.Client
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

// Download fetches the dataset and loads it; see downloadDataset.
func (f *GeoFetcher) Download() (*GeoTable, error) {
	return downloadDataset(f.Client, f.URL, f.Path, LoadGeoFile)
}
