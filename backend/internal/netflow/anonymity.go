package netflow

import (
	"net/http"
	"net/netip"
	"os"
	"time"
)

// Anonymity flags a top destination address can carry, from free lists.
const (
	AnonymityTor     = "tor"
	AnonymityVPN     = "vpn"
	AnonymityHosting = "hosting"
)

// AnonymityMaxAge is how old a list on disk may be before flowd downloads it
// again. Tor's exit list moves within hours; the others change daily.
const AnonymityMaxAge = 6 * time.Hour

// The lists, all free. Tor's comes from the Tor Project itself. X4BNet's
// collect networks VPN providers and hosting companies announce; they mark
// whole provider networks, so a CDN a VPN provider rents from can read as VPN.
const (
	TorExitListURL    = "https://check.torproject.org/torbulkexitlist"
	VPNListURL        = "https://raw.githubusercontent.com/X4BNet/lists_vpn/main/output/vpn/ipv4.txt"
	DatacenterListURL = "https://raw.githubusercontent.com/X4BNet/lists_vpn/main/output/datacenter/ipv4.txt"
)

// AnonymityLists flags an address by the first list holding it, most
// specific first: a Tor exit sits in a hosting network too.
type AnonymityLists struct {
	Tor, VPN, Hosting *IPSet
}

// Classify returns the address's flag, or "" when no list holds it.
func (l AnonymityLists) Classify(ip netip.Addr) string {
	switch {
	case l.Tor.Contains(ip):
		return AnonymityTor
	case l.VPN.Contains(ip):
		return AnonymityVPN
	case l.Hosting.Contains(ip):
		return AnonymityHosting
	}
	return ""
}

// ListFetcher keeps one plain-text address list on disk and loads it.
type ListFetcher struct {
	URL    string
	Path   string
	Client *http.Client
}

// Stale reports whether the copy on disk is missing or older than
// AnonymityMaxAge.
func (f *ListFetcher) Stale(now time.Time) bool {
	info, err := os.Stat(f.Path)
	return err != nil || now.Sub(info.ModTime()) > AnonymityMaxAge
}

// LoadLocal reads the copy on disk.
func (f *ListFetcher) LoadLocal() (*IPSet, error) {
	return LoadIPSetFile(f.Path)
}

// Download fetches the list and loads it; see downloadDataset.
func (f *ListFetcher) Download() (*IPSet, error) {
	return downloadDataset(f.Client, f.URL, f.Path, LoadIPSetFile)
}
