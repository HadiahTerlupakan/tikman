package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// defaultGroupListLimit is more groups than one number is likely to be in;
// the list is not paged because the left pane draws it whole.
const defaultGroupListLimit = 500

// GroupSnapshot is one group as WhatsApp last described it.
type GroupSnapshot struct {
	JID          string
	Name         string
	Topic        string
	TopicID      string
	OwnerJID     string
	IsAdmin      bool
	Announce     bool
	Locked       bool
	Participants []models.WAGroupParticipant
}

// GroupFilter narrows the group list.
type GroupFilter struct {
	AccountID *uuid.UUID
	Search    string
}

// WAGroupService keeps the mirror of the groups each number is in.
type WAGroupService struct {
	db *gorm.DB
}

// NewWAGroupService constructs a WAGroupService.
func NewWAGroupService(db *gorm.DB) *WAGroupService {
	return &WAGroupService{db: db}
}

// SyncAll records the full list of groups one number is in. A group this
// number used to be in and no longer is gets marked left rather than deleted,
// so its history stays readable.
func (s *WAGroupService) SyncAll(accountID uuid.UUID, groups []GroupSnapshot) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		present := make([]string, 0, len(groups))
		for _, snap := range groups {
			if _, err := upsertGroup(tx, accountID, snap); err != nil {
				return err
			}
			present = append(present, snap.JID)
		}

		gone := tx.Model(&models.WAGroup{}).Where("wa_account_id = ? AND NOT has_left", accountID)
		if len(present) > 0 {
			gone = gone.Where("jid NOT IN ?", present)
		}
		if err := gone.Update("has_left", true).Error; err != nil {
			return fmt.Errorf("mark groups left: %w", err)
		}
		return nil
	})
}

// Upsert records one group, as after a change WhatsApp announced.
func (s *WAGroupService) Upsert(accountID uuid.UUID, snap GroupSnapshot) (*models.WAGroup, error) {
	var group *models.WAGroup
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		group, err = upsertGroup(tx, accountID, snap)
		return err
	})
	return group, err
}

// upsertGroup writes a group's metadata and replaces its member list. The
// summary columns are left alone: they belong to the messages, not the sync.
func upsertGroup(tx *gorm.DB, accountID uuid.UUID, snap GroupSnapshot) (*models.WAGroup, error) {
	now := time.Now()
	group := models.WAGroup{
		WAAccountID:      accountID,
		JID:              snap.JID,
		Name:             snap.Name,
		Topic:            snap.Topic,
		TopicID:          snap.TopicID,
		OwnerJID:         snap.OwnerJID,
		ParticipantCount: len(snap.Participants),
		IsAdmin:          snap.IsAdmin,
		Announce:         snap.Announce,
		Locked:           snap.Locked,
		SyncedAt:         &now,
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "wa_account_id"}, {Name: "jid"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name": snap.Name, "topic": snap.Topic, "topic_id": snap.TopicID,
			"owner_jid": snap.OwnerJID, "participant_count": len(snap.Participants),
			"is_admin": snap.IsAdmin, "announce": snap.Announce, "locked": snap.Locked,
			"has_left": false, "synced_at": now, "updated_at": now,
		}),
	}).Create(&group).Error
	if err != nil {
		return nil, fmt.Errorf("store group %s: %w", snap.JID, err)
	}

	stored, err := groupByJID(tx, accountID, snap.JID)
	if err != nil {
		return nil, err
	}
	if err := replaceParticipants(tx, stored.ID, snap.Participants); err != nil {
		return nil, err
	}
	return stored, nil
}

// groupByJID reads a group back after an upsert. Into a fresh struct, not the
// one just inserted: on a conflict that one carries an id that was never
// stored, and First would add it to the condition and find nothing.
func groupByJID(db *gorm.DB, accountID uuid.UUID, jid string) (*models.WAGroup, error) {
	var group models.WAGroup
	if err := db.Where("wa_account_id = ? AND jid = ?", accountID, jid).First(&group).Error; err != nil {
		return nil, fmt.Errorf("load group %s: %w", jid, err)
	}
	return &group, nil
}

func replaceParticipants(tx *gorm.DB, groupID uuid.UUID, members []models.WAGroupParticipant) error {
	if err := tx.Where("group_id = ?", groupID).Delete(&models.WAGroupParticipant{}).Error; err != nil {
		return fmt.Errorf("clear group members: %w", err)
	}
	if len(members) == 0 {
		return nil
	}
	rows := make([]models.WAGroupParticipant, len(members))
	for i, member := range members {
		member.ID = uuid.Nil
		member.GroupID = groupID
		rows[i] = member
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("store group members: %w", err)
	}
	return nil
}

// FindOrCreate answers the group a message arrived in, creating a bare row
// for one the sync has not seen yet; created says the caller should ask
// WhatsApp what the group is called.
func (s *WAGroupService) FindOrCreate(accountID uuid.UUID, jid string) (*models.WAGroup, bool, error) {
	group := models.WAGroup{WAAccountID: accountID, JID: jid}
	res := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&group)
	if res.Error != nil {
		return nil, false, fmt.Errorf("store group %s: %w", jid, res.Error)
	}
	stored, err := groupByJID(s.db, accountID, jid)
	if err != nil {
		return nil, false, err
	}
	return stored, res.RowsAffected > 0, nil
}

// MarkLeft records that a number is no longer in a group.
func (s *WAGroupService) MarkLeft(accountID uuid.UUID, jid string) error {
	err := s.db.Model(&models.WAGroup{}).Where("wa_account_id = ? AND jid = ?", accountID, jid).
		Update("has_left", true).Error
	if err != nil {
		return fmt.Errorf("mark group left: %w", err)
	}
	return nil
}

// List answers the groups, most recently active first.
func (s *WAGroupService) List(filter GroupFilter) ([]models.WAGroup, error) {
	query := s.db.Model(&models.WAGroup{})
	if filter.AccountID != nil {
		query = query.Where("wa_account_id = ?", *filter.AccountID)
	}
	if term := strings.TrimSpace(filter.Search); term != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(term)+"%")
	}

	var groups []models.WAGroup
	err := query.Order("last_message_at IS NULL, last_message_at DESC, name ASC").
		Limit(defaultGroupListLimit).Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return groups, nil
}

// Get answers one group.
func (s *WAGroupService) Get(id uuid.UUID) (*models.WAGroup, error) {
	var group models.WAGroup
	if err := s.db.First(&group, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// Participants answers a group's members, admins first.
func (s *WAGroupService) Participants(groupID uuid.UUID) ([]models.WAGroupParticipant, error) {
	var members []models.WAGroupParticipant
	err := s.db.Where("group_id = ?", groupID).
		Order("CASE role WHEN 'superadmin' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, name, phone").
		Find(&members).Error
	if err != nil {
		return nil, fmt.Errorf("list group members: %w", err)
	}
	return members, nil
}

// MarkRead clears a group's unread count, answering whether there was one.
func (s *WAGroupService) MarkRead(id uuid.UUID) (bool, error) {
	res := s.db.Model(&models.WAGroup{}).Where("id = ? AND unread_count > 0", id).
		Update("unread_count", 0)
	if res.Error != nil {
		return false, fmt.Errorf("clear group unread: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}
