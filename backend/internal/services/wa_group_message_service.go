package services

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"gorm.io/gorm"
)

// groupPreviewRunes matches the varchar(255) the summary is stored in.
const groupPreviewRunes = 255

var (
	// ErrGroupQuoteNotInGroup is a quote of a message another group holds.
	ErrGroupQuoteNotInGroup = fmt.Errorf("%w: pesan yang dikutip bukan milik grup ini", ErrValidation)
	// ErrGroupQuoteNotSent is a quote of a message WhatsApp has not seen yet.
	ErrGroupQuoteNotSent = fmt.Errorf("%w: pesan yang dikutip belum terkirim ke WhatsApp", ErrValidation)
)

// GroupArrival is one message WhatsApp delivered in a group, from a member or
// from the paired number's own phone.
type GroupArrival struct {
	GroupID     uuid.UUID
	WAMessageID string
	Direction   models.MessageDirection
	SenderJID   string
	SenderPhone string
	SenderName  string
	// SenderPushName is the profile name the sender's phone attached.
	SenderPushName string
	Kind           models.MessageKind
	Body           string
	Media          *MediaFile
	ReplyToWAID    string
	Preview        *linkpreview.Preview
	At             time.Time
	// Mentions are the member JIDs the message tags.
	Mentions []string
	// Extra is the JSON a location, contact card or poll carries.
	Extra     string
	Forwarded bool
}

// GroupOutgoing is one message a TikMan user writes to a group.
type GroupOutgoing struct {
	GroupID      uuid.UUID
	SenderUserID uuid.UUID
	SenderName   string
	Kind         models.MessageKind
	Body         string
	Media        *MediaFile
	ReplyToID    *uuid.UUID
	// Mentions are member JIDs the body tags as "@<address>".
	Mentions []string
	// Extra is a poll's question and options, as JSON.
	Extra     string
	Forwarded bool
	// MemberCount is who the message reaches: everyone else in the group.
	MemberCount int
}

// WAGroupMessageService stores group traffic in both directions, and is the
// group outbox.
type WAGroupMessageService struct {
	db *gorm.DB
}

// NewWAGroupMessageService constructs a WAGroupMessageService.
func NewWAGroupMessageService(db *gorm.DB) *WAGroupMessageService {
	return &WAGroupMessageService{db: db}
}

// SaveArrived stores a delivered message, answering false when it was already
// stored: WhatsApp re-delivers on every reconnect. The lookup stands in for
// migration 68's unique index on SQLite; on Postgres a race past the lookup is
// refused by the index and read back as the same "already stored".
func (s *WAGroupMessageService) SaveArrived(in GroupArrival) (*models.WAGroupMessage, bool, error) {
	var stored models.WAGroupMessage
	created := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("group_id = ? AND wa_message_id = ?", in.GroupID, in.WAMessageID).First(&stored).Error
		if lookup == nil {
			return nil
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return fmt.Errorf("look for existing group message: %w", lookup)
		}

		stored = arrivedGroupRow(tx, in)
		if err := tx.Create(&stored).Error; err != nil {
			return fmt.Errorf("store group message: %w", err)
		}
		created = true
		unread := in.Direction == models.MessageIn && in.Kind != models.MessageKindSystem
		return bumpGroup(tx, &stored, stored.Body, unread)
	})
	if err != nil && !created {
		var existing models.WAGroupMessage
		if lookup := s.db.Where("group_id = ? AND wa_message_id = ?", in.GroupID, in.WAMessageID).
			First(&existing).Error; lookup == nil {
			return &existing, false, nil
		}
	}
	if err != nil {
		return nil, false, err
	}
	return &stored, created, nil
}

func arrivedGroupRow(tx *gorm.DB, in GroupArrival) models.WAGroupMessage {
	waID := in.WAMessageID
	status := models.MessageDelivered
	if in.Direction == models.MessageOut {
		status = models.MessageSent
	}
	row := models.WAGroupMessage{
		GroupID:            in.GroupID,
		WAMessageID:        &waID,
		Direction:          in.Direction,
		SenderJID:          in.SenderJID,
		SenderPhone:        in.SenderPhone,
		SenderName:         in.SenderName,
		SenderPushName:     in.SenderPushName,
		Kind:               in.Kind,
		Extra:              in.Extra,
		Forwarded:          in.Forwarded,
		Mentions:           encodeJIDs(in.Mentions),
		Body:               in.Body,
		Status:             status,
		ReplyToID:          quotedGroupRow(tx, in.GroupID, in.ReplyToWAID),
		PreviewURL:         previewURL(in.Preview),
		PreviewTitle:       previewTitle(in.Preview),
		PreviewDescription: previewDescription(in.Preview),
		PreviewThumbnail:   previewThumbnail(in.Preview),
		WATimestamp:        in.At,
	}
	applyGroupMedia(&row, in.Media)
	return row
}

// quotedGroupRow answers nil for a quote this group does not hold, for the
// reason quotedRow does: the message matters more than the grey block.
func quotedGroupRow(tx *gorm.DB, groupID uuid.UUID, waMessageID string) *uuid.UUID {
	if waMessageID == "" {
		return nil
	}
	var quoted models.WAGroupMessage
	if err := tx.Select("id").Where("group_id = ? AND wa_message_id = ?", groupID, waMessageID).
		First(&quoted).Error; err != nil {
		return nil
	}
	return &quoted.ID
}

