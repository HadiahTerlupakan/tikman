package main

import (
	"net/http"
	"net/netip"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// anonymityList is one list and the set loaded from it. A new download
// replaces the set whole, so a lookup never sees half of one.
type anonymityList struct {
	name    string
	fetcher *netflow.ListFetcher
	set     atomic.Pointer[netflow.IPSet]
}

// anonymitySource flags top destination addresses from the Tor exit list and
// X4BNet's VPN and datacenter lists. A list that has not loaded yet flags
// nothing; the others still apply.
type anonymitySource struct {
	tor, vpn, hosting anonymityList
	logger            *zap.Logger
}

func newAnonymitySource(dir string, client *http.Client, logger *zap.Logger) *anonymitySource {
	list := func(name, url, file string) anonymityList {
		return anonymityList{name: name, fetcher: &netflow.ListFetcher{URL: url, Path: filepath.Join(dir, file), Client: client}}
	}
	return &anonymitySource{
		tor:     list(netflow.AnonymityTor, netflow.TorExitListURL, "tor-exits.txt"),
		vpn:     list(netflow.AnonymityVPN, netflow.VPNListURL, "x4bnet-vpn-ipv4.txt"),
		hosting: list(netflow.AnonymityHosting, netflow.DatacenterListURL, "x4bnet-datacenter-ipv4.txt"),
		logger:  logger,
	}
}

func (s *anonymitySource) lists() []*anonymityList {
	return []*anonymityList{&s.tor, &s.vpn, &s.hosting}
}

func (s *anonymitySource) classify(ip netip.Addr) string {
	return netflow.AnonymityLists{Tor: s.tor.set.Load(), VPN: s.vpn.set.Load(), Hosting: s.hosting.set.Load()}.Classify(ip)
}

// preload installs the copies on disk, however old, before collection starts.
func (s *anonymitySource) preload() {
	for _, l := range s.lists() {
		if set, err := l.fetcher.LoadLocal(); err == nil {
			l.set.Store(set)
		}
	}
}

// refresh downloads each list that is missing or AnonymityMaxAge old. A
// failed download keeps the set in use and is tried again on the next call.
func (s *anonymitySource) refresh(now time.Time) {
	for _, l := range s.lists() {
		if l.set.Load() != nil && !l.fetcher.Stale(now) {
			continue
		}
		set, err := l.fetcher.Download()
		if err != nil {
			s.logger.Warn("Anonymity list download failed; keeping the one in use",
				zap.String("list", l.name), zap.Error(err))
			continue
		}
		l.set.Store(set)
		s.logger.Info("Anonymity list loaded", zap.String("list", l.name))
	}
}
