package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UUIDSlice is []uuid.UUID mapped to a native uuid[] column. uuid[] is used
// over jsonb for native pgx support and direct queryability (e.g. containment
// queries like ont_ids @> '{...}'). Value/Scan use the Postgres array literal
// text format so the same type also works with the SQLite driver used in
// tests, which has no array support and stores the value as text.
type UUIDSlice []uuid.UUID

func (s UUIDSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	parts := make([]string, len(s))
	for i, id := range s {
		parts[i] = `"` + id.String() + `"`
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (s *UUIDSlice) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	var raw string
	switch v := value.(type) {
	case []byte:
		raw = string(v)
	case string:
		raw = v
	default:
		return fmt.Errorf("cannot scan %T into UUIDSlice", value)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		*s = UUIDSlice{}
		return nil
	}
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return fmt.Errorf("cannot parse %q as uuid[] array literal", raw)
	}
	elements := strings.Split(raw[1:len(raw)-1], ",")
	result := make(UUIDSlice, 0, len(elements))
	for _, e := range elements {
		e = strings.Trim(strings.TrimSpace(e), `"`)
		id, err := uuid.Parse(e)
		if err != nil {
			return fmt.Errorf("invalid uuid %q in array literal: %w", e, err)
		}
		result = append(result, id)
	}
	*s = result
	return nil
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

// BatchJob tracks a multi-ONT provisioning run. ont_ids is a native uuid[]
// array (native support, queryable via containment operators); ont_results is
// a jsonb map of ont_id -> per-ONT result, updated as the batch progresses so
// a long run stays observable.
type BatchJob struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TemplateID  uuid.UUID      `gorm:"type:uuid;not null" json:"template_id"`
	ONTIDs      UUIDSlice      `gorm:"type:uuid[]" json:"ont_ids"`
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
