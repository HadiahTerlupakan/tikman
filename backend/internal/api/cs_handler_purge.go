package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
)

// DeleteMessage removes one message from the inbox, together with whatever it
// was carrying.
//
// It removes it here and nowhere else: the copy on the customer's phone stays.
// Taking that one back is WhatsApp's revoke, which only works on messages the
// CS sent and only for a while after sending, so offering it on this button
// would work for some rows and silently not for others.
func (h *CSHandler) DeleteMessage(c *gin.Context) {
	msgID, ok := pathUUID(c, "id", "INVALID_MESSAGE_ID")
	if !ok {
		return
	}

	msg, err := h.messages.Get(msgID)
	if err != nil {
		mapCSError(c, err, "MESSAGE_DELETE_FAILED")
		return
	}
	if !h.mayPurgeThread(c, msg.ConversationID, "MESSAGE_DELETE_FAILED") {
		return
	}

	if _, err := h.purge.Message(msgID); err != nil {
		mapCSError(c, err, "MESSAGE_DELETE_FAILED")
		return
	}

	h.auditPurge(c, "delete_message", "cs_message", msgID, 1)
	h.announceEvent(c.Request.Context(), wa.Event{
		Type:           wa.EventStatus,
		ConversationID: msg.ConversationID.String(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"removed": 1}})
}

// ClearConversation empties one thread, leaving the thread itself in the inbox.
func (h *CSHandler) ClearConversation(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	if _, err := h.conversations.Get(convID); err != nil {
		mapCSError(c, err, "CONVERSATION_CLEAR_FAILED")
		return
	}
	if !h.mayPurgeThread(c, convID, "CONVERSATION_CLEAR_FAILED") {
		return
	}

	removed, err := h.purge.Conversation(convID)
	if err != nil {
		mapCSError(c, err, "CONVERSATION_CLEAR_FAILED")
		return
	}

	h.auditPurge(c, "clear_messages", "cs_conversation", convID, removed)
	h.announceEvent(c.Request.Context(), wa.Event{
		Type:           wa.EventStatus,
		ConversationID: convID.String(),
	})
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"removed": removed}})
}

// ClearAccountMessages empties every thread on one number without removing the
// number itself, so it keeps answering with a clean history behind it.
func (h *CSHandler) ClearAccountMessages(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	if _, err := h.accounts.Get(id); err != nil {
		mapCSError(c, err, "ACCOUNT_CLEAR_FAILED")
		return
	}

	removed, err := h.purge.Account(id)
	if err != nil {
		mapCSError(c, err, "ACCOUNT_CLEAR_FAILED")
		return
	}

	h.auditPurge(c, "clear_messages", "wa_account", id, removed)
	h.announceInboxChanged(c)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"removed": removed}})
}

// ClearInbox empties every thread on every number.
func (h *CSHandler) ClearInbox(c *gin.Context) {
	removed, err := h.purge.Inbox()
	if err != nil {
		mapCSError(c, err, "INBOX_CLEAR_FAILED")
		return
	}

	h.auditPurge(c, "clear_messages", "cs_inbox", uuid.Nil, removed)
	h.announceInboxChanged(c)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"removed": removed}})
}

// DeleteAccount removes a WhatsApp number along with every thread, message and
// file belonging to it.
//
// The rows go first and the pairing second. The API owns the database and can
// report a real failure to the admin who asked; the wa process is reached only
// over a fire-and-forget channel, so making the deletion wait on it would mean
// a number that silently stayed when that process was down. Going the other
// way round, the worst case is a pairing left standing, and the rescan in that
// process closes the session on its own once the row is gone.
func (h *CSHandler) DeleteAccount(c *gin.Context) {
	id, ok := pathUUID(c, "id", "INVALID_ACCOUNT_ID")
	if !ok {
		return
	}
	account, err := h.accounts.Get(id)
	if err != nil {
		mapCSError(c, err, "ACCOUNT_DELETE_FAILED")
		return
	}

	if err := h.purge.DeleteAccount(id); err != nil {
		mapCSError(c, err, "ACCOUNT_DELETE_FAILED")
		return
	}

	actorID, _ := middleware.GetUserID(c)
	old := map[string]interface{}{"label": account.Label, "jid": account.JID}
	if err := h.audit.Log(actorID, "delete", "wa_account", id, old, nil,
		c.ClientIP(), c.Request.UserAgent()); err != nil {
		h.logger.Warn("audit log for wa account deletion failed", zap.Error(err))
	}

	// Best effort by design: the number is already gone from the inbox, and
	// refusing to tell the admin that because a pub/sub message did not land
	// would be reporting a failure that did not happen.
	if err := h.publishControl(c, wa.ControlMessage{Action: wa.ControlDelete, AccountID: id.String()}); err != nil {
		h.logger.Warn("Could not ask the WhatsApp process to give up a deleted pairing",
			zap.String("account_id", id.String()), zap.Error(err))
	}

	h.announceInboxChanged(c)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}

// mayPurgeThread reports whether the caller may remove messages from a thread,
// answering the caller directly when they may not.
//
// An admin may clear any thread; everyone else may clear only one they hold.
// That is the same gate as replying, on purpose: the CS working a thread is
// the one who can tell a mistyped message from the customer's own words.
func (h *CSHandler) mayPurgeThread(c *gin.Context, convID uuid.UUID, code string) bool {
	if role, ok := middleware.GetUserRole(c); ok && role == models.UserRoleAdmin {
		return true
	}
	userID, _ := middleware.GetUserID(c)
	if err := h.conversations.EnsureHolder(convID, userID); err != nil {
		h.refuseNotHolder(c, convID, err, code)
		return false
	}
	return true
}

// announceInboxChanged wakes every CS browser after a purge that was not about
// one thread. The event names no conversation because the change is not in one:
// a browser hearing it refetches the inbox rather than a thread.
func (h *CSHandler) announceInboxChanged(c *gin.Context) {
	h.announceEvent(c.Request.Context(), wa.Event{Type: wa.EventStatus})
}

// auditPurge records a removal that cannot be undone. A failed write is logged
// and does not fail the request: the messages are already gone, and refusing to
// say so would be worse than an audit trail with one gap.
func (h *CSHandler) auditPurge(c *gin.Context, action, resource string, id uuid.UUID, removed int) {
	actorID, _ := middleware.GetUserID(c)
	err := h.audit.Log(actorID, action, resource, id, nil,
		map[string]interface{}{"removed": removed}, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		h.logger.Warn("audit log for a CS purge failed",
			zap.String("action", action), zap.Error(err))
	}
}
