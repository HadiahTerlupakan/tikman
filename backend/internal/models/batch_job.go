package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UUIDSlice wraps []uuid.UUID with Value()/Scan() for DB compatibility.
// Production Postgres stores as uuid[]; SQLite tests store as jsonb.
type UUIDSlice []uuid.UUID

func (s UUIDSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *UUIDSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return fmt.Errorf("cannot scan %T into UUIDSlice", value)
	}
}

// BatchJob status values:
// pending -> running -> success | failed | partial_rollback.
// partial_rollback means some ONTs were provisioned and then reverted because
// a later ONT in the batch failed.
const (
	BatchStatusPending         = "pending"
	BatchStatusRunning         = "running"
	BatchStatusSuccess         = "success"
	BatchStatusFailed          = "failed"
	BatchStatusPartialRollback = "partial_rollback"
)

// BatchJob tracks a multi-ONT provisioning run. ont_ids is a jsonb array of
// UUIDs (jsonb also supports containment queries and stays compatible with the
// SQLite driver used in tests); ont_results is a jsonb map of ont_id -> per-ONT
// result, updated as the batch progresses so a long run stays observable.
// The migration in migrations/09_create_provisioning_tables.sql declares
// ont_ids as UUID[] — the column type should be reconciled to jsonb.
type BatchJob struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TemplateID  uuid.UUID      `gorm:"type:uuid;not null" json:"template_id"`
	ONTIDs      datatypes.JSON `gorm:"type:jsonb" json:"ont_ids"`
	Status      string         `gorm:"type:varchar(20);not null;default:pending;index:idx_batch_jobs_status" json:"status"`
	ONTResults  datatypes.JSON `gorm:"type:jsonb" json:"ont_results,omitempty"`
	CreatedBy   *uuid.UUID     `gorm:"type:uuid" json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

func (b *BatchJob) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (BatchJob) TableName() string {
	return "batch_jobs"
}
