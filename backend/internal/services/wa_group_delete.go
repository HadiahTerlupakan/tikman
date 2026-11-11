package services

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

var (
	// ErrGroupNotLeft refuses to delete a group the number is still in: the
	// next sync would only bring it back, without the history it had.
	ErrGroupNotLeft = errors.New("Grup hanya bisa dihapus setelah nomor ini keluar dari grup")
	// ErrGroupMediaLeftBehind says the group is gone but some of its files
	// would not come off the disk.
	ErrGroupMediaLeftBehind = errors.New("group deleted, media left on disk")
)

// orphanMemberAvatars names the member photos only this group was keeping.
// A face is stored once per number and shared by every group the member is
// in or wrote in, so one still seen elsewhere stays; one seen nowhere else
// would otherwise be fetched from WhatsApp on every sweep for nobody. Former
// members count too: a sync drops their member row but not their photo, and
// the messages they wrote here are the last trace of them.
const orphanMemberAvatars = `SELECT c.jid FROM (
    SELECT jid FROM wa_group_participants WHERE group_id = ?
    UNION SELECT sender_jid FROM wa_group_messages WHERE group_id = ? AND sender_jid <> ''
  ) c
WHERE NOT EXISTS (SELECT 1 FROM wa_group_participants o JOIN wa_groups g ON g.id = o.group_id
                  WHERE g.wa_account_id = ? AND o.group_id <> ? AND o.jid = c.jid)
  AND NOT EXISTS (SELECT 1 FROM wa_group_messages m JOIN wa_groups g ON g.id = m.group_id
                  WHERE g.wa_account_id = ? AND m.group_id <> ? AND m.sender_jid = c.jid)`

// DeleteLeft removes a group the number has left, with everything TikMan kept
// about it: members, messages and what hangs off them, its graph report, and
// the photos and attachments on disk. WhatsApp is not told; there is nothing
// left there for the number to do.
//
// The rows go in one transaction and the files only after it commits, so a
// rollback never leaves rows pointing at files that are gone. A file that
// will not come off the disk is answered as ErrGroupMediaLeftBehind: the
// group is deleted by then and saying otherwise would be wrong.
func (s *WAGroupService) DeleteLeft(id uuid.UUID, mediaRoot string) error {
	var paths []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		group, err := claimLeftGroup(tx, id)
		if err != nil {
			return err
		}
		if paths, err = groupFilePaths(tx, group); err != nil {
			return err
		}
		return deleteGroupRows(tx, group)
	})
	if err != nil {
		return err
	}
	if err := removeGroupFiles(mediaRoot, paths); err != nil {
		return fmt.Errorf("%w: %w", ErrGroupMediaLeftBehind, err)
	}
	return nil
}

