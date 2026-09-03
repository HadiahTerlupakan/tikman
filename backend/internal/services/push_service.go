package services

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PushService owns the Firebase Installation IDs push notifications are sent to.
type PushService struct {
	db *gorm.DB
}

func NewPushService(db *gorm.DB) *PushService {
	return &PushService{db: db}
}

// Subscribe registers a device. Re-registering the same FID — the normal case
// on every app load once permission is already granted — updates the owner
// instead of creating a duplicate row, so an installation that outlives a
// logout on a shared machine follows whoever is using it now.
func (s *PushService) Subscribe(userID uuid.UUID, fid string) error {
	sub := models.PushSubscription{UserID: userID, FID: fid}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fid"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "updated_at"}),
	}).Create(&sub).Error
}

// Unsubscribe removes a device, scoped to the caller's own userID so one CS
// cannot silently unregister another's.
func (s *PushService) Unsubscribe(userID uuid.UUID, fid string) error {
	return s.db.Where("user_id = ? AND fid = ?", userID, fid).
		Delete(&models.PushSubscription{}).Error
}

// FIDsForRoles returns every distinct installation ID belonging to a user
// holding one of the given roles.
func (s *PushService) FIDsForRoles(roles ...models.UserRole) ([]string, error) {
	var fids []string
	err := s.db.Model(&models.PushSubscription{}).
		Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role IN ?", roles).
		Distinct().
		Pluck("push_subscriptions.fid", &fids).Error
	return fids, err
}

// RemoveFIDs deletes every row naming one of these installation IDs — used
// after a send reports them as no longer registered, so the table does not
// accumulate dead devices forever.
func (s *PushService) RemoveFIDs(fids []string) error {
	if len(fids) == 0 {
		return nil
	}
	return s.db.Where("fid IN ?", fids).Delete(&models.PushSubscription{}).Error
}
