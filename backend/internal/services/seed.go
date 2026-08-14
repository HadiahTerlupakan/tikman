package services

import (
	"fmt"

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
	_, err := service.Create("admin", "admin@tikman.local", "admin123", models.UserRoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	logger.Info("Default admin user created",
		zap.String("username", "admin"),
		zap.String("password", "admin123"),
	)

	return nil
}
