package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Site struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Location    string    `gorm:"type:text"`
	Description string    `gorm:"type:text"`
	Latitude    *float64  `gorm:"type:double precision" json:"latitude,omitempty"`
	Longitude   *float64  `gorm:"type:double precision" json:"longitude,omitempty"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (s *Site) TableName() string {
	return "sites"
}
