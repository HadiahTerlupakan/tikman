package main

import (
	"context"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/tikman/olt-provisioning/internal/netflow"
	"go.uber.org/zap"
)

// asnSaveTimeout covers upserting every name in the dataset, hundreds of
// thousands of rows in batches, on a VPS whose database is also taking the
// minute writes. It is not flowWriteTimeout: a deadline sized for one minute's
// rows would cancel this upsert part-way on every start. It runs off the
// collection path, so a slow or failed save delays only the names.
const asnSaveTimeout = 10 * time.Minute

// asnRetryInterval is how soon a download is tried again while no table is
// loaded.
const asnRetryInterval = mappingRefresh

type asnNameSaver interface {
	SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, updatedAt time.Time) error
}

// asnSource holds the table lookups read. A new dataset replaces it whole, so
// a lookup never sees half of one.
type asnSource struct {
	table   atomic.Pointer[netflow.ASNTable]
	fetcher *netflow.ASNFetcher
	names   asnNameSaver
	logger  *zap.Logger
}

// lookup names the AS of ip; until a dataset loads every address is OtherASN.
func (s *asnSource) lookup(ip netip.Addr) uint32 {
	table := s.table.Load()
	if table == nil {
		return netflow.OtherASN
	}
	return table.Lookup(ip)
}

// nextRefresh is how long until the next download. With no table at all every
// destination is stored as ASN 0, and the daily rollup keeps those rows
// forever, so a first start without network or a copy on disk retries on the
// mapping's interval rather than waiting a day.
func (s *asnSource) nextRefresh() time.Duration {
	if s.table.Load() == nil {
		return asnRetryInterval
	}
	return netflow.ASNMaxAge
}

// preload installs the copy on disk, however old, before collection starts.
// Without it every flow until start finishes, which downloads and saves names
// for seconds to minutes, is counted as ASN 0. It saves no names: start does.
func (s *asnSource) preload() {
	table, _, err := s.fetcher.LoadLocal()
	if err != nil {
		s.logger.Warn("No ASN dataset on disk to start with", zap.Error(err))
		return
	}
	s.table.Store(table)
}

// start loads the copy on disk, downloading first when it is missing or a
// day old.
func (s *asnSource) start(now time.Time) {
	if !s.fetcher.Stale(now) && s.installLocal() {
		return
	}
	s.refresh()
}

// refresh downloads a new dataset. On failure the table in use stays; with
// none in use yet, a stale copy on disk is better than none.
func (s *asnSource) refresh() {
	table, at, err := s.fetcher.Download()
	if err != nil {
		s.logger.Warn("ASN dataset download failed; keeping the table in use", zap.Error(err))
		if s.table.Load() == nil {
			s.installLocal()
		}
		return
	}
	s.install(table, at)
}

func (s *asnSource) installLocal() bool {
	table, at, err := s.fetcher.LoadLocal()
	if err != nil {
		s.logger.Warn("No usable ASN dataset on disk", zap.Error(err))
		return false
	}
	s.install(table, at)
	return true
}

func (s *asnSource) install(table *netflow.ASNTable, at time.Time) {
	s.table.Store(table)
	ctx, cancel := context.WithTimeout(context.Background(), asnSaveTimeout)
	defer cancel()
	if err := s.names.SaveASNNames(ctx, table.Names(), at); err != nil {
		s.logger.Error("Failed to save ASN names", zap.Error(err))
	}
	s.logger.Info("ASN dataset loaded", zap.Int("asns", len(table.Names())), zap.Time("downloaded_at", at))
}
