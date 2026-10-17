package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/middleware"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"github.com/tikman/olt-provisioning/internal/wa"
)

// StarRequest stars a message for the team or takes the star away.
type StarRequest struct {
	Starred bool `json:"starred"`
}

// ForwardRequest names the groups a message is forwarded to.
type ForwardRequest struct {
	GroupIDs []string `json:"group_ids" binding:"required,min=1"`
}

// CreatePollRequest is a poll a user writes.
type CreatePollRequest struct {
	Question        string   `json:"question" binding:"required"`
	Options         []string `json:"options" binding:"required"`
	SelectableCount int      `json:"selectable_count"`
}

// StarGroupMessage stars or unstars a message.
func (h *CSGroupHandler) StarGroupMessage(c *gin.Context) {
	_, msg, ok := h.loadMessage(c, "STAR_FAILED")
	if !ok {
		return
	}
	var req StarRequest
	if !bindJSON(c, &req) {
		return
	}
	starred, err := h.messages.SetStarred(msg.ID, req.Starred)
	if err != nil {
		mapCSError(c, err, "STAR_FAILED")
		return
	}
	h.inbox.announceEvent(c.Request.Context(), wa.Event{
		Type: wa.EventGroupMessage, GroupID: msg.GroupID.String(), MessageID: msg.ID.String(),
	})
	c.JSON(http.StatusOK, gin.H{"data": starred})
}

