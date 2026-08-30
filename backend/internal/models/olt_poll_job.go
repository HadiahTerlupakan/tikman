package models

import (
	"time"

	"github.com/google/uuid"
)

// PollKind is one tier of polling work for an OLT.
type PollKind string

const (
	// PollKindStatus reads the phase state table only. It is the cheapest read
	// and the one that has to be fresh, because it is what says a subscriber is
	// down.
	PollKindStatus PollKind = "status"
	// PollKindMetrics reads optical power and traffic counters. Optical power
	// does not move minute to minute, so this runs far less often than status.
	PollKindMetrics PollKind = "metrics"
	// PollKindDiscovery walks the inventory to find ONUs added or removed.
	// Measured on this installation it costs as much as the poll it used to run
	// alongside, to learn something that changes a few times a day.
	PollKindDiscovery PollKind = "discovery"
)

// OLTPollJob is one OLT's schedule for one tier of work, and the row a worker
// claims to take that work.
//
// The tags must match migrations/32_add_olt_poll_jobs.sql. AutoMigrate runs
// before the SQL migrations, so a model that disagrees with its migration wins
// and the migration silently does nothing — which is how an index on this
// codebase once ended up built on the wrong columns.
type OLTPollJob struct {
	OLTID uuid.UUID `gorm:"type:uuid;not null;primaryKey" json:"olt_id"`
	Kind  PollKind  `gorm:"type:varchar(16);not null;primaryKey" json:"kind"`

	DueAt    time.Time  `gorm:"not null;index" json:"due_at"`
	LockedBy *string    `gorm:"type:varchar(64)" json:"locked_by"`
	LockedAt *time.Time `json:"locked_at"`

	LastRunAt           *time.Time `json:"last_run_at"`
	LastDurationMs      *int64     `json:"last_duration_ms"`
	LastError           *string    `json:"last_error"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
}

// TableName returns the table name for OLTPollJob.
func (OLTPollJob) TableName() string {
	return "olt_poll_jobs"
}