// claimLeftGroup locks the group by writing to it before anything is read. A
// sync marking it joined again then either commits first, and this refuses,
// or waits behind the delete; it cannot land between the check and the
// delete and lose a group the number is back in.
func claimLeftGroup(tx *gorm.DB, id uuid.UUID) (*models.WAGroup, error) {
	res := tx.Model(&models.WAGroup{}).Where("id = ? AND has_left", id).Update("updated_at", time.Now())
	if res.Error != nil {
		return nil, fmt.Errorf("claim group: %w", res.Error)
	}
	var group models.WAGroup
	if err := tx.First(&group, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if res.RowsAffected == 0 {
		return nil, ErrGroupNotLeft
	}
	return &group, nil
}

// groupAvatars narrows wa_avatars to the group's own photo and the member
// photos nothing else needs.
func groupAvatars(tx *gorm.DB, group *models.WAGroup) *gorm.DB {
	orphans := tx.Raw(orphanMemberAvatars,
		group.ID, group.ID, group.WAAccountID, group.ID, group.WAAccountID, group.ID)
	return tx.Model(&models.WAAvatar{}).
		Where("wa_account_id = ? AND (jid = ? OR jid IN (?))", group.WAAccountID, group.JID, orphans)
}

// groupFilePaths lists the files the group's rows point at. It runs before the
// rows go, because on Postgres the messages cascade away with the group.
func groupFilePaths(tx *gorm.DB, group *models.WAGroup) ([]string, error) {
	var media, avatars []string
	err := tx.Model(&models.WAGroupMessage{}).
		Where("group_id = ? AND media_path <> ''", group.ID).Pluck("media_path", &media).Error
	if err != nil {
		return nil, fmt.Errorf("list group media: %w", err)
	}
	if err := groupAvatars(tx, group).Where("path <> ''").Pluck("path", &avatars).Error; err != nil {
		return nil, fmt.Errorf("list group avatars: %w", err)
	}
	return append(media, avatars...), nil
}

// deleteGroupRows removes the group and its dependants explicitly, so SQLite,
// which has none of the migrations' foreign keys, ends the same as Postgres.
// Avatars go first because which ones are orphans is read from the members.
//
// The group's management requests stay, with no group, as migration 68's SET
// NULL has it: who asked the number to leave is history worth keeping, and
// the wa process refuses one that names no group rather than acting on it.
func deleteGroupRows(tx *gorm.DB, group *models.WAGroup) error {
	if err := groupAvatars(tx, group).Delete(&models.WAAvatar{}).Error; err != nil {
		return fmt.Errorf("delete group avatars: %w", err)
	}
	if err := detachTelegramReports(tx, group.ID); err != nil {
		return err
	}
	messages := tx.Model(&models.WAGroupMessage{}).Select("id").Where("group_id = ?", group.ID)
	reports := tx.Model(&models.GraphReport{}).Select("id").Where("wa_group_id = ?", group.ID)
	steps := []struct {
		rows  any
		query string
		arg   any
	}{
		{&models.WAGroupReaction{}, "message_id IN (?)", messages},
		{&models.WAGroupReceipt{}, "message_id IN (?)", messages},
		{&models.WAGroupPollVote{}, "message_id IN (?)", messages},
		{&models.WAGroupMessage{}, "group_id = ?", group.ID},
		{&models.WAGroupParticipant{}, "group_id = ?", group.ID},
		{&models.GraphReportPort{}, "report_id IN (?)", reports},
		{&models.GraphReport{}, "wa_group_id = ?", group.ID},
	}
	for _, step := range steps {
		if err := tx.Where(step.query, step.arg).Delete(step.rows).Error; err != nil {
			return fmt.Errorf("delete group rows: %w", err)
		}
	}
	err := tx.Model(&models.WAGroupAction{}).Where("group_id = ?", group.ID).Update("group_id", nil).Error
	if err != nil {
		return fmt.Errorf("detach group actions: %w", err)
	}
	res := tx.Where("id = ? AND has_left", group.ID).Delete(&models.WAGroup{})
	if res.Error != nil {
		return fmt.Errorf("delete group: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrGroupNotLeft
	}
	return nil
}

// removeGroupFiles takes the group's files off the disk, reporting every path
// it could not rather than stopping at the first.
func removeGroupFiles(root string, paths []string) error {
	var failures []error
	for _, rel := range paths {
		full, ok := MediaPathWithin(root, rel)
		if !ok {
			failures = append(failures, fmt.Errorf("refuse %s: outside the media root", rel))
			continue
		}
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove %s: %w", rel, err))
		}
	}
	return errors.Join(failures...)
}

// detachTelegramReports lets a report that also goes to Telegram outlive the
// WhatsApp groups being deleted and carry on there; without it the group's
// foreign key cascades the whole report away. groups is one group's ID or a
// subquery of several. Every path that deletes groups goes through here, so
// they cannot disagree on which reports survive.
func detachTelegramReports(tx *gorm.DB, groups any) error {
	err := tx.Model(&models.GraphReport{}).Where("wa_group_id IN (?) AND telegram_chat_id IS NOT NULL", groups).
		Updates(map[string]any{"wa_group_id": nil, "wa_sent_on": nil}).Error
	if err != nil {
		return fmt.Errorf("detach graph reports: %w", err)
	}
	return nil
}
