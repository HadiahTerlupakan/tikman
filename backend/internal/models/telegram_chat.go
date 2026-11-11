package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TelegramChat is a group or channel the bot has been seen in, learnt from
// its updates: the Bot API cannot list a bot's chats, so a chat appears here
// only once the bot is added to it or reads a message there.
type TelegramChat struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	// ChatID is Telegram's own ID. A group upgraded to a supergroup gets a new
	// one, written over the old so a report bound to the row keeps working.
	ChatID int64  `gorm:"not null;uniqueIndex" json:"chat_id"`
	Title  string `gorm:"type:varchar(255);not null;default:''" json:"title"`
	Type   string `gorm:"type:varchar(32);not null;default:''" json:"type"`
	// IsForum is a supergroup with topics, as Telegram last described it.
	IsForum bool `gorm:"not null;default:false" json:"is_forum"`
	// Active is false once the bot has left or been removed.
	Active    bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// TableName pins the table name.
func (TelegramChat) TableName() string { return "telegram_chats" }

// BeforeCreate assigns the ID.
func (c *TelegramChat) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// TelegramTopic is a forum topic the bot has seen in a chat. The Bot API has
// no call listing a forum's topics, so one appears here only once a message
// shows it: its creation, a rename, or a post inside it.
type TelegramTopic struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	TelegramChatID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_telegram_topics_chat_thread" json:"-"`
	ThreadID       int64     `gorm:"not null;uniqueIndex:idx_telegram_topics_chat_thread" json:"thread_id"`
	// Name is empty for a topic seen only through a message that did not
	// name it.
	Name      string    `gorm:"type:varchar(255);not null;default:''" json:"name"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// TableName pins the table name.
func (TelegramTopic) TableName() string { return "telegram_topics" }

// BeforeCreate assigns the ID.
func (t *TelegramTopic) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
