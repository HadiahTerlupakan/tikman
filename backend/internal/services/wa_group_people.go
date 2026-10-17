package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// NotePushName records the profile name a member's phone attached to a
// message, under whichever of their addresses the member list knows. It only
// ever fills in or updates a name; an empty one tells nothing.
func (s *WAGroupService) NotePushName(groupID uuid.UUID, addresses []string, pushName string) error {
	if pushName == "" || len(addresses) == 0 {
		return nil
	}
	err := s.db.Model(&models.WAGroupParticipant{}).
		Where("group_id = ? AND (jid IN ? OR lid IN ?) AND push_name <> ?", groupID, addresses, addresses, pushName).
		Update("push_name", pushName).Error
	if err != nil {
		return fmt.Errorf("note push name: %w", err)
	}
	return nil
}

// SetPinned pins a group to the top of the list or lets it go.
func (s *WAGroupService) SetPinned(id uuid.UUID, pinned bool) (*models.WAGroup, error) {
	var at *time.Time
	if pinned {
		now := time.Now()
		at = &now
	}
	res := s.db.Model(&models.WAGroup{}).Where("id = ?", id).Update("pinned_at", at)
	if res.Error != nil {
		return nil, fmt.Errorf("pin group: %w", res.Error)
	}
	return s.Get(id)
}

// FindByJID answers a group the number is in, nil when it has none by that JID.
func (s *WAGroupService) FindByJID(accountID uuid.UUID, jid string) (*models.WAGroup, error) {
	var groups []models.WAGroup
	if err := s.db.Where("wa_account_id = ? AND jid = ?", accountID, jid).Limit(1).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("find group: %w", err)
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return &groups[0], nil
}

// MembersOtherThanUs counts who a message to the group reaches.
func (s *WAGroupService) MembersOtherThanUs(groupID uuid.UUID) (int, error) {
	var count int64
	err := s.db.Model(&models.WAGroupParticipant{}).Where("group_id = ? AND NOT is_self", groupID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count group members: %w", err)
	}
	return int(count), nil
}

func (s *WAGroupService) markGroupAvatars(groups []models.WAGroup) error {
	if len(groups) == 0 {
		return nil
	}
	jids := make([]string, len(groups))
	for i, g := range groups {
		jids[i] = g.JID
	}
	shown, err := avatarsShown(s.db, jids)
	if err != nil {
		return err
	}
	for i := range groups {
		_, groups[i].HasAvatar = shown[avatarKey{groups[i].WAAccountID, groups[i].JID}]
	}
	return nil
}

func (s *WAGroupService) markMemberAvatars(groupID uuid.UUID, members []models.WAGroupParticipant) error {
	if len(members) == 0 {
		return nil
	}
	group, err := s.groupRow(groupID)
	if err != nil {
		return err
	}
	jids := make([]string, len(members))
	for i, m := range members {
		jids[i] = m.JID
	}
	shown, err := avatarsShown(s.db, jids)
	if err != nil {
		return err
	}
	for i := range members {
		_, members[i].HasAvatar = shown[avatarKey{group.WAAccountID, members[i].JID}]
	}
	return nil
}

func (s *WAGroupService) groupRow(id uuid.UUID) (*models.WAGroup, error) {
	var group models.WAGroup
	if err := s.db.First(&group, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
