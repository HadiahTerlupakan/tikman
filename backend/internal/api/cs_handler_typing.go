package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// SetTyping shows or clears the "typing…" line on the customer's phone.
//
// Only the CS holding the thread may raise it. Anyone with the inbox open can
// read along, and a customer watching two colleagues browse past would see
// somebody typing who was never going to answer.
//
// The state is not stored anywhere. It is true for a few seconds and then is
// not, so it is handed straight to the process holding the WhatsApp connection
// and forgotten; a browser that closes mid-word leaves a line the customer's
// own phone clears after a moment.
func (h *CSHandler) SetTyping(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	var req TypingRequest
	if !bindJSON(c, &req) {
		return
	}

	userID, _ := middleware.GetUserID(c)
	if err := h.conversations.EnsureHolder(convID, userID); err != nil {
		h.refuseNotHolder(c, convID, err, "TYPING_FAILED")
		return
	}

	payload, err := json.Marshal(wa.PresenceMessage{
		ConversationID: convID.String(),
		Typing:         req.Typing,
	})
	if err != nil {
		mapCSError(c, err, "TYPING_FAILED")
		return
	}
	if err := h.redis.Publish(c.Request.Context(), wa.PresenceChannel, payload).Err(); err != nil {
		// The reply itself is unaffected, and the line the customer would have
		// seen is worth nothing a moment later.
		h.logger.Warn("publish cs typing update failed",
			zap.String("conversation_id", convID.String()), zap.Error(err))
	}
	c.Status(http.StatusNoContent)
}
