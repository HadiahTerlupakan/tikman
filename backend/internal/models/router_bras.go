package models

import (
	"time"

	"github.com/google/uuid"
)

// RouterBRASSample is one minute of a BRAS: what the page charts and judges a
// drop by. The RADIUS columns are the cumulative counters as read, summed over
// servers, so a period's count is the difference between two samples.
// PoolMinFree is the fewest free addresses in any pool whose total is known,
// nil when no pool's is.
type RouterBRASSample struct {
	RouterID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"router_id"`
	Time           time.Time `gorm:"primaryKey" json:"time"`
	Sessions       int       `gorm:"not null" json:"sessions"`
	CPUMax         int16     `gorm:"not null" json:"cpu_max"`
	CPUAvg         int16     `gorm:"not null" json:"cpu_avg"`
	RadiusAccepts  int64     `gorm:"not null" json:"radius_accepts"`
	RadiusRejects  int64     `gorm:"not null" json:"radius_rejects"`
	RadiusTimeouts int64     `gorm:"not null" json:"radius_timeouts"`
	PoolMinFree    *int      `json:"pool_min_free"`
}

// TableName returns the table name for RouterBRASSample.
func (RouterBRASSample) TableName() string { return "router_bras_samples" }
