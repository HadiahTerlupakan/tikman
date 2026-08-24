package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Template vendors supported by the provisioning system.
const (
	VendorZTE  = "zte"
	VendorHSGQ = "hsgq"
)

// ConfigTemplate is a reusable ONT configuration pattern. The ConfigFields
// shape is per-vendor (ZTE and HSGQ CLIs differ), so it stays schemaless jsonb
// and is validated at apply time rather than by the database.
type ConfigTemplate struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name         string         `gorm:"type:varchar(255);not null;uniqueIndex:config_templates_name_key" json:"name"`
	Description  string         `gorm:"type:text" json:"description,omitempty"`
	Vendor       string         `gorm:"type:varchar(50);not null;index:idx_config_templates_vendor" json:"vendor"`
	ConfigFields datatypes.JSON `gorm:"type:jsonb" json:"config_fields"`
	IsDefault    bool           `gorm:"default:false" json:"is_default"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

func (c *ConfigTemplate) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (ConfigTemplate) TableName() string {
	return "config_templates"
}
