package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ProvisioningJob status values form a state machine:
// pending -> running -> success | failed, and failed -> rolled_back once the
// before_snapshot has been restored.
const (
	ProvisioningStatusPending    = "pending"
	ProvisioningStatusRunning    = "running"
	ProvisioningStatusSuccess    = "success"
	ProvisioningStatusFailed     = "failed"
	ProvisioningStatusRolledBack = "rolled_back"
)

// ProvisioningJob tracks a single ONT provisioning run. BeforeSnapshot is
// captured before any change so a failed job can be rolled back to the exact
// prior config. ConfigSnapshot records what was actually sent for auditing.
//
// The partial unique index uq_provisioning_jobs_running_ont (only one
// status='running' row per ont_id) lives in
// migrations/09_create_provisioning_tables.sql — GORM cannot express partial
// indexes, and two concurrent writers on the same ONT would corrupt its config.
type ProvisioningJob struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	ONTID          uuid.UUID      `gorm:"type:uuid;not null;index:idx_provisioning_jobs_ont_status,priority:1" json:"ont_id"`
	ONUID          int            `gorm:"-" json:"onu_id,omitempty"`
	TemplateID     *uuid.UUID     `gorm:"type:uuid" json:"template_id,omitempty"`
	Status         string         `gorm:"type:varchar(20);not null;default:pending;index:idx_provisioning_jobs_ont_status,priority:2" json:"status"`
	ConfigSnapshot datatypes.JSON `gorm:"type:jsonb" json:"config_snapshot,omitempty"`
	BeforeSnapshot datatypes.JSON `gorm:"type:jsonb" json:"before_snapshot,omitempty"`
	ErrorMessage   *string        `gorm:"type:text" json:"error_message,omitempty"`
	CreatedBy      *uuid.UUID     `gorm:"type:uuid" json:"created_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

func (p *ProvisioningJob) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (ProvisioningJob) TableName() string {
	return "provisioning_jobs"
}
