package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WAGroupReaction is one member's emoji on one group message. A member has
// at most one: choosing another replaces it, and an empty one removes it.
type WAGroupReaction struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_wa_group_reactions_member,priority:1"`
	ReactorJID  string    `gorm:"column:reactor_jid;type:varchar(128);not null;uniqueIndex:uq_wa_group_reactions_member,priority:2"`
	ReactorName string    `gorm:"type:varchar(255);not null;default:''"`
	IsSelf      bool      `gorm:"not null;default:false"`
	Emoji       string    `gorm:"type:varchar(32);not null"`
	UpdatedAt   time.Time
}

func (r *WAGroupReaction) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupReaction) TableName() string { return "wa_group_reactions" }

// WAGroupReceipt is how far one of our messages got with one member.
type WAGroupReceipt struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"-"`
	MessageID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_wa_group_receipts_member,priority:1" json:"-"`
	ReaderJID   string     `gorm:"column:reader_jid;type:varchar(128);not null;uniqueIndex:uq_wa_group_receipts_member,priority:2" json:"jid"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	// Name is filled in on the way out from the member list.
	Name string `gorm:"-" json:"name"`
}

func (r *WAGroupReceipt) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupReceipt) TableName() string { return "wa_group_receipts" }

// WAGroupPollVote is one member's current choice in a poll. WhatsApp sends
// the whole selection on every change, so the row is replaced, not added to.
type WAGroupPollVote struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_wa_group_poll_votes_member,priority:1"`
	VoterJID  string    `gorm:"column:voter_jid;type:varchar(128);not null;uniqueIndex:uq_wa_group_poll_votes_member,priority:2"`
	VoterName string    `gorm:"type:varchar(255);not null;default:''"`
	IsSelf    bool      `gorm:"not null;default:false"`
	// Options is the JSON list of the option names chosen.
	Options   string `gorm:"type:text;not null"`
	UpdatedAt time.Time
}

func (v *WAGroupPollVote) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAGroupPollVote) TableName() string { return "wa_group_poll_votes" }

// WAAvatar is the profile photo of a group or a person, as one number sees
// it. Groups and their members share the table because a member's face is
// the same in every group, and fetching it once per group would multiply the
// queries that get an unofficial number flagged.
type WAAvatar struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	WAAccountID uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:uq_wa_avatars_account_jid,priority:1"`
	JID         string     `gorm:"column:jid;type:varchar(128);not null;uniqueIndex:uq_wa_avatars_account_jid,priority:2"`
	AvatarID    string     `gorm:"type:varchar(64);not null;default:''"`
	Path        string     `gorm:"type:text;not null;default:''"`
	CheckedAt   *time.Time `gorm:"index"`
}

func (a *WAAvatar) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (WAAvatar) TableName() string { return "wa_avatars" }
