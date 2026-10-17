package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupRole is what one member may do in a WhatsApp group.
type GroupRole string

const (
	GroupRoleMember     GroupRole = "member"
	GroupRoleAdmin      GroupRole = "admin"
	GroupRoleSuperAdmin GroupRole = "superadmin"
)

// WAGroup is one WhatsApp group a paired number belongs to.
//
// Groups are kept apart from cs_conversations on purpose. A thread there is
// one customer: it is claimed by whoever answers, timed for the response KPI,
// linked to an ONT and offered as a broadcast recipient. None of that means
// anything for a group, and a busy group sharing that list would bury the
// customers waiting behind it.
//
// The metadata is a mirror of what WhatsApp answered and is rewritten on every
// sync. The summary fields (LastMessage*, UnreadCount) are ours, kept on the
// row so the list does not have to scan the messages table to draw itself.
type WAGroup struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	// The unique index is declared here as well as in migration 68 because the
	// sync upserts on it, and the SQLite suite only gets what the tags say.
	WAAccountID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_wa_groups_account_jid,priority:1" json:"wa_account_id"`
	// The column tag is load-bearing, as on WAAccount: GORM would name it "j_id".
	JID              string `gorm:"column:jid;type:varchar(128);not null;uniqueIndex:uq_wa_groups_account_jid,priority:2" json:"jid"`
	Name             string `gorm:"type:varchar(255)" json:"name"`
	Topic            string `gorm:"type:text" json:"topic"`
	TopicID          string `gorm:"type:varchar(64)" json:"-"`
	OwnerJID         string `gorm:"column:owner_jid;type:varchar(128)" json:"owner_jid"`
	ParticipantCount int    `gorm:"not null;default:0" json:"participant_count"`
	// IsAdmin is whether the paired number itself administers the group, which
	// is what decides whether it may manage it, or post in an announce group.
	IsAdmin  bool `gorm:"not null;default:false" json:"is_admin"`
	Announce bool `gorm:"not null;default:false" json:"announce"`
	Locked   bool `gorm:"not null;default:false" json:"locked"`
	// Left marks a group the number is no longer in. The row and its history
	// stay readable; sending to it is refused. The column is has_left because
	// LEFT is reserved in SQL.
	Left bool `gorm:"column:has_left;not null;default:false" json:"left"`

	LastMessageAt      *time.Time  `gorm:"index" json:"last_message_at,omitempty"`
	LastMessagePreview string      `gorm:"type:varchar(255)" json:"last_message_preview"`
	LastMessageKind    MessageKind `gorm:"type:varchar(20)" json:"last_message_kind"`
	LastMessageSender  string      `gorm:"type:varchar(255)" json:"last_message_sender"`
	// LastMessageDirection lets the list say "Anda:" for what the number
	// itself sent last, rather than naming it like any member.
	LastMessageDirection MessageDirection `gorm:"type:varchar(3)" json:"last_message_direction"`
	UnreadCount          int              `gorm:"not null;default:0" json:"unread_count"`

	SyncedAt  *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (g *WAGroup) BeforeCreate(tx *gorm.DB) error {
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroup) TableName() string { return "wa_groups" }

// WAGroupParticipant is one member of a group, as WhatsApp last listed them.
// The sync replaces a group's rows wholesale, so a member who left simply
// stops appearing.
type WAGroupParticipant struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"-"`
	// JID is the address WhatsApp sends to, a LID as often as a phone number.
	JID   string    `gorm:"column:jid;type:varchar(128);not null" json:"jid"`
	LID   string    `gorm:"column:lid;type:varchar(128)" json:"-"`
	Phone string    `gorm:"type:varchar(20)" json:"phone"`
	Name  string    `gorm:"type:varchar(255)" json:"name"`
	Role  GroupRole `gorm:"type:varchar(20);not null" json:"role"`
	// IsSelf marks the paired number itself. Stored rather than worked out on
	// the way out because only the wa process knows the number's LID, and in a
	// LID-addressed group that is the only name it appears under.
	IsSelf bool `gorm:"not null;default:false" json:"is_self"`
}

func (p *WAGroupParticipant) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupParticipant) TableName() string { return "wa_group_participants" }
