package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// quoteTarget reads the message a reply quotes, answering false when the
// request named one that cannot be quoted — the refusal is already written.
//
// It runs before anything is stored. On the media path that ordering is what
// keeps an attachment from being written to disk for a reply that is about to
// be refused; it is also why SendMedia carries the id in the query string
// rather than the form, since reading a form field would parse the multipart
// body before MaxBytesReader has been wrapped around it.
func (h *CSHandler) quoteTarget(
	c *gin.Context, convID uuid.UUID, raw, code string,
) (*models.CSMessage, bool) {
	if raw == "" {
		return nil, true
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "reply_to_id bukan id yang sah",
			Code:  "INVALID_REPLY_TO_ID",
		})
		return nil, false
	}

	target, err := h.messages.QuoteTarget(convID, id)
	if err != nil {
		if errors.Is(err, services.ErrQuoteNotInThread) || errors.Is(err, services.ErrQuoteNotSent) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_REPLY_TO"})
			return nil, false
		}
		mapCSError(c, err, code)
		return nil, false
	}
	return target, true
}

// quotedID is the stored half of a quote: the id that goes in the row.
func quotedID(target *models.CSMessage) *uuid.UUID {
	if target == nil {
		return nil
	}
	return &target.ID
}

// attachQuote puts the quoted block on a freshly queued reply, so the thread
// draws it immediately instead of only after the next history fetch.
func attachQuote(msg *models.CSMessage, target *models.CSMessage) {
	if target != nil {
		msg.ReplyTo = target.AsQuote()
	}
}
