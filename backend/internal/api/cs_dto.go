package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// TypingRequest is a CS starting or stopping writing on a thread they hold.
// The field is not required: a false has to be as sendable as a true, and
// binding:"required" would reject exactly the message that clears the line.
type TypingRequest struct {
	Typing bool `json:"typing"`
}

// SendMessageRequest is a CS reply on a thread they hold.
type SendMessageRequest struct {
	Body string `json:"body" binding:"required"`
	// ReplyToID names the message this reply quotes, empty when it quotes
	// nothing.
	ReplyToID string `json:"reply_to_id"`
}

// AssignRequest hands a thread to one CS. Sending your own id is taking it over.
type AssignRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

// SetStatusRequest closes a thread. There is no manual reopen: a thread
// comes back on its own when the customer writes again, and a CS who closed
// one by mistake takes it back with Assign.
type SetStatusRequest struct {
	Status models.ConversationStatus `json:"status" binding:"required"`
}

// LinkONTRequest ties a thread to a subscriber's ONT, or unties it when null.
type LinkONTRequest struct {
	ONTID *uuid.UUID `json:"ont_id"`
}

// CreateAccountRequest adds a WhatsApp number for the team to answer from.
type CreateAccountRequest struct {
	Label string `json:"label" binding:"required"`
}
