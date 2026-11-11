package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ErrGraphReportGroupTaken is a report for a WhatsApp group that already has
// one through another of TikMan's numbers. Each number mirrors the group as a
// row of its own, so the per-row unique index cannot see it, and both numbers
// would post every report.
var ErrGraphReportGroupTaken = errors.New("Grup ini sudah punya laporan")

// groupReportHolder is a report already on a group with the same WhatsApp JID.
type groupReportHolder struct {
	WAGroupID   uuid.UUID
	WAAccountID uuid.UUID
}

// claimGroup refuses a report for a group that already has one, on this row
// or on another number's row for the same WhatsApp group. It runs in the
// write's transaction, and on Postgres under a lock on the JID, so two admins
// saving through two numbers at once cannot both pass.
func claimGroup(tx *gorm.DB, groupID, self uuid.UUID) error {
	var group models.WAGroup
	if err := tx.Select("id", "jid").First(&group, "id = ?", groupID).Error; err != nil {
		return fmt.Errorf("look up group: %w", err)
	}
	if tx.Name() == "postgres" {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "graph-report-group:"+group.JID).Error; err != nil {
			return fmt.Errorf("lock group: %w", err)
		}
	}
	var holders []groupReportHolder
	err := tx.Table("graph_reports").Select("graph_reports.wa_group_id, wa_groups.wa_account_id").
		Joins("JOIN wa_groups ON wa_groups.id = graph_reports.wa_group_id").
		Where("wa_groups.jid = ? AND graph_reports.id <> ?", group.JID, self).Scan(&holders).Error
	if err != nil {
		return fmt.Errorf("look for existing graph report: %w", err)
	}
	if len(holders) == 0 {
		return nil
	}
	for _, holder := range holders {
		if holder.WAGroupID == groupID {
			return ErrGraphReportDuplicate
		}
	}
	return groupTakenBy(tx, holders[0].WAAccountID)
}

func groupTakenBy(tx *gorm.DB, accountID uuid.UUID) error {
	names, err := accountNames(tx, []uuid.UUID{accountID})
	if err != nil {
		return err
	}
	return fmt.Errorf("%w lewat nomor %s", ErrGraphReportGroupTaken, names[accountID])
}

// accountNames answers what the page calls each number: its label, or the
// number itself for one nobody labelled.
func accountNames(db *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	var accounts []models.WAAccount
	if err := db.Select("id", "label", "jid").Where("id IN ?", ids).Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("load WhatsApp numbers: %w", err)
	}
	names := make(map[uuid.UUID]string, len(accounts))
	for _, account := range accounts {
		names[account.ID] = accountName(account)
	}
	return names, nil
}

func accountName(account models.WAAccount) string {
	if label := strings.TrimSpace(account.Label); label != "" {
		return label
	}
	return numberUser(account.JID)
}
