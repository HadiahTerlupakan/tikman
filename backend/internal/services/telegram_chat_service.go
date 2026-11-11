package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// reportableChatTypes are the chats a report can go to; a private chat with
// the bot is someone talking to it, not a customer's group.
var reportableChatTypes = map[string]bool{
	telegram.ChatTypeGroup: true, telegram.ChatTypeSupergroup: true, telegram.ChatTypeChannel: true,
}

// botMemberStatuses are the statuses in which the bot can post. "restricted"
// is left out: a restricted bot may well be unable to send photos.
var botMemberStatuses = map[string]bool{
	telegram.StatusMember: true, telegram.StatusAdministrator: true, telegram.StatusCreator: true,
}

// TelegramChatService keeps the chats the bot has been seen in.
type TelegramChatService struct {
	db *gorm.DB
}

// NewTelegramChatService constructs a TelegramChatService.
func NewTelegramChatService(db *gorm.DB) *TelegramChatService {
	return &TelegramChatService{db: db}
}

// List answers every chat, by title.
func (s *TelegramChatService) List() ([]models.TelegramChat, error) {
	chats := []models.TelegramChat{}
	if err := s.db.Order("title ASC").Find(&chats).Error; err != nil {
		return nil, fmt.Errorf("list Telegram chats: %w", err)
	}
	return chats, nil
}

// Apply records what one update says about a chat. Re-applying an update is
// harmless, as the bot's offset lives only in memory and a restart reads the
// unconfirmed ones again.
func (s *TelegramChatService) Apply(update telegram.Update) error {
	if member := update.MyChatMember; member != nil {
		return s.upsert(member.Chat, botMemberStatuses[member.NewChatMember.Status])
	}
	msg := update.Message
	if msg == nil {
		return nil
	}
	// The old group's last message: its row becomes the supergroup's, and is
	// not written back under the ID the group no longer has.
	if msg.MigrateToChatID != 0 {
		return s.Migrate(msg.Chat.ID, msg.MigrateToChatID)
	}
	if msg.MigrateFromChatID != 0 {
		if err := s.Migrate(msg.MigrateFromChatID, msg.Chat.ID); err != nil {
			return err
		}
	}
	// A message reaching the bot means the bot is in the chat.
	return s.upsert(msg.Chat, true)
}

func (s *TelegramChatService) upsert(chat telegram.Chat, active bool) error {
	if !reportableChatTypes[chat.Type] {
		return nil
	}
	row := models.TelegramChat{
		ChatID: chat.ID, Title: clampRunes(chat.Title, graphReportTextRunes), Type: chat.Type, Active: active,
	}
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "type", "active", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("store Telegram chat: %w", err)
	}
	// GORM leaves a false bool out of an insert, so a chat first seen as left
	// would take the column's default and read as active.
	if !active {
		return s.db.Model(&models.TelegramChat{}).Where("chat_id = ?", chat.ID).Update("active", false).Error
	}
	return nil
}

// Migrate moves a group's row to the supergroup it was upgraded to, so a
// report bound to the row follows. A row the supergroup already has, seen
// before the upgrade was, gives way unless a report holds it; when one does,
// that row is the supergroup and the group's row is only retired, since two
// rows cannot share a chat ID. Migrating a group already migrated does nothing.
func (s *TelegramChatService) Migrate(fromChatID, toChatID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var old models.TelegramChat
		err := tx.Where("chat_id = ?", fromChatID).Limit(1).Find(&old).Error
		if err != nil || old.ID == uuid.Nil {
			return err
		}
		held := tx.Model(&models.GraphReport{}).Select("telegram_chat_id").Where("telegram_chat_id IS NOT NULL")
		err = tx.Where("chat_id = ? AND id NOT IN (?)", toChatID, held).Delete(&models.TelegramChat{}).Error
		if err != nil {
			return fmt.Errorf("clear supergroup row: %w", err)
		}
		var kept int64
		if err := tx.Model(&models.TelegramChat{}).Where("chat_id = ?", toChatID).Count(&kept).Error; err != nil {
			return fmt.Errorf("look for supergroup row: %w", err)
		}
		if kept > 0 {
			return tx.Model(&old).Updates(map[string]any{"active": false, "updated_at": time.Now()}).Error
		}
		err = tx.Model(&old).Updates(map[string]any{
			"chat_id": toChatID, "type": telegram.ChatTypeSupergroup, "active": true, "updated_at": time.Now(),
		}).Error
		if err != nil {
			return fmt.Errorf("migrate Telegram chat: %w", err)
		}
		return nil
	})
}

// DeactivateAll marks every chat as one the bot is not in. Rows are learnt
// from one bot's updates; another bot is in none of them until its own
// updates say so.
func (s *TelegramChatService) DeactivateAll() error {
	err := s.db.Model(&models.TelegramChat{}).Where("active").
		Updates(map[string]any{"active": false, "updated_at": time.Now()}).Error
	if err != nil {
		return fmt.Errorf("deactivate Telegram chats: %w", err)
	}
	return nil
}

// Deactivate marks a chat as one the bot can no longer post to.
func (s *TelegramChatService) Deactivate(id uuid.UUID) error {
	err := s.db.Model(&models.TelegramChat{}).Where("id = ?", id).
		Updates(map[string]any{"active": false, "updated_at": time.Now()}).Error
	if err != nil {
		return fmt.Errorf("deactivate Telegram chat: %w", err)
	}
	return nil
}
