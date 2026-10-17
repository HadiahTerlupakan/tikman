package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// editWindow is how long after sending a message may still be edited.
// WhatsApp allows twenty minutes; fifteen leaves the queue time to deliver
// the edit before WhatsApp would refuse it.
const editWindow = 15 * time.Minute

// ReactionRequest is an emoji on a message; "" takes ours away.
type ReactionRequest struct {
	Emoji string `json:"emoji"`
}

// EditMessageRequest is the new words of one of our messages.
type EditMessageRequest struct {
	Body string `json:"body" binding:"required"`
}

// VoteRequest is a choice in a poll; none withdraws the vote.
type VoteRequest struct {
	Options []string `json:"options"`
}

// loadMessage reads the group and the message the path names, refusing a
// message from another group so a URL cannot reach across.
func (h *CSGroupHandler) loadMessage(c *gin.Context, code string) (*models.WAGroup, *models.WAGroupMessage, bool) {
	group, ok := h.loadGroup(c, code)
	if !ok {
		return nil, nil, false
	}
	msgID, ok := pathUUID(c, "message_id", "INVALID_MESSAGE_ID")
	if !ok {
		return nil, nil, false
	}
	msg, err := h.messages.Get(msgID)
	if err != nil || msg.GroupID != group.ID {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "pesan tidak ditemukan di grup ini", Code: code})
		return nil, nil, false
	}
	return group, msg, true
}

// actOnMessage loads the message, lets check refuse it, and queues the
// action for the wa process.
func (h *CSGroupHandler) actOnMessage(
	c *gin.Context, kind models.GroupActionKind, args services.GroupActionArgs,
	check func(*models.WAGroup, *models.WAGroupMessage) bool,
) {
	code := "GROUP_MESSAGE_ACTION_FAILED"
	group, msg, ok := h.loadMessage(c, code)
	if !ok {
		return
	}
	if group.Left {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Nomor ini sudah tidak menjadi anggota grup", Code: "GROUP_LEFT"})
		return
	}
	if msg.Deleted || msg.Kind == models.MessageKindSystem {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Pesan ini tidak bisa diubah", Code: code})
		return
	}
	if check != nil && !check(group, msg) {
		return
	}
	args.MessageID = msg.ID.String()
	userID, _ := middleware.GetUserID(c)
	h.queueAction(c, services.GroupActionRequest{
		AccountID: group.WAAccountID, GroupID: &group.ID, Action: kind, Args: args, UserID: userID,
	}, code)
}

// ReactToMessage puts our emoji on a message, or takes it off.
func (h *CSGroupHandler) ReactToMessage(c *gin.Context) {
	var req ReactionRequest
	if !bindJSON(c, &req) {
		return
	}
	h.actOnMessage(c, models.GroupActionReact, services.GroupActionArgs{Emoji: req.Emoji}, nil)
}

// EditGroupMessage replaces the words of one of our recent text messages.
func (h *CSGroupHandler) EditGroupMessage(c *gin.Context) {
	var req EditMessageRequest
	if !bindJSON(c, &req) {
		return
	}
	userID, _ := middleware.GetUserID(c)
	h.actOnMessage(c, models.GroupActionEdit, services.GroupActionArgs{Body: req.Body},
		func(_ *models.WAGroup, msg *models.WAGroupMessage) bool {
			switch {
			// Only its writer: a message carries its writer's initials, and
			// anyone else changing its words would be speaking in their name.
			case !writtenBy(msg, userID) || msg.Kind != models.MessageKindText:
				c.JSON(http.StatusConflict, ErrorResponse{Error: "Hanya pesan teks yang Anda tulis sendiri yang bisa diedit", Code: "NOT_OURS"})
				return false
			case time.Since(msg.WATimestamp) > editWindow:
				c.JSON(http.StatusConflict, ErrorResponse{Error: "Pesan ini sudah terlalu lama untuk diedit", Code: "EDIT_WINDOW"})
				return false
			}
			return true
		})
}

// DeleteGroupMessage deletes a message for everyone. Its writer may; an
// admin may delete any the number sent, and a member's only where the number
// administers the group, which is what WhatsApp itself requires.
func (h *CSGroupHandler) DeleteGroupMessage(c *gin.Context) {
	role, _ := middleware.GetUserRole(c)
	userID, _ := middleware.GetUserID(c)
	h.actOnMessage(c, models.GroupActionRevoke, services.GroupActionArgs{},
		func(group *models.WAGroup, msg *models.WAGroupMessage) bool {
			admin := role == models.UserRoleAdmin
			ours := msg.Direction == models.MessageOut
			if writtenBy(msg, userID) || (admin && ours) || (admin && group.IsAdmin) {
				return true
			}
			c.JSON(http.StatusForbidden, ErrorResponse{
				Error: "Pesan orang lain hanya bisa dihapus Admin", Code: "FORBIDDEN",
			})
			return false
		})
}

// writtenBy says whether this TikMan user wrote the message. One typed on
// the phone holding the number has no writer here and belongs to nobody.
func writtenBy(msg *models.WAGroupMessage, userID uuid.UUID) bool {
	return msg.Direction == models.MessageOut && msg.SenderUserID != nil && *msg.SenderUserID == userID
}

// VoteInPoll answers a poll.
func (h *CSGroupHandler) VoteInPoll(c *gin.Context) {
	var req VoteRequest
	if !bindJSON(c, &req) {
		return
	}
	h.actOnMessage(c, models.GroupActionVote, services.GroupActionArgs{Options: req.Options},
		func(_ *models.WAGroup, msg *models.WAGroupMessage) bool {
			if msg.Kind != models.MessageKindPoll {
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Pesan ini bukan polling", Code: "NOT_A_POLL"})
				return false
			}
			return h.validVote(c, msg, req.Options)
		})
}

func (h *CSGroupHandler) validVote(c *gin.Context, msg *models.WAGroupMessage, options []string) bool {
	poll, err := services.DecodePoll(msg)
	if err != nil {
		mapCSError(c, err, "VOTE_FAILED")
		return false
	}
	known := make(map[string]bool, len(poll.Options))
	for _, option := range poll.Options {
		known[option.Name] = true
	}
	for _, option := range options {
		if !known[option] {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Pilihan tidak ada di polling ini", Code: "VOTE_FAILED"})
			return false
		}
	}
	if poll.SelectableCount > 0 && len(options) > poll.SelectableCount {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Terlalu banyak pilihan untuk polling ini", Code: "VOTE_FAILED"})
		return false
	}
	return true
}
