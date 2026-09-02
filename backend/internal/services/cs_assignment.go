package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// CSAssignmentService shares incoming conversations round the agents who
// actually have the inbox open.
type CSAssignmentService struct {
	db            *gorm.DB
	conversations *CSConversationService
	presence      Presence
}

// NewCSAssignmentService constructs a CSAssignmentService.
func NewCSAssignmentService(db *gorm.DB, conversations *CSConversationService, presence Presence) *CSAssignmentService {
	return &CSAssignmentService{db: db, conversations: conversations, presence: presence}
}

// AssignOne hands one waiting conversation to the next agent in the rotation.
// It answers nil when nobody is online: the thread then waits rather than
// landing in the inbox of someone who went home, and AssignWaiting picks it up
// when the next shift opens the page.
func (s *CSAssignmentService) AssignOne(ctx context.Context, conversationID uuid.UUID) (*uuid.UUID, error) {
	conv, err := s.conversations.Get(conversationID)
	if err != nil {
		return nil, err
	}
	// Only a thread nobody holds is handed out. Inbound calls this on every
	// message, so without this a customer's second message would move their
	// thread to a different agent mid-problem — and burn a rotation turn doing
	// it, skewing the share-out for everyone else.
	if conv.Status != models.ConversationUnassigned {
		return nil, nil
	}

	online, err := s.presence.Online(ctx)
	if err != nil {
		return nil, fmt.Errorf("read online agents: %w", err)
	}
	if len(online) == 0 {
		return nil, nil
	}

	turn, err := s.presence.NextTurn(ctx)
	if err != nil {
		return nil, fmt.Errorf("advance rotation: %w", err)
	}

	holder := online[turn%uint64(len(online))]
	if err := s.conversations.Assign(conversationID, holder); err != nil {
		return nil, err
	}
	return &holder, nil
}

// AssignWaiting shares out everything that arrived while nobody was watching.
func (s *CSAssignmentService) AssignWaiting(ctx context.Context) (int, error) {
	var waiting []models.CSConversation
	err := s.db.Where("status = ?", models.ConversationUnassigned).
		Order("last_message_at ASC").
		Find(&waiting).Error
	if err != nil {
		return 0, fmt.Errorf("list waiting conversations: %w", err)
	}

	assigned := 0
	for _, conv := range waiting {
		holder, err := s.AssignOne(ctx, conv.ID)
		if err != nil {
			return assigned, err
		}
		// These rows were selected as unassigned, so AssignOne's nil here can
		// only mean the rotation is dry: nobody is online, and the rest of the
		// queue will wait too.
		if holder == nil {
			break
		}
		assigned++
	}
	return assigned, nil
}
