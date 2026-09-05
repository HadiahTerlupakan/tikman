package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageDirection says who wrote a message.
type MessageDirection string

const (
	MessageIn  MessageDirection = "in"
	MessageOut MessageDirection = "out"
)

// MessageKind is what a message carries.
type MessageKind string

const (
	MessageKindText     MessageKind = "text"
	MessageKindImage    MessageKind = "image"
	MessageKindDocument MessageKind = "document"
	MessageKindAudio    MessageKind = "audio"
	MessageKindVideo    MessageKind = "video"
)

// MessageStatus is how far an outbound message has travelled. Inbound messages
// are stored as delivered.
type MessageStatus string

const (
	MessageQueued    MessageStatus = "queued"
	MessageSent      MessageStatus = "sent"
	MessageDelivered MessageStatus = "delivered"
	MessageRead      MessageStatus = "read"
	MessageFailed    MessageStatus = "failed"
)

// CSMessage is one WhatsApp message in a thread.
//
// A row with Status MessageQueued is the outbox: there is no second table. A
// reply written while the WhatsApp process was down is still sitting here when
// it comes back, which is what stops a CS reply from vanishing silently.
//
// WAMessageID is a pointer because an outbound message has no WhatsApp id until
// it is sent, and many may wait at once. The uniqueness that makes inbound
// storage idempotent is a partial index added in migration 41; SQLite tests do
// not get it, so CSMessageService checks for the duplicate in Go as well.
type CSMessage struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID        `gorm:"type:uuid;not null;index" json:"conversation_id"`
	WAMessageID    *string          `gorm:"type:varchar(128);index" json:"wa_message_id,omitempty"`
	Direction      MessageDirection `gorm:"type:varchar(3);not null" json:"direction"`
	SenderUserID   *uuid.UUID       `gorm:"type:uuid;index" json:"sender_user_id,omitempty"`
	Kind           MessageKind      `gorm:"type:varchar(20);not null" json:"kind"`
	Body           string           `gorm:"type:text" json:"body"`
	MediaPath      string           `gorm:"type:text" json:"-"`
	MediaMime      string           `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize      int64            `json:"media_size,omitempty"`
	MediaFilename  string           `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status         MessageStatus    `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason     string           `gorm:"type:text" json:"fail_reason,omitempty"`
	ReplyToID      *uuid.UUID       `gorm:"type:uuid" json:"reply_to_id,omitempty"`

	// The link card, stored rather than resolved on display: an outgoing one
	// is what the wa process already fetched to attach to the message, and an
	// incoming one arrives inside the protobuf WhatsApp sends. Neither costs a
	// request here, and a card kept with its message still renders years later
	// when the page behind it has gone.
	PreviewURL         string `gorm:"type:text" json:"preview_url,omitempty"`
	PreviewTitle       string `gorm:"type:text" json:"preview_title,omitempty"`
	PreviewDescription string `gorm:"type:text" json:"preview_description,omitempty"`
	// PreviewThumbnail is a JPEG. Measured on this deployment 3.7% of messages
	// carry a link, so the media retention sweep deliberately leaves these
	// alone — it clears files from disk, and these are rows.
	// encoding/json renders []byte as base64, the same shape the composer
	// already receives from the link-preview endpoint.
	PreviewThumbnail []byte    `gorm:"type:bytea" json:"preview_thumbnail,omitempty"`
	WATimestamp      time.Time `gorm:"index" json:"wa_timestamp"`
	CreatedAt        time.Time `json:"created_at"`

	// ReplyTo is the message this one quotes, filled in on the way out of the
	// service. It is not a column — the quote is stored as ReplyToID alone —
	// because a copy of the quoted text would be a second version of words that
	// already exist one row away.
	ReplyTo *QuotedMessage `gorm:"-" json:"reply_to,omitempty"`
}

// QuotedMessage is as much of a quoted message as a thread needs to draw the
// little block above a reply. It is deliberately not the whole message: a
// quote is a pointer back, not a second copy of the conversation.
type QuotedMessage struct {
	ID            uuid.UUID        `json:"id"`
	Direction     MessageDirection `json:"direction"`
	Kind          MessageKind      `json:"kind"`
	Body          string           `json:"body"`
	MediaFilename string           `json:"media_filename,omitempty"`
}

// AsQuote reduces a message to the block a reply draws above itself.
func (m *CSMessage) AsQuote() *QuotedMessage {
	return &QuotedMessage{
		ID:            m.ID,
		Direction:     m.Direction,
		Kind:          m.Kind,
		Body:          m.Body,
		MediaFilename: m.MediaFilename,
	}
}

func (m *CSMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSMessage) TableName() string { return "cs_messages" }

// CSQuickReply is a canned answer a CS can insert instead of retyping it.
type CSQuickReply struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string    `gorm:"type:varchar(100);not null" json:"title"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (q *CSQuickReply) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSQuickReply) TableName() string { return "cs_quick_replies" }
