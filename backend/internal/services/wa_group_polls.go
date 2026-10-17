package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm/clause"
)

// maxPollOptions is WhatsApp's own limit.
const maxPollOptions = 12

// PollExtra is what a poll stores in WAGroupMessage.Extra. Voters are not
// stored there: they are filled in from wa_group_poll_votes on the way out.
type PollExtra struct {
	Question        string       `json:"question"`
	SelectableCount int          `json:"selectable_count"`
	Options         []PollOption `json:"options"`
}

// PollOption is one choice and who picked it.
type PollOption struct {
	Name   string              `json:"name"`
	Voters []models.GroupActor `json:"voters"`
}

// NewPollExtra validates a poll a user writes and encodes it.
func NewPollExtra(question string, options []string, selectable int) (string, error) {
	if question == "" {
		return "", fmt.Errorf("%w: pertanyaan polling wajib diisi", ErrValidation)
	}
	options = uniqueStrings(options)
	if len(options) < 2 || len(options) > maxPollOptions {
		return "", fmt.Errorf("%w: polling butuh 2 sampai %d pilihan berbeda", ErrValidation, maxPollOptions)
	}
	if selectable < 0 || selectable > len(options) {
		return "", fmt.Errorf("%w: jumlah pilihan yang boleh dipilih tidak valid", ErrValidation)
	}
	extra := PollExtra{Question: question, SelectableCount: selectable}
	for _, name := range options {
		if name == "" {
			return "", fmt.Errorf("%w: pilihan polling tidak boleh kosong", ErrValidation)
		}
		extra.Options = append(extra.Options, PollOption{Name: name, Voters: []models.GroupActor{}})
	}
	encoded, err := json.Marshal(extra)
	if err != nil {
		return "", fmt.Errorf("encode poll: %w", err)
	}
	return string(encoded), nil
}

// DecodePoll reads a poll back.
func DecodePoll(msg *models.WAGroupMessage) (PollExtra, error) {
	var extra PollExtra
	if err := json.Unmarshal([]byte(msg.Extra), &extra); err != nil {
		return extra, fmt.Errorf("decode poll: %w", err)
	}
	return extra, nil
}

// SetVote records one member's current choice in a poll; an empty choice
// withdraws their vote.
func (s *WAGroupMessageService) SetVote(messageID uuid.UUID, voter models.GroupActor, options []string) error {
	if len(options) == 0 {
		err := s.db.Where("message_id = ? AND voter_jid = ?", messageID, voter.JID).Delete(&models.WAGroupPollVote{}).Error
		if err != nil {
			return fmt.Errorf("withdraw poll vote: %w", err)
		}
		return nil
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("encode poll vote: %w", err)
	}
	row := models.WAGroupPollVote{
		MessageID: messageID, VoterJID: voter.JID, VoterName: voter.Name, IsSelf: voter.IsSelf,
		Options: string(encoded), UpdatedAt: time.Now(),
	}
	err = s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "voter_jid"}},
		DoUpdates: clause.AssignmentColumns([]string{"options", "voter_name", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("store poll vote: %w", err)
	}
	return nil
}

// attachPollVotes puts the voters on each poll's options. A vote naming an
// option the poll does not have is dropped: it can only be from a client
// that hashed something else.
func (s *WAGroupMessageService) attachPollVotes(rows []models.WAGroupMessage) error {
	polls := make([]uuid.UUID, 0)
	for _, row := range rows {
		if row.Kind == models.MessageKindPoll && row.Extra != "" {
			polls = append(polls, row.ID)
		}
	}
	if len(polls) == 0 {
		return nil
	}
	var votes []models.WAGroupPollVote
	if err := s.db.Where("message_id IN ?", polls).Order("updated_at").Find(&votes).Error; err != nil {
		return fmt.Errorf("load poll votes: %w", err)
	}
	byPoll := make(map[uuid.UUID][]models.WAGroupPollVote)
	for _, v := range votes {
		byPoll[v.MessageID] = append(byPoll[v.MessageID], v)
	}
	for i := range rows {
		if rows[i].Kind != models.MessageKindPoll || rows[i].Extra == "" {
			continue
		}
		tallied, err := tallyPoll(&rows[i], byPoll[rows[i].ID])
		if err == nil {
			rows[i].ExtraData = tallied
		}
	}
	return nil
}

func tallyPoll(msg *models.WAGroupMessage, votes []models.WAGroupPollVote) (json.RawMessage, error) {
	extra, err := DecodePoll(msg)
	if err != nil {
		return nil, err
	}
	at := make(map[string]int, len(extra.Options))
	for i := range extra.Options {
		at[extra.Options[i].Name] = i
		extra.Options[i].Voters = []models.GroupActor{}
	}
	for _, v := range votes {
		var chosen []string
		if json.Unmarshal([]byte(v.Options), &chosen) != nil {
			continue
		}
		for _, name := range chosen {
			if i, ok := at[name]; ok {
				extra.Options[i].Voters = append(extra.Options[i].Voters,
					models.GroupActor{JID: v.VoterJID, Name: v.VoterName, IsSelf: v.IsSelf})
			}
		}
	}
	return json.Marshal(extra)
}
