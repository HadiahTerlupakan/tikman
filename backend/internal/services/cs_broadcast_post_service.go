package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// defaultPostHistoryLimit is one screen of broadcast history.
const defaultPostHistoryLimit = 50

// BroadcastPost is one announcement as the caller supplies it, before it is a
// row. ChannelJID is empty for a status.
type BroadcastPost struct {
	WAAccountID  uuid.UUID
	Destination  models.BroadcastDestination
	ChannelJID   string
	SenderUserID uuid.UUID
	Kind         models.MessageKind
	Body         string
	Media        *MediaFile
}

// CSBroadcastPostService is the outbox and the history of announcements.
// They are one table: a queued row is what the drainer claims, and the same
// row is what the sender reads the outcome from afterwards.
type CSBroadcastPostService struct {
	db *gorm.DB
}

// NewCSBroadcastPostService constructs a CSBroadcastPostService.
func NewCSBroadcastPostService(db *gorm.DB) *CSBroadcastPostService {
	return &CSBroadcastPostService{db: db}
}

// buildBroadcastPost validates one announcement and turns it into a row ready
// to insert, without touching the database. Queue and QueueAll both go
// through this, so the two never drift apart on what makes a row valid.
//
// The destination and the channel are checked against each other here as well
// as by the database, because the unit suite runs on SQLite, which has none of
// migration 49's constraints — and a contradictory row would otherwise only
// fail at the drainer, hours later, as a failure nobody could act on.
func buildBroadcastPost(in BroadcastPost) (*models.WABroadcastPost, error) {
	post := models.WABroadcastPost{
		WAAccountID:  in.WAAccountID,
		Destination:  in.Destination,
		SenderUserID: in.SenderUserID,
		Kind:         in.Kind,
		Body:         in.Body,
		Status:       models.BroadcastQueued,
	}

	switch in.Destination {
	case models.DestinationChannel:
		if in.ChannelJID == "" {
			return nil, fmt.Errorf("kiriman saluran harus menyebut salurannya")
		}
		jid := in.ChannelJID
		post.DestinationJID = &jid
	case models.DestinationStatus:
		if in.ChannelJID != "" {
			return nil, fmt.Errorf("status tidak menyebut saluran")
		}
	default:
		return nil, fmt.Errorf("tujuan tidak dikenal %q", in.Destination)
	}

	if in.Media != nil {
		post.MediaPath = in.Media.Path
		post.MediaMime = in.Media.Mime
		post.MediaFilename = in.Media.Filename
		post.MediaSize = in.Media.Size
	}
	return &post, nil
}

// Queue writes an announcement as waiting to be sent. The wa process claims it,
// and one written while that process was down is still here when it comes back.
func (s *CSBroadcastPostService) Queue(in BroadcastPost) (*models.WABroadcastPost, error) {
	post, err := buildBroadcastPost(in)
	if err != nil {
		return nil, err
	}
	if err := s.db.Create(post).Error; err != nil {
		return nil, fmt.Errorf("queue broadcast post: %w", err)
	}
	return post, nil
}

// QueueAll writes every announcement in the request as one transaction, so a
// request naming several destinations lands as all of them or none.
//
// One announcement is one act, even when it reaches more than one place. A
// request that half-succeeded — one row committed, the next failing on a DB
// blip, a full disk, or a bad row — would tell the sender nothing went out
// while something in fact did, which is worse than telling them nothing sent
// at all: on the media path in particular, the caller deletes the upload on
// any error from this call, and a row left committed after that delete would
// name a file that no longer exists, sitting in the outbox unsendable and
// invisible to anyone but a drainer failure nobody reads.
func (s *CSBroadcastPostService) QueueAll(posts []BroadcastPost) ([]models.WABroadcastPost, error) {
	rows := make([]models.WABroadcastPost, 0, len(posts))
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, in := range posts {
			post, err := buildBroadcastPost(in)
			if err != nil {
				return err
			}
			if err := tx.Create(post).Error; err != nil {
				return fmt.Errorf("queue broadcast post: %w", err)
			}
			rows = append(rows, *post)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListRecent returns the latest announcements across every destination,
// newest first. Not filtered by channel: one action can now reach two places,
// and a per-channel history would hide half of what was just sent.
func (s *CSBroadcastPostService) ListRecent(limit int) ([]models.WABroadcastPost, error) {
	if limit <= 0 || limit > defaultPostHistoryLimit {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WABroadcastPost
	err := s.db.Order("created_at DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list broadcast posts: %w", err)
	}
	return rows, nil
}

// ClaimQueued returns the updates still waiting on one number, oldest first.
//
// Scoped to the account for the same reason the message outbox is: the session
// holding a number claims only its own work, so an update leaves through the
// number that actually administers the channel.
func (s *CSBroadcastPostService) ClaimQueued(accountID uuid.UUID, limit int) ([]models.WABroadcastPost, error) {
	if limit <= 0 {
		limit = defaultPostHistoryLimit
	}
	var rows []models.WABroadcastPost
	err := s.db.Where("status = ? AND wa_account_id = ?", models.BroadcastQueued, accountID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued broadcast posts: %w", err)
	}
	return rows, nil
}

// MarkSent records that WhatsApp accepted an update, and clears any reason a
// previous attempt left behind.
func (s *CSBroadcastPostService) MarkSent(id uuid.UUID, waMessageID string) error {
	sentAt := time.Now()
	return s.update(id, map[string]any{
		"status":        models.BroadcastSent,
		"wa_message_id": waMessageID,
		"fail_reason":   "",
		"sent_at":       sentAt,
	})
}

// MarkFailed records why an update could not be sent, so the sender reads a
// reason rather than watching an announcement quietly disappear.
func (s *CSBroadcastPostService) MarkFailed(id uuid.UUID, reason string) error {
	return s.update(id, map[string]any{
		"status":      models.BroadcastFailed,
		"fail_reason": reason,
	})
}

func (s *CSBroadcastPostService) update(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.WABroadcastPost{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update broadcast post: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
