package services

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// validateTelegram checks the report's Telegram chat and topic. What the
// report already goes to is let through where a new choice would not be: the
// table's on/off switch resends the whole report, and turning off a report
// whose bot was removed, or whose forum lost its topics, must not need a new
// destination first.
func (s *GraphReportService) validateTelegram(in GraphReportInput, current *models.GraphReport) error {
	if in.TelegramChatID == nil {
		if in.TelegramThreadID != nil {
			return fmt.Errorf("%w: topik Telegram dipilih tanpa grup Telegram", ErrValidation)
		}
		return nil
	}
	keepingChat := current != nil && current.TelegramChatID != nil && *current.TelegramChatID == *in.TelegramChatID
	chat, err := s.validateTelegramChat(*in.TelegramChatID, keepingChat)
	if err != nil || in.TelegramThreadID == nil {
		return err
	}
	if keepingChat && current.TelegramThreadID != nil && *current.TelegramThreadID == *in.TelegramThreadID {
		return nil
	}
	return s.validateTelegramTopic(chat, *in.TelegramThreadID)
}

// validateTelegramChat refuses a chat the bot has left, unless the report
// already goes there.
func (s *GraphReportService) validateTelegramChat(id uuid.UUID, keeping bool) (models.TelegramChat, error) {
	var chat models.TelegramChat
	err := s.db.Select("id", "active", "is_forum").First(&chat, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return chat, fmt.Errorf("%w: grup Telegram tidak ditemukan", ErrValidation)
	}
	if err != nil {
		return chat, fmt.Errorf("look up Telegram chat: %w", err)
	}
	if !chat.Active && !keeping {
		return chat, fmt.Errorf("%w: bot sudah tidak ada di grup Telegram ini", ErrValidation)
	}
	return chat, nil
}

// validateTelegramTopic refuses a topic the bot has not seen in the chat.
// Telegram would refuse an unknown one only at the first morning's send.
func (s *GraphReportService) validateTelegramTopic(chat models.TelegramChat, threadID int64) error {
	if !chat.IsForum {
		return fmt.Errorf("%w: grup Telegram ini tidak memakai topik", ErrValidation)
	}
	var known int64
	err := s.db.Model(&models.TelegramTopic{}).
		Where("telegram_chat_id = ? AND thread_id = ?", chat.ID, threadID).Count(&known).Error
	if err != nil {
		return fmt.Errorf("look up Telegram topic: %w", err)
	}
	if known == 0 {
		return fmt.Errorf("%w: topik Telegram tidak ditemukan", ErrValidation)
	}
	return nil
}

// claimDestinations refuses a report for a group, chat or forum topic that
// already has one. It runs in the write's transaction.
func claimDestinations(tx *gorm.DB, in GraphReportInput, self uuid.UUID) error {
	if in.WAGroupID != nil {
		if err := claimGroup(tx, *in.WAGroupID, self); err != nil {
			return err
		}
	}
	if in.TelegramChatID == nil {
		return nil
	}
	taken := tx.Model(&models.GraphReport{}).Where("telegram_chat_id = ? AND id <> ?", *in.TelegramChatID, self)
	message := "grup Telegram ini sudah memiliki laporan grafik"
	if in.TelegramThreadID == nil {
		taken = taken.Where("telegram_thread_id IS NULL")
	} else {
		taken = taken.Where("telegram_thread_id = ?", *in.TelegramThreadID)
		message = "topik Telegram ini sudah memiliki laporan grafik"
	}
	var count int64
	if err := taken.Count(&count).Error; err != nil {
		return fmt.Errorf("look for existing graph report: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("%w: %s", ErrValidation, message)
	}
	return nil
}
