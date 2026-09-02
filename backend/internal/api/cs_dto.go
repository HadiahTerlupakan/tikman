package api

import (
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// SendMessageRequest is a CS reply on a thread they hold.
type SendMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

// AssignRequest hands a thread to one CS. Sending your own id is taking it over.
type AssignRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}

// SetStatusRequest closes or reopens a thread.
type SetStatusRequest struct {
	Status models.ConversationStatus `json:"status" binding:"required"`
}

// LinkONTRequest ties a thread to a subscriber's ONT, or unties it when null.
type LinkONTRequest struct {
	ONTID *uuid.UUID `json:"ont_id"`
}
