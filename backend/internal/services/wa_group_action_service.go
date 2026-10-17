package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

const (
	// maxGroupNameRunes is WhatsApp's own limit; a longer name is refused by
	// the server with a bare 406, which would reach the admin as gibberish.
	maxGroupNameRunes = 25
	// maxGroupTopicRunes is where WhatsApp cuts a description.
	maxGroupTopicRunes = 2048
	// maxGroupParticipants bounds one request. WhatsApp groups hold 1024.
	maxGroupParticipants = 1024
	phoneJIDSuffix       = "@s.whatsapp.net"
	lidJIDSuffix         = "@lid"
)

var (
	// ErrGroupActionUnknown is an action this service does not know.
	ErrGroupActionUnknown = fmt.Errorf("%w: aksi grup tidak dikenal", ErrValidation)
	// ErrGroupActionNeedsGroup is a management action that names no group.
	ErrGroupActionNeedsGroup = fmt.Errorf("%w: aksi ini harus menyebut grupnya", ErrValidation)
)

// GroupActionArgs is what an action carries besides its kind. Participants
// are JIDs by the time they are stored.
type GroupActionArgs struct {
	Name         string   `json:"name,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	Participants []string `json:"participants,omitempty"`
	Enabled      bool     `json:"enabled,omitempty"`
	// MessageID names the message a react, edit, revoke or vote acts on.
	MessageID string   `json:"message_id,omitempty"`
	Emoji     string   `json:"emoji,omitempty"`
	Body      string   `json:"body,omitempty"`
	Options   []string `json:"options,omitempty"`
}

// GroupActionRequest is one management request as the API received it.
type GroupActionRequest struct {
	AccountID uuid.UUID
	GroupID   *uuid.UUID
	Action    models.GroupActionKind
	Args      GroupActionArgs
	UserID    uuid.UUID
}

// WAGroupActionService is the queue of management requests for the wa
// process, and where their answers are read back from.
type WAGroupActionService struct {
	db *gorm.DB
}

// NewWAGroupActionService constructs a WAGroupActionService.
func NewWAGroupActionService(db *gorm.DB) *WAGroupActionService {
	return &WAGroupActionService{db: db}
}

// Queue validates a request and writes it for the wa process to carry out.
// Validation happens here rather than in the wa process so the admin is told
// at once, not by a failure a few seconds later.
func (s *WAGroupActionService) Queue(req GroupActionRequest) (*models.WAGroupAction, error) {
	args, err := validateGroupAction(req)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("encode group action: %w", err)
	}
	action := models.WAGroupAction{
		WAAccountID: req.AccountID,
		GroupID:     req.GroupID,
		Action:      req.Action,
		Payload:     string(payload),
		Status:      models.GroupActionQueued,
		RequestedBy: req.UserID,
	}
	if err := s.db.Create(&action).Error; err != nil {
		return nil, fmt.Errorf("queue group action: %w", err)
	}
	return &action, nil
}

func validateGroupAction(req GroupActionRequest) (GroupActionArgs, error) {
	args := req.Args
	if req.Action != models.GroupActionCreate && req.GroupID == nil {
		return args, ErrGroupActionNeedsGroup
	}
	switch req.Action {
	case models.GroupActionCreate:
		if err := validGroupName(args.Name); err != nil {
			return args, err
		}
		return withParticipants(args)
	case models.GroupActionRename:
		return args, validGroupName(args.Name)
	case models.GroupActionSetTopic:
		if utf8.RuneCountInString(args.Topic) > maxGroupTopicRunes {
			return args, fmt.Errorf("%w: deskripsi grup paling panjang %d karakter", ErrValidation, maxGroupTopicRunes)
		}
		return args, nil
	case models.GroupActionAdd, models.GroupActionRemove, models.GroupActionPromote, models.GroupActionDemote:
		return withParticipants(args)
	case models.GroupActionSetAnnounce, models.GroupActionSetLocked,
		models.GroupActionInviteLink, models.GroupActionResetInviteLink, models.GroupActionLeave,
		models.GroupActionHistory:
		return args, nil
	case models.GroupActionReact, models.GroupActionRevoke, models.GroupActionVote:
		return args, needsMessage(args)
	case models.GroupActionEdit:
		if strings.TrimSpace(args.Body) == "" {
			return args, fmt.Errorf("%w: isi pesan tidak boleh kosong", ErrValidation)
		}
		return args, needsMessage(args)
	default:
		return args, ErrGroupActionUnknown
	}
}

func needsMessage(args GroupActionArgs) error {
	if _, err := uuid.Parse(args.MessageID); err != nil {
		return fmt.Errorf("%w: aksi ini harus menyebut pesannya", ErrValidation)
	}
	return nil
}

func validGroupName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: nama grup wajib diisi", ErrValidation)
	}
	if utf8.RuneCountInString(trimmed) > maxGroupNameRunes {
		return fmt.Errorf("%w: nama grup paling panjang %d karakter", ErrValidation, maxGroupNameRunes)
	}
	return nil
}

// withParticipants turns what an admin typed into JIDs. A member picked from
// the list arrives as a JID already; a number typed in arrives in whichever of
// the Indonesian forms they use.
func withParticipants(args GroupActionArgs) (GroupActionArgs, error) {
	if len(args.Participants) == 0 {
		return args, fmt.Errorf("%w: sebutkan minimal satu anggota", ErrValidation)
	}
	if len(args.Participants) > maxGroupParticipants {
		return args, fmt.Errorf("%w: paling banyak %d anggota sekali minta", ErrValidation, maxGroupParticipants)
	}
	seen := make(map[string]struct{}, len(args.Participants))
	jids := make([]string, 0, len(args.Participants))
	for _, raw := range args.Participants {
		jid, err := participantJID(raw)
		if err != nil {
			return args, err
		}
		if _, dup := seen[jid]; dup {
			continue
		}
		seen[jid] = struct{}{}
		jids = append(jids, jid)
	}
	args.Participants = jids
	return args, nil
}

func participantJID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasSuffix(raw, phoneJIDSuffix) || strings.HasSuffix(raw, lidJIDSuffix) {
		if strings.HasPrefix(raw, "@") {
			return "", fmt.Errorf("%w: anggota %q tidak valid", ErrValidation, raw)
		}
		return raw, nil
	}
	phone, err := utils.NormalizePhone(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrValidation, err.Error())
	}
	return phone + phoneJIDSuffix, nil
}

// Args decodes what an action carries.
func (s *WAGroupActionService) Args(action *models.WAGroupAction) (GroupActionArgs, error) {
	var args GroupActionArgs
	if err := json.Unmarshal([]byte(action.Payload), &args); err != nil {
		return args, fmt.Errorf("decode group action: %w", err)
	}
	return args, nil
}

// ClaimQueued answers the actions waiting on one number, oldest first.
func (s *WAGroupActionService) ClaimQueued(accountID uuid.UUID, limit int) ([]models.WAGroupAction, error) {
	var rows []models.WAGroupAction
	err := s.db.Where("status = ? AND wa_account_id = ?", models.GroupActionQueued, accountID).
		Order("created_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("claim queued group actions: %w", err)
	}
	return rows, nil
}

// MarkDone records what WhatsApp answered. groupID is set for a create, whose
// group did not exist when the request was written.
func (s *WAGroupActionService) MarkDone(id uuid.UUID, result string, groupID *uuid.UUID) error {
	fields := map[string]any{"status": models.GroupActionDone, "result": result, "done_at": time.Now()}
	if groupID != nil {
		fields["group_id"] = *groupID
	}
	return s.update(id, fields)
}

// MarkFailed records why WhatsApp refused an action.
func (s *WAGroupActionService) MarkFailed(id uuid.UUID, reason string) error {
	return s.update(id, map[string]any{
		"status": models.GroupActionFailed, "fail_reason": reason, "done_at": time.Now(),
	})
}

func (s *WAGroupActionService) update(id uuid.UUID, fields map[string]any) error {
	res := s.db.Model(&models.WAGroupAction{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("update group action: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Get answers one action.
func (s *WAGroupActionService) Get(id uuid.UUID) (*models.WAGroupAction, error) {
	var action models.WAGroupAction
	if err := s.db.First(&action, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &action, nil
}
