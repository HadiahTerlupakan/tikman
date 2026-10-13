package main

import (
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// geoSource holds the country table top-IP rows are located with. A new
// dataset replaces it whole, so a lookup never sees half of one.
type geoSource struct {
	table   atomic.Pointer[netflow.GeoTable]
	fetcher *netflow.GeoFetcher
	logger  *zap.Logger
}

// lookup locates ip; until a dataset loads every address is "".
func (s *geoSource) lookup(ip netip.Addr) string {
	table := s.table.Load()
	if table == nil {
		return ""
	}
	return table.Lookup(ip)
}

// nextRefresh retries on the mapping's interval while no table is loaded, as
// asnSource does, rather than leaving a week of rows unlocated.
func (s *geoSource) nextRefresh() time.Duration {
	if s.table.Load() == nil {
		return asnRetryInterval
	}
	return netflow.GeoMaxAge
}

// start loads the copy on disk, downloading first when it is missing or a
// day old.
func (s *geoSource) start(now time.Time) {
	if !s.fetcher.Stale(now) && (s.table.Load() != nil || s.installLocal()) {
		return
	}
	s.refresh()
}

// refresh downloads a new dataset. On failure the table in use stays; with
// none in use yet, a stale copy on disk is better than none.
func (s *geoSource) refresh() {
	table, err := s.fetcher.Download()
	if err != nil {
		s.logger.Warn("Country dataset download failed; keeping the table in use", zap.Error(err))
		if s.table.Load() == nil {
			s.installLocal()
		}
		return
	}
	s.table.Store(table)
	s.logger.Info("Country dataset loaded")
}

func (s *geoSource) installLocal() bool {
	table, err := s.fetcher.LoadLocal()
	if err != nil {
		s.logger.Warn("No usable country dataset on disk", zap.Error(err))
		return false
	}
	s.table.Store(table)
	s.logger.Info("Country dataset loaded from disk")
	return true
}
