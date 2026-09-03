package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushService owns the device tokens push notifications are sent to.
type PushService struct {
	db *gorm.DB
}

func NewPushService(db *gorm.DB) *PushService {
	return &PushService{db: db}
}

// Subscribe registers a device. Re-registering the same token — the normal
// case on every app load once permission is already granted — updates the
// owner instead of creating a duplicate row, so a token that outlives a
// logout on a shared machine follows whoever is using it now.
func (s *PushService) Subscribe(userID uuid.UUID, token string) error {
	sub := models.PushSubscription{UserID: userID, FCMToken: token}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fcm_token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "updated_at"}),
	}).Create(&sub).Error
}

// Unsubscribe removes a device, scoped to the caller's own userID so one CS
// cannot silently unregister another's.
func (s *PushService) Unsubscribe(userID uuid.UUID, token string) error {
	return s.db.Where("user_id = ? AND fcm_token = ?", userID, token).
		Delete(&models.PushSubscription{}).Error
}

// TokensForRoles returns every distinct device token belonging to a user
// holding one of the given roles.
func (s *PushService) TokensForRoles(roles ...models.UserRole) ([]string, error) {
	var tokens []string
	err := s.db.Model(&models.PushSubscription{}).
		Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role IN ?", roles).
		Distinct().
		Pluck("push_subscriptions.fcm_token", &tokens).Error
	return tokens, err
}

// RemoveTokens deletes every row naming one of these tokens — used after a
// send reports them as no longer registered, so the table does not
// accumulate dead devices forever.
func (s *PushService) RemoveTokens(tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return s.db.Where("fcm_token IN ?", tokens).Delete(&models.PushSubscription{}).Error
}
