package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CSHandler serves the shared WhatsApp inbox: the conversation list, and the
// messages inside each thread.
type CSHandler struct {
	conversations *services.CSConversationService
	messages      *services.CSMessageService
	quickReplies  *services.CSQuickReplyService
	assignment    *services.CSAssignmentService
	presence      services.Presence
	audit         *services.AuditService
	publisher     *wa.Publisher
	redis         *redis.Client
	logger        *zap.Logger
	mediaRoot     string
}

// NewCSHandler constructs a CSHandler.
func NewCSHandler(
	conversations *services.CSConversationService,
	messages *services.CSMessageService,
	quickReplies *services.CSQuickReplyService,
	assignment *services.CSAssignmentService,
	presence services.Presence,
	audit *services.AuditService,
	publisher *wa.Publisher,
	redis *redis.Client,
	logger *zap.Logger,
	mediaRoot string,
) *CSHandler {
	return &CSHandler{
		conversations: conversations,
		messages:      messages,
		quickReplies:  quickReplies,
		assignment:    assignment,
		presence:      presence,
		audit:         audit,
		publisher:     publisher,
		redis:         redis,
		logger:        logger,
		mediaRoot:     mediaRoot,
	}
}

// mapCSError turns a service failure into the response the mapping table
// calls for: validation as 400, a missing row as 404, everything else as 500.
// services.ErrNotHolder is not handled here — the 409 it produces needs to
// name the holder, which only the caller that already loaded the conversation
// can do (see refuseNotHolder in cs_handler_messages.go).
func mapCSError(c *gin.Context, err error, code string) {
	if badRequest(c, err, code) {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "not found", Code: code})
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error(), Code: code})
}

// queryInt reads an integer query parameter, answering zero for one that is
// absent or unparseable — callers treat zero as "use the default".
func queryInt(c *gin.Context, name string) int {
	v, _ := strconv.Atoi(c.Query(name))
	return v
}

// ListConversations answers the inbox: everyone's threads by default, or one
// of the views a CS switches between — their own, unassigned, or closed.
func (h *CSHandler) ListConversations(c *gin.Context) {
	filter := services.ConversationFilter{
		Search: c.Query("search"),
		Limit:  queryInt(c, "limit"),
		Offset: queryInt(c, "offset"),
	}

	switch {
	case c.Query("mine") == "true":
		userID, _ := middleware.GetUserID(c)
		filter.Mine = &userID
	case c.Query("unassigned") == "true":
		filter.Unassigned = true
	case c.Query("closed") == "true":
		filter.Closed = true
	}

	rows, err := h.conversations.List(filter)
	if err != nil {
		mapCSError(c, err, "CONVERSATION_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// Assign hands a conversation to one CS, including a takeover of a thread
// someone else already holds. The handover is always audited: an assignment
// that changes with no trace is how blame lands on the wrong person.
func (h *CSHandler) Assign(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	var req AssignRequest
	if !bindJSON(c, &req) {
		return
	}

	before, err := h.conversations.Get(convID)
	if err != nil {
		mapCSError(c, err, "ASSIGN_FAILED")
		return
	}

	if err := h.conversations.Assign(convID, req.UserID); err != nil {
		mapCSError(c, err, "ASSIGN_FAILED")
		return
	}

	h.auditHandover(c, convID, before.AssignedUserID, req.UserID)

	after, err := h.conversations.Get(convID)
	if err != nil {
		mapCSError(c, err, "ASSIGN_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": after})
}

// auditHandover records who held a conversation before this assignment and
// who holds it now. A failed write is logged and does not fail the request —
// the assignment already committed, and refusing to tell the CS that would be
// worse than an audit trail with one gap.
func (h *CSHandler) auditHandover(c *gin.Context, convID uuid.UUID, before *uuid.UUID, after uuid.UUID) {
	actorID, _ := middleware.GetUserID(c)
	var oldValue map[string]interface{}
	if before != nil {
		oldValue = map[string]interface{}{"assigned_user_id": before.String()}
	}
	newValue := map[string]interface{}{"assigned_user_id": after.String()}

	if err := h.audit.Log(actorID, "assign", "cs_conversation", convID, oldValue, newValue,
		c.ClientIP(), c.Request.UserAgent()); err != nil {
		h.logger.Warn("audit log for conversation handover failed", zap.Error(err))
	}
}

// SetStatus closes a thread. Reopening one is not a manual action here: a
// customer writing again goes through FindOrCreate, and a CS picking a closed
// thread back up goes through Assign — both already put the conversation into
// the state the database's check constraint requires (open needs a holder,
// unassigned needs none), and this endpoint does not invent a third path.
func (h *CSHandler) SetStatus(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	var req SetStatusRequest
	if !bindJSON(c, &req) {
		return
	}

	if req.Status != models.ConversationClosed {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "only closing a thread is supported here; reopen it by assigning it to someone",
			Code:  "UNSUPPORTED_STATUS",
		})
		return
	}

	if err := h.conversations.Close(convID); err != nil {
		mapCSError(c, err, "SET_STATUS_FAILED")
		return
	}

	conv, err := h.conversations.Get(convID)
	if err != nil {
		mapCSError(c, err, "SET_STATUS_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": conv})
}

// LinkONT ties a thread to a subscriber's ONT, or unties it when the body
// names none.
func (h *CSHandler) LinkONT(c *gin.Context) {
	convID, ok := pathUUID(c, "id", "INVALID_CONVERSATION_ID")
	if !ok {
		return
	}
	var req LinkONTRequest
	if !bindJSON(c, &req) {
		return
	}

	if err := h.conversations.LinkONT(convID, req.ONTID); err != nil {
		mapCSError(c, err, "LINK_ONT_FAILED")
		return
	}

	conv, err := h.conversations.Get(convID)
	if err != nil {
		mapCSError(c, err, "LINK_ONT_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": conv})
}
