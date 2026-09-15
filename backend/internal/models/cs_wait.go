package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WaitEndReason is how a customer's wait for an answer ended.
type WaitEndReason string

const (
	// WaitReplied is a CS answering through TikMan.
	WaitReplied WaitEndReason = "replied"
	// WaitPhone is an answer typed outside TikMan, on a device linked to the
	// number, so nobody is recorded as having given it.
	WaitPhone WaitEndReason = "phone"
	// WaitClosed is a CS closing the thread without answering.
	WaitClosed WaitEndReason = "closed"
	// WaitAbandoned is a wait the customer left: they went quiet for more than
	// a day and then wrote about something new.
	WaitAbandoned WaitEndReason = "abandoned"
)

// CSWait is one stretch of a customer waiting for an answer, recorded as it
// happens rather than worked out from the messages later. Deleting messages,
// emptying a thread or removing a number leaves these rows alone, which is why
// there is no foreign key: a figure built on them cannot be changed by deleting
// what it was built from.
type CSWait struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	WAAccountID    uuid.UUID `gorm:"type:uuid;not null" json:"wa_account_id"`
	// StartedAt is when the wait's first message reached TikMan and
	// CustomerSentAt when the customer sent it. The gap between them is how the
	// report tells a system delay from a slow answer.
	StartedAt      time.Time `gorm:"not null" json:"started_at"`
	CustomerSentAt time.Time `gorm:"not null" json:"customer_sent_at"`
	// LastCustomerSentAt is the newest message in the wait by the customer's
	// clock, which is what the silence rule measures from.
	LastCustomerSentAt time.Time      `gorm:"not null" json:"last_customer_sent_at"`
	EndedAt            *time.Time     `json:"ended_at,omitempty"`
	EndReason          *WaitEndReason `gorm:"type:varchar(20)" json:"end_reason,omitempty"`
	// EndedBy is the CS who replied or closed. It is nil for a phone answer and
	// an abandoned wait.
	EndedBy        *uuid.UUID `gorm:"type:uuid" json:"ended_by,omitempty"`
	ReplyMessageID *uuid.UUID `gorm:"type:uuid" json:"reply_message_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (w *CSWait) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName specifies the table name for GORM.
func (CSWait) TableName() string { return "cs_waits" }
