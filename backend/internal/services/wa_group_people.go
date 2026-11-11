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

// MemberJIDsOtherThanUs answers the address of everyone a message to the
// group reaches, the list a tag of the whole group names. TikMan's other
// numbers in the group are left out as well as the sending one: is_self marks
// only the number whose mirror this is, and tagging our own second phone tells
// no customer anything.
func (s *WAGroupService) MemberJIDsOtherThanUs(groupID uuid.UUID) ([]string, error) {
	var members []models.WAGroupParticipant
	err := s.db.Select("jid", "lid", "phone").Where("group_id = ? AND NOT is_self", groupID).
		Order("jid").Find(&members).Error
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	ours, err := s.ourNumbers()
	if err != nil {
		return nil, err
	}
	jids := make([]string, 0, len(members))
	for _, member := range members {
		if !ours[member.Phone] && !ours[numberUser(member.JID)] && !ours[numberUser(member.LID)] {
			jids = append(jids, member.JID)
		}
	}
	return uniqueStrings(jids), nil
}

// ourNumbers answers the user part of every address TikMan's numbers go by.
// An account stores only its phone number, and in a LID-addressed group
// another of our numbers often shows by LID alone, with no phone behind it.
// Each number knows its own LID, though: the mirror it keeps marks its own
// member row is_self, so those rows name every LID we hold.
func (s *WAGroupService) ourNumbers() (map[string]bool, error) {
	var addresses []string
	if err := s.db.Model(&models.WAAccount{}).Where("jid <> ''").Pluck("jid", &addresses).Error; err != nil {
		return nil, fmt.Errorf("list WhatsApp numbers: %w", err)
	}
	var selves []models.WAGroupParticipant
	if err := s.db.Distinct("jid", "lid", "phone").Where("is_self").Find(&selves).Error; err != nil {
		return nil, fmt.Errorf("list our group addresses: %w", err)
	}
	for _, self := range selves {
		addresses = append(addresses, self.JID, self.LID, self.Phone)
	}
	ours := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		if user := numberUser(address); user != "" {
			ours[user] = true
		}
	}
	return ours, nil
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
