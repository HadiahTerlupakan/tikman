package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// flowWriteBatch is rows per INSERT. A busy minute is a few thousand rows;
// batches keep each statement well under Postgres's parameter limit.
const flowWriteBatch = 1000

// FlowStore writes what flowd aggregated.
type FlowStore struct {
	db *gorm.DB
}

// NewFlowStore creates a flow store.
func NewFlowStore(db *gorm.DB) *FlowStore {
	return &FlowStore{db: db}
}

// nullablePOP stores the Lainnya POP, uuid.Nil in memory, as NULL.
func nullablePOP(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// WriteMinute stores one closed minute. CreateInBatches runs in one
// transaction, so a minute is stored whole or not at all.
func (s *FlowStore) WriteMinute(ctx context.Context, rows []netflow.MinuteRow) error {
	if len(rows) == 0 {
		return nil
	}
	records := make([]models.RouterFlowMinute, len(rows))
	for i, r := range rows {
		records[i] = models.RouterFlowMinute{Time: r.Time, RouterID: r.RouterID, POPInterfaceID: nullablePOP(r.POP),
			UpstreamInterfaceID: r.Upstream, Direction: string(r.Direction), ASN: int64(r.ASN),
			Bytes: int64(r.Bytes), Packets: int64(r.Packets)}
	}
	return s.db.WithContext(ctx).CreateInBatches(records, flowWriteBatch).Error
}

// WriteTopIPs stores one closed window's top addresses.
func (s *FlowStore) WriteTopIPs(ctx context.Context, rows []netflow.TopIPRow) error {
	if len(rows) == 0 {
		return nil
	}
	records := make([]models.RouterFlowTopIP, len(rows))
	for i, r := range rows {
		records[i] = models.RouterFlowTopIP{Time: r.Time, RouterID: r.RouterID, POPInterfaceID: nullablePOP(r.POP),
			Direction: string(r.Direction), IP: r.IP.String(), ASN: int64(r.ASN), Bytes: int64(r.Bytes)}
	}
	return s.db.WithContext(ctx).CreateInBatches(records, flowWriteBatch).Error
}

// MarkSeen records when each router's IPFIX last arrived. UpdateColumn leaves
// updated_at alone: this runs every minute and is not an edit of the router.
func (s *FlowStore) MarkSeen(ctx context.Context, routerIDs []uuid.UUID, at time.Time) error {
	if len(routerIDs) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&models.MikrotikRouter{}).Where("id IN ?", routerIDs).
		UpdateColumn("flow_last_seen_at", at).Error
}

// SaveASNNames upserts the dataset's names. updatedAt is the dataset's
// download time; the newest one is the version the Router tab shows.
func (s *FlowStore) SaveASNNames(ctx context.Context, names map[uint32]netflow.ASNInfo, updatedAt time.Time) error {
	if len(names) == 0 {
		return nil
	}
	rows := make([]models.ASNName, 0, len(names))
	for asn, info := range names {
		rows = append(rows, models.ASNName{ASN: int64(asn), Name: info.Name, Country: info.Country, UpdatedAt: updatedAt})
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "asn"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "country", "updated_at"}),
	}).CreateInBatches(rows, flowWriteBatch).Error
}
