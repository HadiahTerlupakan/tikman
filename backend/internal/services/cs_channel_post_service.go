package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// defaultPostHistoryLimit is one screen of a channel's broadcast history.
const defaultPostHistoryLimit = 50

// ChannelPost is one update as the caller supplies it, before it is a row.
type ChannelPost struct {
	WAAccountID  uuid.UUID
	ChannelJID   string
	SenderUserID uuid.UUID
	Kind         models.MessageKind
	Body         string
	Media        *MediaFile
}

// CSChannelPostService is the outbox and the history of channel updates. They
// are one table: a queued row is what the drainer claims, and the same row is
// what the sender reads the outcome from afterwards.
type CSChannelPostService struct {
	db *gorm.DB
}

// NewCSChannelPostService constructs a CSChannelPostService.
func NewCSChannelPostService(db *gorm.DB) *CSChannelPostService {
	return &CSChannelPostService{db: db}
}

// Queue writes an update as waiting to be sent. The wa process claims it, and
// one written while that process was down is still here when it comes back.
func (s *CSChannelPostService) Queue(in ChannelPost) (*models.WAChannelPost, error) {
	post := models.WAChannelPost{
		WAAccountID:  in.WAAccountID,
		ChannelJID:   in.ChannelJID,
		SenderUserID: in.SenderUserID,
		Kind:         in.Kind,
		Body:         in.Body,
		Status:       models.ChannelPostQueued,
	}
	if in.Media != nil {
		post.MediaPath = in.Media.Path
		post.MediaMime = in.Media.Mime
		post.MediaFilename = in.Media.Filename
		post.MediaSize = in.Media.Size
	}
	if err := s.db.Create(&post).Error; err != nil {
		return nil, fmt.Errorf("queue channel post: %w", err)
	}
	return &post, nil
}

// ListFor returns one channel's broadcast history, newest first.
//
// Keyed by the JID rather than the wa_channels row id: the sync deletes and
// rebuilds those rows on every pass, and the record of what was announced
// must not disappear with them.
func (s *CSChannelPostService) ListFor(channelJID string, limit int) ([]models.WAChannelPost, error) {
	if limit <= 0 || limit > defaultPostHistoryLimit {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WAChannelPost
	err := s.db.Where("channel_jid = ?", channelJID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list channel posts: %w", err)
	}
	return rows, nil
}

// ClaimQueued returns the updates still waiting on one number, oldest first.
//
// Scoped to the account for the same reason the message outbox is: the session
// holding a number claims only its own work, so an update leaves through the
// number that actually administers the channel.
func (s *CSChannelPostService) ClaimQueued(accountID uuid.UUID, limit int) ([]models.WAChannelPost, error) {
	if limit <= 0 {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WAChannelPost
	err := s.db.Where("status = ? AND wa_account_id = ?", models.ChannelPostQueued, accountID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued channel posts: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted an update, and clears any reason a
// previous attempt left behind.
func (s *CSChannelPostService) MarkSent(id uuid.UUID, waMessageID string) error {
	sentAt := time.Now()
	return s.update(id, map[string]any{
		"status":        models.ChannelPostSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
		"sent_at":       sentAt,
	})
}

// MarkFailed records why an update could not be sent, so the sender reads a
// reason rather than watching an announcement quietly disappear.
func (s *CSChannelPostService) MarkFailed(id uuid.UUID, reason string) error {
	return s.update(id, map[string]any{
		"status":      models.ChannelPostFailed,
		"fail_reason": reason,
	})
}

func (s *CSChannelPostService) update(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.WAChannelPost{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update channel post: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
