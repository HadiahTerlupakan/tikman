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
