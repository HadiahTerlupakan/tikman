package models

import "time"

// WorkerHeartbeatPoller names the row the polling worker stamps. Each worker
// gets its own row so a second one can be added without changing the schema.
const WorkerHeartbeatPoller = "poller"

// WorkerHeartbeat records that a worker finished a full cycle. The API and the
// worker are separate processes that share only this database, so without a
// row here the API has no way to tell a working poller from a dead one — and a
// dead poller leaves every ONT status frozen at its last known value while the
// rest of the system still reports itself healthy.
type WorkerHeartbeat struct {
	Name   string    `gorm:"primaryKey;size:64" json:"name"`
	BeatAt time.Time `gorm:"column:beat_at;not null" json:"beat_at"`
}

// TableName returns the table name for WorkerHeartbeat.
func (WorkerHeartbeat) TableName() string {
	return "worker_heartbeats"
}
