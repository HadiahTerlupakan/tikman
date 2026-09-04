package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// defaultConversationLimit keeps one inbox page bounded; at thousands of chats
// a day an unbounded list would fetch a year of history to draw one screen.
const defaultConversationLimit = 50

// LastMessage is the one line a CS reads to decide whether to open a thread.
type LastMessage struct {
	Body      string                  `json:"body"`
	Kind      models.MessageKind      `json:"kind"`
	Direction models.MessageDirection `json:"direction"`
	At        time.Time               `json:"at"`
}

// ConversationSummary is a thread as the inbox list shows it. The conversation
// is embedded, so a caller that only wants the thread reads it unchanged.
type ConversationSummary struct {
	models.CSConversation
	LastMessage *LastMessage `json:"last_message,omitempty"`
	// WAAccountLabel names the number this thread came in on. With one number
	// it is noise; with several it is the difference between a CS knowing
	// which of your numbers a customer is talking to and guessing.
	WAAccountLabel string `json:"wa_account_label,omitempty"`
}

// ConversationFilter narrows the inbox to one of the views a CS switches
// between: their own threads, the ones nobody holds, or the finished ones.
type ConversationFilter struct {
	Mine *uuid.UUID
	// AwaitingReply is every thread whose last message came from the customer,
	// whoever holds it. One rule covers both the chat nobody has answered yet
	// and the customer who wrote again after theirs was closed.
	AwaitingReply bool
	Closed        bool
	Search        string
	Limit         int
	Offset        int
}

// List draws one page of the inbox, newest first.
func (s *CSConversationService) List(f ConversationFilter) ([]ConversationSummary, error) {
	q := s.db.Model(&models.CSConversation{})

	switch {
	case f.Mine != nil:
		q = q.Where("assigned_user_id = ? AND status <> ?", *f.Mine, models.ConversationClosed)
	case f.AwaitingReply:
		q = q.Where("last_message_direction = ?", models.MessageIn)
	case f.Closed:
		q = q.Where("status = ?", models.ConversationClosed)
	}

	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where("customer_name LIKE ? OR customer_phone LIKE ?", like, like)
	}

	limit := f.Limit
	if limit <= 0 || limit > defaultConversationLimit {
		limit = defaultConversationLimit
	}

	var rows []models.CSConversation
	if err := q.Order("last_message_at DESC").Limit(limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	summaries := make([]ConversationSummary, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		summaries[i] = ConversationSummary{CSConversation: row}
		ids[i] = row.ID
	}

	latest, err := s.lastMessages(ids)
	if err != nil {
		return nil, err
	}
	labels, err := s.accountLabels()
	if err != nil {
		return nil, err
	}
	for i := range summaries {
		if m, ok := latest[summaries[i].ID]; ok {
			summaries[i].LastMessage = m
		}
		summaries[i].WAAccountLabel = labels[summaries[i].WAAccountID]
	}
	return summaries, nil
}

// accountLabels reads every number's name in one query. The table holds one
// row per CS number — a handful — so there is nothing to gain from narrowing
// it to the page, and a join would repeat the label on every row instead.
func (s *CSConversationService) accountLabels() (map[uuid.UUID]string, error) {
	var rows []models.WAAccount
	if err := s.db.Select("id", "label").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("read wa account labels: %w", err)
	}
	labels := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		labels[row.ID] = row.Label
	}
	return labels, nil
}

// lastMessages fetches the newest message of each thread in one query. One
// query rather than one per row: an inbox page is fifty threads, and fifty
// round trips to render a list is how a list stops being worth rendering.
func (s *CSConversationService) lastMessages(ids []uuid.UUID) (map[uuid.UUID]*LastMessage, error) {
	latest := map[uuid.UUID]*LastMessage{}
	if len(ids) == 0 {
		return latest, nil
	}

	type row struct {
		ConversationID uuid.UUID
		Body           string
		Kind           models.MessageKind
		Direction      models.MessageDirection
		WATimestamp    time.Time
	}
	var found []row
	err := s.db.Raw(`
		SELECT conversation_id, body, kind, direction, wa_timestamp
		FROM (
			SELECT conversation_id, body, kind, direction, wa_timestamp,
			       ROW_NUMBER() OVER (
			           PARTITION BY conversation_id
			           ORDER BY wa_timestamp DESC, id DESC
			       ) AS rn
			FROM cs_messages
			WHERE conversation_id IN ?
		) ranked
		WHERE rn = 1`, ids).Scan(&found).Error
	if err != nil {
		return nil, fmt.Errorf("load last messages: %w", err)
	}

	for _, r := range found {
		latest[r.ConversationID] = &LastMessage{
			Body: r.Body, Kind: r.Kind, Direction: r.Direction, At: r.WATimestamp,
		}
	}
	return latest, nil
}
