package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// PortAlertDestinationInput is the NOC destination as the admin submitted
// it. Both destinations nil leaves the NOC without notices.
type PortAlertDestinationInput struct {
	WAGroupID        *uuid.UUID
	TelegramChatID   *uuid.UUID
	TelegramThreadID *int64
	Enabled          bool
}

// PortAlertDestinationView is the NOC destination as the page shows it, null
// or empty where it has none.
type PortAlertDestinationView struct {
	WAAccountID       *uuid.UUID `json:"wa_account_id"`
	WAGroupID         *uuid.UUID `json:"wa_group_id"`
	GroupName         string     `json:"group_name"`
	TelegramChatID    *uuid.UUID `json:"telegram_chat_id"`
	TelegramChatTitle string     `json:"telegram_chat_title"`
	TelegramThreadID  *int64     `json:"telegram_thread_id"`
	TelegramTopicName string     `json:"telegram_topic_name"`
	Enabled           bool       `json:"enabled"`
}

// PortAlertDestinationService stores where the NOC hears of every port
// going down or up.
type PortAlertDestinationService struct {
	db      *gorm.DB
	reports *GraphReportService
}

// NewPortAlertDestinationService constructs a PortAlertDestinationService.
func NewPortAlertDestinationService(db *gorm.DB) *PortAlertDestinationService {
	return &PortAlertDestinationService{db: db, reports: NewGraphReportService(db)}
}

// Get answers the NOC destination; one never set reads as none, enabled.
func (s *PortAlertDestinationService) Get() (*PortAlertDestinationView, error) {
	row, err := s.current()
	if err != nil {
		return nil, err
	}
	view := PortAlertDestinationView{
		WAGroupID: row.WAGroupID, TelegramChatID: row.TelegramChatID, TelegramThreadID: row.TelegramThreadID,
		Enabled: row.ID == uuid.Nil || row.Enabled,
	}
	if row.WAGroupID != nil {
		groups, _, err := reportGroups(s.db, []uuid.UUID{*row.WAGroupID})
		if err != nil {
			return nil, err
		}
		if group, ok := groups[*row.WAGroupID]; ok {
			view.GroupName, view.WAAccountID = group.Name, &group.WAAccountID
		}
	}
	if row.TelegramChatID != nil {
		if err := s.nameTelegram(&view); err != nil {
			return nil, err
		}
	}
	return &view, nil
}

func (s *PortAlertDestinationService) nameTelegram(view *PortAlertDestinationView) error {
	chats, err := reportChats(s.db, []uuid.UUID{*view.TelegramChatID})
	if err != nil {
		return err
	}
	view.TelegramChatTitle = chats[*view.TelegramChatID]
	if view.TelegramThreadID == nil {
		return nil
	}
	topics, err := reportTopics(s.db, []uuid.UUID{*view.TelegramChatID})
	if err != nil {
		return err
	}
	view.TelegramTopicName = topics[topicKey{*view.TelegramChatID, *view.TelegramThreadID}]
	return nil
}

// Set replaces the NOC destination. It is checked as a report's would be,
// save that a group or topic may also carry a report: the NOC reading its
// own customers' notices there is a choice, not a clash.
func (s *PortAlertDestinationService) Set(in PortAlertDestinationInput) (*PortAlertDestinationView, error) {
	row, err := s.current()
	if err != nil {
		return nil, err
	}
	if in.WAGroupID != nil {
		if err := s.reports.validateWAGroup(*in.WAGroupID); err != nil {
			return nil, err
		}
	}
	held := &models.GraphReport{TelegramChatID: row.TelegramChatID, TelegramThreadID: row.TelegramThreadID}
	telegramIn := GraphReportInput{TelegramChatID: in.TelegramChatID, TelegramThreadID: in.TelegramThreadID}
	if err := s.reports.validateTelegram(telegramIn, held); err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		row = models.PortAlertDestination{
			WAGroupID: in.WAGroupID, TelegramChatID: in.TelegramChatID, TelegramThreadID: in.TelegramThreadID,
			Enabled: in.Enabled,
		}
		err = s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			// GORM leaves a false bool out of an insert, and the column's
			// default would turn the notices on.
			return tx.Model(&row).Update("enabled", in.Enabled).Error
		})
	} else {
		err = s.db.Model(&row).Updates(map[string]any{
			"wa_group_id": in.WAGroupID, "telegram_chat_id": in.TelegramChatID,
			"telegram_thread_id": in.TelegramThreadID, "enabled": in.Enabled,
		}).Error
	}
	if err != nil {
		return nil, fmt.Errorf("store NOC port alert destination: %w", err)
	}
	return s.Get()
}

func (s *PortAlertDestinationService) current() (models.PortAlertDestination, error) {
	var row models.PortAlertDestination
	if err := s.db.Limit(1).Find(&row).Error; err != nil {
		return row, fmt.Errorf("load NOC port alert destination: %w", err)
	}
	return row, nil
}

// detachPortAlertGroups clears the WhatsApp groups being deleted from the NOC
// destination. Postgres does this by migration 77's foreign key; SQLite has
// none, and the paths that delete groups keep the two the same.
func detachPortAlertGroups(tx *gorm.DB, groups any) error {
	err := tx.Model(&models.PortAlertDestination{}).Where("wa_group_id IN (?)", groups).
		Update("wa_group_id", nil).Error
	if err != nil {
		return fmt.Errorf("detach NOC port alert group: %w", err)
	}
	return nil
}
