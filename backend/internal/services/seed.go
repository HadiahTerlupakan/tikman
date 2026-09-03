package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func CreateDefaultAdmin(db *gorm.DB, logger *zap.Logger) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		logger.Info("Users already exist, skipping default admin creation")
		return nil
	}

	service := NewUserService(db)
	_, err := service.Create("admin", "admin@tikman.local", "admin123", "", models.UserRoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	logger.Info("Default admin user created",
		zap.String("username", "admin"),
		zap.String("password", "admin123"),
	)

	return nil
}

// SeedService handles database seeding operations.
type SeedService struct {
	db *gorm.DB
}

// NewSeedService creates a new SeedService.
func NewSeedService(db *gorm.DB) *SeedService {
	return &SeedService{db: db}
}

// SeedEventsForONT creates event records for an ONT based on discovered state.
// DB writes belong in services, not handlers. Errors are counted per event so
// a failed offline insert does not skip the online insert, matching the
// handler's original behavior.
func (s *SeedService) SeedEventsForONT(ontID uuid.UUID, lastOfflineReason string, lastOnlineTime, lastOfflineTime time.Time) (created, errs int) {
	if !lastOfflineTime.IsZero() {
		event := &models.ONTEvent{
			ONTID:     ontID,
			EventType: models.EventTypeOffline,
			EventTime: lastOfflineTime,
			Reason:    lastOfflineReason,
		}
		if err := s.db.Create(event).Error; err != nil {
			errs++
		} else {
			created++
		}
	}

	if !lastOnlineTime.IsZero() {
		var durationSeconds *int64
		if !lastOfflineTime.IsZero() && lastOnlineTime.After(lastOfflineTime) {
			duration := int64(lastOnlineTime.Sub(lastOfflineTime).Seconds())
			durationSeconds = &duration
		}

		event := &models.ONTEvent{
			ONTID:           ontID,
			EventType:       models.EventTypeOnline,
			EventTime:       lastOnlineTime,
			Reason:          "System startup",
			DurationSeconds: durationSeconds,
		}
		if err := s.db.Create(event).Error; err != nil {
			errs++
		} else {
			created++
		}
	}

	return created, errs
}
