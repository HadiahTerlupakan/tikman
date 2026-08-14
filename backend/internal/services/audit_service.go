package services

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AuditService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewAuditService(db *gorm.DB, logger *zap.Logger) *AuditService {
	return &AuditService{
		db:     db,
		logger: logger,
	}
}

func (s *AuditService) Log(
	userID uuid.UUID,
	action string,
	resourceType string,
	resourceID uuid.UUID,
	oldValue map[string]interface{},
	newValue map[string]interface{},
	ipAddress string,
	userAgent string,
) error {
	var oldJSON, newJSON datatypes.JSON
	var err error

	if oldValue != nil {
		oldJSON, err = json.Marshal(oldValue)
		if err != nil {
			s.logger.Error("Failed to marshal old value", zap.Error(err))
			oldJSON = nil
		}
	}

	if newValue != nil {
		newJSON, err = json.Marshal(newValue)
		if err != nil {
			s.logger.Error("Failed to marshal new value", zap.Error(err))
			newJSON = nil
		}
	}

	auditLog := &models.AuditLog{
		UserID:       &userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   &resourceID,
		OldValue:     oldJSON,
		NewValue:     newJSON,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	if err := s.db.Create(auditLog).Error; err != nil {
		s.logger.Error("Failed to create audit log",
			zap.Error(err),
			zap.String("action", action),
			zap.String("resource_type", resourceType),
		)
		return err
	}

	return nil
}
