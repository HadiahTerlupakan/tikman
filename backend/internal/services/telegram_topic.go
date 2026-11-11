package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"gorm.io/gorm/clause"
)

// TelegramTopicView is a forum topic as the report form lists it.
type TelegramTopicView struct {
	ThreadID int64  `json:"thread_id"`
	Name     string `json:"name"`
}

// TelegramChatView is a chat with the forum topics the bot has seen in it,
// empty for a chat that is no forum.
type TelegramChatView struct {
	models.TelegramChat
	Topics []TelegramTopicView `json:"topics"`
}

// topicSighting is what one message says of a topic. A name from the topic's
// creation or a rename is current; one read off the creation message a post
// replies to may be from before a rename, so it only fills a blank.
type topicSighting struct {
	threadID int64
	name     string
	current  bool
}

// sightTopic answers the topic msg shows, if any. A reply in General carries
// a thread ID too, so a plain message counts only when it is a topic message.
func sightTopic(msg *telegram.Message) (topicSighting, bool) {
	if !msg.Chat.IsForum || msg.MessageThreadID == 0 {
		return topicSighting{}, false
	}
	sighting := topicSighting{threadID: msg.MessageThreadID}
	switch {
	case msg.ForumTopicCreated != nil:
		sighting.name, sighting.current = msg.ForumTopicCreated.Name, true
	case msg.ForumTopicEdited != nil:
		// A rename of the icon alone carries no name, and is no rename.
		sighting.name, sighting.current = msg.ForumTopicEdited.Name, true
	case !msg.IsTopicMessage:
		return topicSighting{}, false
	case msg.ReplyToMessage != nil && msg.ReplyToMessage.ForumTopicCreated != nil:
		sighting.name = msg.ReplyToMessage.ForumTopicCreated.Name
	}
	sighting.name = clampRunes(sighting.name, graphReportTextRunes)
	return sighting, true
}

// recordTopic stores the topic msg shows, under the chat's row. A known name
// is never blanked: most sightings do not name the topic.
func (s *TelegramChatService) recordTopic(msg *telegram.Message) error {
	sighting, ok := sightTopic(msg)
	if !ok {
		return nil
	}
	var chat models.TelegramChat
	if err := s.db.Select("id").Where("chat_id = ?", msg.Chat.ID).Limit(1).Find(&chat).Error; err != nil {
		return fmt.Errorf("look up Telegram chat: %w", err)
	}
	if chat.ID == uuid.Nil {
		return nil
	}
	row := models.TelegramTopic{TelegramChatID: chat.ID, ThreadID: sighting.threadID, Name: sighting.name}
	err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "telegram_chat_id"}, {Name: "thread_id"}}, DoNothing: true,
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("store Telegram topic: %w", err)
	}
	if sighting.name == "" {
		return nil
	}
	rename := s.db.Model(&models.TelegramTopic{}).
		Where("telegram_chat_id = ? AND thread_id = ?", chat.ID, sighting.threadID)
	if !sighting.current {
		rename = rename.Where("name = ''")
	}
	if err := rename.Updates(map[string]any{"name": sighting.name, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("name Telegram topic: %w", err)
	}
	return nil
}

// List answers every chat, by title, with its topics by name.
func (s *TelegramChatService) List() ([]TelegramChatView, error) {
	chats := []models.TelegramChat{}
	if err := s.db.Order("title ASC").Find(&chats).Error; err != nil {
		return nil, fmt.Errorf("list Telegram chats: %w", err)
	}
	var topics []models.TelegramTopic
	if err := s.db.Order("name ASC, thread_id ASC").Find(&topics).Error; err != nil {
		return nil, fmt.Errorf("list Telegram topics: %w", err)
	}
	byChat := make(map[uuid.UUID][]TelegramTopicView, len(chats))
	for _, topic := range topics {
		byChat[topic.TelegramChatID] = append(byChat[topic.TelegramChatID],
			TelegramTopicView{ThreadID: topic.ThreadID, Name: topic.Name})
	}
	views := make([]TelegramChatView, len(chats))
	for i, chat := range chats {
		views[i] = TelegramChatView{TelegramChat: chat, Topics: byChat[chat.ID]}
		if views[i].Topics == nil {
			views[i].Topics = []TelegramTopicView{}
		}
	}
	return views, nil
}