func applyGroupMedia(row *models.WAGroupMessage, media *MediaFile) {
	if media == nil {
		return
	}
	row.MediaPath = media.Path
	row.MediaMime = media.Mime
	row.MediaFilename = media.Filename
	row.MediaSize = media.Size
}

// bumpGroup brings the group's summary up to date on the caller's transaction,
// so the list never shows a message that failed to store, or misses one that
// did. Only a message newer than the summary moves it: history fetched from
// the phone arrives oldest last, and must neither replace the latest message
// nor count as unread.
func bumpGroup(tx *gorm.DB, msg *models.WAGroupMessage, preview string, unread bool) error {
	sender := msg.SenderName
	if sender == "" && msg.SenderPushName != "" {
		sender = "~" + msg.SenderPushName
	}
	if sender == "" {
		sender = msg.SenderPhone
	}
	fields := map[string]any{
		"last_message_at":        msg.WATimestamp,
		"last_message_preview":   clampRunes(preview, groupPreviewRunes),
		"last_message_kind":      msg.Kind,
		"last_message_sender":    clampRunes(sender, groupPreviewRunes),
		"last_message_direction": msg.Direction,
	}
	if unread {
		fields["unread_count"] = gorm.Expr("unread_count + 1")
	}
	err := tx.Model(&models.WAGroup{}).
		Where("id = ? AND (last_message_at IS NULL OR last_message_at <= ?)", msg.GroupID, msg.WATimestamp).
		Updates(fields).Error
	if err != nil {
		return fmt.Errorf("bump group: %w", err)
	}
	return nil
}

func clampRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}

// Queue writes a message as waiting to be sent.
func (s *WAGroupMessageService) Queue(out GroupOutgoing) (*models.WAGroupMessage, error) {
	sender := out.SenderUserID
	msg := models.WAGroupMessage{
		GroupID:      out.GroupID,
		Direction:    models.MessageOut,
		SenderName:   out.SenderName,
		SenderUserID: &sender,
		Kind:         out.Kind,
		Body:         out.Body,
		Status:       models.MessageQueued,
		ReplyToID:    out.ReplyToID,
		Extra:        out.Extra,
		Forwarded:    out.Forwarded,
		MemberCount:  out.MemberCount,
		Mentions:     encodeJIDs(out.Mentions),
		WATimestamp:  time.Now(),
	}
	applyGroupMedia(&msg, out.Media)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&msg).Error; err != nil {
			return fmt.Errorf("queue group message: %w", err)
		}
		return bumpGroup(tx, &msg, withNames(tx, msg.GroupID, msg.Body, out.Mentions), false)
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// QuoteTarget answers the message a reply may quote: one in this group that
// WhatsApp already has.
func (s *WAGroupMessageService) QuoteTarget(groupID, messageID uuid.UUID) (*models.WAGroupMessage, error) {
	var target models.WAGroupMessage
	err := s.db.Where("id = ? AND group_id = ?", messageID, groupID).First(&target).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGroupQuoteNotInGroup
	}
	if err != nil {
		return nil, fmt.Errorf("load quoted group message: %w", err)
	}
	if target.WAMessageID == nil {
		return nil, ErrGroupQuoteNotSent
	}
	return &target, nil
}

// History answers one page of a group, newest first, with quotes resolved.
func (s *WAGroupMessageService) History(groupID uuid.UUID, limit, offset int) ([]models.WAGroupMessage, error) {
	if limit <= 0 || limit > defaultHistoryLimit {
		limit = defaultHistoryLimit
	}
	var rows []models.WAGroupMessage
	err := s.db.Where("group_id = ?", groupID).
		Order("wa_timestamp DESC").Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load group history: %w", err)
	}
	if err := s.attachQuotes(rows); err != nil {
		return nil, err
	}
	return rows, s.decorate(groupID, rows)
}

// attachQuotes fills in every quoted message on a page in one query.
func (s *WAGroupMessageService) attachQuotes(rows []models.WAGroupMessage) error {
	wanted := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		if row.ReplyToID != nil {
			wanted = append(wanted, *row.ReplyToID)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	var quoted []models.WAGroupMessage
	if err := s.db.Where("id IN ?", wanted).Find(&quoted).Error; err != nil {
		return fmt.Errorf("load quoted group messages: %w", err)
	}
	byID := make(map[uuid.UUID]*models.WAGroupMessage, len(quoted))
	for i := range quoted {
		byID[quoted[i].ID] = &quoted[i]
	}
	for i := range rows {
		if rows[i].ReplyToID == nil {
			continue
		}
		if target, ok := byID[*rows[i].ReplyToID]; ok {
			rows[i].ReplyTo = target.AsQuote()
		}
	}
	return nil
}

// Get answers one message.
func (s *WAGroupMessageService) Get(id uuid.UUID) (*models.WAGroupMessage, error) {
	var msg models.WAGroupMessage
	if err := s.db.First(&msg, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}
