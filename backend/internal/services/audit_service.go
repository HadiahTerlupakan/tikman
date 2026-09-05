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

// marshalAuditValue encodes one side of a change. A value that will not encode
// costs the entry that half rather than the whole entry: an audit log missing a
// diff is still evidence that the action happened.
func (s *AuditService) marshalAuditValue(side string, value map[string]interface{}) datatypes.JSON {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		s.logger.Error("Failed to marshal audit value", zap.String("side", side), zap.Error(err))
		return nil
	}
	return encoded
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
	auditLog := &models.AuditLog{
		UserID:       &userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   &resourceID,
		OldValue:     s.marshalAuditValue("old", oldValue),
		NewValue:     s.marshalAuditValue("new", newValue),
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
