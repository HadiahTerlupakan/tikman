package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WAGroupMessage is one message in a group.
//
// It mirrors CSMessage, including being its own outbox (a queued row is what
// the wa process sends), with one real difference: a group has many senders,
// so who wrote each message is stored on the message rather than on the
// thread.
type WAGroupMessage struct {
	ID          uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	GroupID     uuid.UUID        `gorm:"type:uuid;not null;index" json:"group_id"`
	WAMessageID *string          `gorm:"type:varchar(128);index" json:"wa_message_id,omitempty"`
	Direction   MessageDirection `gorm:"type:varchar(3);not null" json:"direction"`
	SenderJID   string           `gorm:"column:sender_jid;type:varchar(128)" json:"sender_jid"`
	SenderPhone string           `gorm:"type:varchar(20)" json:"sender_phone"`
	SenderName  string           `gorm:"type:varchar(255)" json:"sender_name"`
	// SenderUserID is the TikMan user who wrote an outbound row, nil for one
	// typed on the phone.
	SenderUserID  *uuid.UUID    `gorm:"type:uuid" json:"sender_user_id,omitempty"`
	Kind          MessageKind   `gorm:"type:varchar(20);not null" json:"kind"`
	Body          string        `gorm:"type:text" json:"body"`
	MediaPath     string        `gorm:"type:text" json:"-"`
	MediaMime     string        `gorm:"type:varchar(100)" json:"media_mime,omitempty"`
	MediaSize     int64         `json:"media_size,omitempty"`
	MediaFilename string        `gorm:"type:varchar(255)" json:"media_filename,omitempty"`
	Status        MessageStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	FailReason    string        `gorm:"type:text" json:"fail_reason,omitempty"`
	ReplyToID     *uuid.UUID    `gorm:"type:uuid" json:"reply_to_id,omitempty"`
	// Mentions is the JSON list of members an outgoing message tags. Body
	// keeps WhatsApp's own form, "@<address>", because that is what has to be
	// sent; the history shows it with names (see services.ResolveMentions).
	Mentions string `gorm:"type:text;not null;default:''" json:"-"`

	PreviewURL         string `gorm:"type:text" json:"preview_url,omitempty"`
	PreviewTitle       string `gorm:"type:text" json:"preview_title,omitempty"`
	PreviewDescription string `gorm:"type:text" json:"preview_description,omitempty"`
	PreviewThumbnail   []byte `gorm:"type:bytea" json:"preview_thumbnail,omitempty"`

	WATimestamp time.Time `gorm:"index" json:"wa_timestamp"`
	CreatedAt   time.Time `json:"created_at"`

	ReplyTo *GroupQuotedMessage `gorm:"-" json:"reply_to,omitempty"`
}

// GroupQuotedMessage is the block a group reply draws above itself. Unlike a
// customer thread, "the other side" is not one person, so it names the sender.
type GroupQuotedMessage struct {
	ID            uuid.UUID        `json:"id"`
	Direction     MessageDirection `json:"direction"`
	Kind          MessageKind      `json:"kind"`
	Body          string           `json:"body"`
	MediaFilename string           `json:"media_filename,omitempty"`
	SenderName    string           `json:"sender_name"`
}

// AsQuote reduces a message to the block a reply draws above itself.
func (m *WAGroupMessage) AsQuote() *GroupQuotedMessage {
	return &GroupQuotedMessage{
		ID:            m.ID,
		Direction:     m.Direction,
		Kind:          m.Kind,
		Body:          m.Body,
		MediaFilename: m.MediaFilename,
		SenderName:    m.SenderName,
	}
}

func (m *WAGroupMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupMessage) TableName() string { return "wa_group_messages" }

// GroupActionKind is one thing an admin can ask the number to do to a group.
type GroupActionKind string

const (
	GroupActionCreate          GroupActionKind = "create"
	GroupActionRename          GroupActionKind = "rename"
	GroupActionSetTopic        GroupActionKind = "set_topic"
	GroupActionAdd             GroupActionKind = "add"
	GroupActionRemove          GroupActionKind = "remove"
	GroupActionPromote         GroupActionKind = "promote"
	GroupActionDemote          GroupActionKind = "demote"
	GroupActionSetAnnounce     GroupActionKind = "set_announce"
	GroupActionSetLocked       GroupActionKind = "set_locked"
	GroupActionInviteLink      GroupActionKind = "invite_link"
	GroupActionResetInviteLink GroupActionKind = "reset_invite_link"
	GroupActionLeave           GroupActionKind = "leave"
)

// GroupActionStatus is how far a requested action got.
type GroupActionStatus string

const (
	GroupActionQueued GroupActionStatus = "queued"
	GroupActionDone   GroupActionStatus = "done"
	GroupActionFailed GroupActionStatus = "failed"
)

// WAGroupAction is one management request waiting for, or answered by, the wa
// process.
//
// A row rather than a control message on Redis: the API holds no WhatsApp
// connection, so it cannot do these itself, and an admin who asks to remove a
// member needs to learn whether WhatsApp agreed. Pub/sub would lose the
// request if the wa process were down and has nowhere to put the answer.
type WAGroupAction struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	WAAccountID uuid.UUID       `gorm:"type:uuid;not null;index" json:"wa_account_id"`
	GroupID     *uuid.UUID      `gorm:"type:uuid;index" json:"group_id,omitempty"`
	Action      GroupActionKind `gorm:"type:varchar(30);not null" json:"action"`
	// Payload is the request's arguments as JSON (see services.GroupActionArgs).
	Payload     string            `gorm:"type:text;not null" json:"-"`
	Status      GroupActionStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	Result      string            `gorm:"type:text" json:"result"`
	FailReason  string            `gorm:"type:text" json:"fail_reason"`
	RequestedBy uuid.UUID         `gorm:"type:uuid;not null" json:"requested_by"`
	CreatedAt   time.Time         `json:"created_at"`
	DoneAt      *time.Time        `json:"done_at,omitempty"`
}

func (a *WAGroupAction) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupAction) TableName() string { return "wa_group_actions" }
