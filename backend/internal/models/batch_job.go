package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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

// BatchJob tracks a multi-ONT provisioning run. ont_ids is a postgres UUID[]
// for efficient array containment queries; ont_results is a jsonb map of
// ont_id -> per-ONT result, updated as the batch progresses so a long run
// stays observable.
type BatchJob struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	TemplateID  uuid.UUID      `gorm:"type:uuid;not null" json:"template_id"`
	ONTIDs      []uuid.UUID    `gorm:"type:uuid[]" json:"ont_ids"`
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
