package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ServiceProfile struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key"`
	OLTID       uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_olt_profile_id"`
	ProfileName string    `gorm:"type:varchar(255);not null"`
	ProfileID   int       `gorm:"not null;uniqueIndex:idx_olt_profile_id"`
	Description string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (sp *ServiceProfile) BeforeCreate(tx *gorm.DB) error {
	if sp.ID == uuid.Nil {
		sp.ID = uuid.New()
	}
	return nil
}

func (sp *ServiceProfile) TableName() string {
	return "service_profiles"
}

type LineProfile struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key"`
	OLTID         uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_olt_line_profile_id"`
	ProfileName   string    `gorm:"type:varchar(255);not null"`
	ProfileID     int       `gorm:"not null;uniqueIndex:idx_olt_line_profile_id"`
	BandwidthDown int       `gorm:"comment:'Bandwidth in Mbps'"`
	BandwidthUp   int       `gorm:"comment:'Bandwidth in Mbps'"`
	VLANID        int
	Description   string    `gorm:"type:text"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (lp *LineProfile) BeforeCreate(tx *gorm.DB) error {
	if lp.ID == uuid.Nil {
		lp.ID = uuid.New()
	}
	return nil
}

func (lp *LineProfile) TableName() string {
	return "line_profiles"
}
