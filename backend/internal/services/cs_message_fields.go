package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"gorm.io/gorm"
)

// The column-level edits a stored message goes through, and the link preview
// fields that ride with it.

// inboundRow renders an arriving message as the row that will hold it. It runs
// on the caller's transaction because the quoted message it points at must be
// looked up in the same one.
func inboundRow(tx *gorm.DB, in InboundMessage) models.CSMessage {
	waID := in.WAMessageID
	row := models.CSMessage{
		ConversationID:     in.ConversationID,
		WAMessageID:        &waID,
		Direction:          models.MessageIn,
		Kind:               in.Kind,
		Body:               in.Body,
		PreviewURL:         previewURL(in.Preview),
		PreviewTitle:       previewTitle(in.Preview),
		PreviewDescription: previewDescription(in.Preview),
		PreviewThumbnail:   previewThumbnail(in.Preview),
		Status:             models.MessageDelivered,
		ReplyToID:          quotedRow(tx, in.ConversationID, in.ReplyToWAID),
		WATimestamp:        in.At,
	}
	applyMedia(&row, in.Media)
	return row
}

// bumpConversation runs on the caller's transaction, never on s.db: it is half
// of storing a message, and must not be able to commit or fail on its own.
func bumpConversation(tx *gorm.DB, conversationID uuid.UUID, at time.Time) error {
	err := tx.Model(&models.CSConversation{}).Where("id = ?", conversationID).
		Updates(map[string]any{
			"last_message_at":        at,
			"last_message_direction": models.MessageIn,
			"unread_count":           gorm.Expr("unread_count + 1"),
		}).Error
	if err != nil {
		return fmt.Errorf("bump conversation: %w", err)
	}
	return nil
}

func (s *CSMessageService) updateMessage(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.CSMessage{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update message: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func applyMedia(msg *models.CSMessage, media *MediaFile) {
	if media == nil {
		return
	}
	msg.MediaPath = media.Path
	msg.MediaMime = media.Mime
	msg.MediaFilename = media.Filename
	msg.MediaSize = media.Size
}

// The four accessors keep the nil check in one place rather than repeating it
// at each field of the row being built.

func previewURL(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.URL
}

func previewTitle(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.Title
}

func previewDescription(p *linkpreview.Preview) string {
	if p == nil {
		return ""
	}
	return p.Description
}

func previewThumbnail(p *linkpreview.Preview) []byte {
	if p == nil {
		return nil
	}
	return p.Thumbnail
}