// StarredGroupMessages answers a group's starred messages.
func (h *CSGroupHandler) StarredGroupMessages(c *gin.Context) {
	group, ok := h.loadGroup(c, "STARRED_FAILED")
	if !ok {
		return
	}
	rows, err := h.messages.Starred(group.ID)
	if err != nil {
		mapCSError(c, err, "STARRED_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// MessageReceipts answers who got and read one of our messages.
func (h *CSGroupHandler) MessageReceipts(c *gin.Context) {
	_, msg, ok := h.loadMessage(c, "RECEIPTS_FAILED")
	if !ok {
		return
	}
	if msg.Direction != models.MessageOut {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Info pesan hanya untuk pesan kita sendiri", Code: "NOT_OURS"})
		return
	}
	rows, err := h.messages.Receipts(msg.ID)
	if err != nil {
		mapCSError(c, err, "RECEIPTS_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// RequestGroupHistory asks the phone for the messages before the oldest one
// stored. It needs one to count back from.
func (h *CSGroupHandler) RequestGroupHistory(c *gin.Context) {
	group, ok := h.sendableGroup(c, "HISTORY_FAILED")
	if !ok {
		return
	}
	if _, err := h.messages.OldestAnchor(group.ID); err != nil {
		if errors.Is(err, services.ErrNoHistoryAnchor) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error(), Code: "NO_ANCHOR"})
			return
		}
		mapCSError(c, err, "HISTORY_FAILED")
		return
	}
	userID, _ := middleware.GetUserID(c)
	h.queueAction(c, services.GroupActionRequest{
		AccountID: group.WAAccountID, GroupID: &group.ID, Action: models.GroupActionHistory, UserID: userID,
	}, "HISTORY_FAILED")
}

// CreateGroupPoll queues a poll to a group.
func (h *CSGroupHandler) CreateGroupPoll(c *gin.Context) {
	group, ok := h.sendableGroup(c, "POLL_FAILED")
	if !ok {
		return
	}
	var req CreatePollRequest
	if !bindJSON(c, &req) {
		return
	}
	extra, err := services.NewPollExtra(req.Question, req.Options, req.SelectableCount)
	if err != nil {
		mapCSError(c, err, "POLL_FAILED")
		return
	}
	userID, _ := middleware.GetUserID(c)
	msg, err := h.messages.Queue(services.GroupOutgoing{
		GroupID: group.ID, SenderUserID: userID, SenderName: h.senderName(userID),
		Kind: models.MessageKindPoll, Body: "Polling: " + req.Question, Extra: extra,
		MemberCount: h.memberCount(group.ID),
	})
	if err != nil {
		mapCSError(c, err, "POLL_FAILED")
		return
	}
	h.queued(c.Request.Context(), msg)
	c.JSON(http.StatusCreated, gin.H{"data": msg})
}

// ReplyPrivately opens the sender's own thread in the customer inbox, the
// way "Balas secara pribadi" opens a one-to-one chat on a phone.
func (h *CSGroupHandler) ReplyPrivately(c *gin.Context) {
	group, msg, ok := h.loadMessage(c, "REPLY_PRIVATE_FAILED")
	if !ok {
		return
	}
	if msg.Direction != models.MessageIn || msg.Kind == models.MessageKindSystem || msg.SenderJID == "" {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Pesan ini tidak punya pengirim untuk dibalas", Code: "REPLY_PRIVATE_FAILED"})
		return
	}
	jid, phone := msg.SenderJID, msg.SenderPhone
	if phone != "" {
		jid = phone + "@s.whatsapp.net"
	}
	name := msg.SenderName
	if name == "" {
		name = msg.SenderPushName
	}
	conv, err := h.inbox.conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: group.WAAccountID, JID: jid, Phone: phone, Name: name,
	})
	if err != nil {
		mapCSError(c, err, "REPLY_PRIVATE_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"conversation_id": conv.ID}})
}

// ForwardGroupMessage copies a message into other groups' outboxes, marked
// forwarded the way WhatsApp marks it. Every target is checked first so a
// request lands everywhere or nowhere it was refused.
func (h *CSGroupHandler) ForwardGroupMessage(c *gin.Context) {
	_, msg, ok := h.loadMessage(c, "FORWARD_FAILED")
	if !ok {
		return
	}
	var req ForwardRequest
	if !bindJSON(c, &req) {
		return
	}
	if !forwardable(msg) {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Pesan ini tidak bisa diteruskan", Code: "FORWARD_FAILED"})
		return
	}
	targets, ok := h.forwardTargets(c, req.GroupIDs)
	if !ok {
		return
	}
	userID, _ := middleware.GetUserID(c)
	sent := make([]*models.WAGroupMessage, 0, len(targets))
	for _, target := range targets {
		queued, err := h.forwardTo(target, msg, userID)
		if err != nil {
			mapCSError(c, err, "FORWARD_FAILED")
			return
		}
		h.queued(c.Request.Context(), queued)
		sent = append(sent, queued)
	}
	c.JSON(http.StatusCreated, gin.H{"data": sent})
}

func forwardable(msg *models.WAGroupMessage) bool {
	if msg.Deleted || msg.WAMessageID == nil {
		return false
	}
	switch msg.Kind {
	case models.MessageKindSystem, models.MessageKindPoll, models.MessageKindLocation, models.MessageKindContact:
		return false
	}
	return msg.Kind == models.MessageKindText || msg.MediaPath != ""
}

func (h *CSGroupHandler) forwardTargets(c *gin.Context, ids []string) ([]*models.WAGroup, bool) {
	targets := make([]*models.WAGroup, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid ID format", Code: "INVALID_GROUP_ID"})
			return nil, false
		}
		group, err := h.groups.Get(id)
		if err != nil {
			mapCSError(c, err, "FORWARD_FAILED")
			return nil, false
		}
		if group.Left || (group.Announce && !group.IsAdmin) {
			c.JSON(http.StatusConflict, ErrorResponse{
				Error: fmt.Sprintf("Nomor ini tidak bisa mengirim ke grup %q", group.Name), Code: "FORWARD_FAILED",
			})
			return nil, false
		}
		targets = append(targets, group)
	}
	return targets, true
}

// forwardTo queues one copy. The attachment is copied too: each row owns its
// file, and the retention sweep or a deletion of one must not take another's.
func (h *CSGroupHandler) forwardTo(target *models.WAGroup, msg *models.WAGroupMessage, userID uuid.UUID) (*models.WAGroupMessage, error) {
	var media *services.MediaFile
	if msg.MediaPath != "" {
		copied, err := h.copyMedia(msg)
		if err != nil {
			return nil, err
		}
		media = copied
	}
	queued, err := h.messages.Queue(services.GroupOutgoing{
		GroupID: target.ID, SenderUserID: userID, SenderName: h.senderName(userID),
		Kind: msg.Kind, Body: msg.Body, Media: media, Forwarded: true, MemberCount: h.memberCount(target.ID),
	})
	if err != nil && media != nil {
		h.inbox.removeOrphanedUpload(media.Path)
	}
	return queued, err
}

func (h *CSGroupHandler) copyMedia(msg *models.WAGroupMessage) (*services.MediaFile, error) {
	src, ok := mediaPathWithin(h.inbox.mediaRoot, msg.MediaPath)
	if !ok {
		return nil, fmt.Errorf("lampiran tidak ditemukan")
	}
	rel := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"), uuid.NewString()+filepath.Ext(msg.MediaPath))
	dst := filepath.Join(h.inbox.mediaRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("lampiran sudah tidak tersedia: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, fmt.Errorf("create media file: %w", err)
	}
	defer out.Close()
	size, err := io.Copy(out, in)
	if err != nil {
		_ = os.Remove(dst)
		return nil, fmt.Errorf("copy media file: %w", err)
	}
	return &services.MediaFile{Path: rel, Mime: msg.MediaMime, Filename: msg.MediaFilename, Size: size}, nil
}

// memberCount is who a message reaches; a failure only costs the blue ticks.
func (h *CSGroupHandler) memberCount(groupID uuid.UUID) int {
	count, err := h.groups.MembersOtherThanUs(groupID)
	if err != nil {
		return 0
	}
	return count
}
