package services

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// decorate fills in what a page of a group needs beyond the rows themselves:
// who was tagged, the reactions, how far our messages got, and which senders
// have a face to show. One query each for the whole page.
func (s *WAGroupMessageService) decorate(groupID uuid.UUID, rows []models.WAGroupMessage) error {
	if len(rows) == 0 {
		return nil
	}
	s.nameMentions(groupID, rows)
	if err := s.attachReactions(rows); err != nil {
		return err
	}
	if err := s.attachReceiptCounts(rows); err != nil {
		return err
	}
	if err := s.attachPollVotes(rows); err != nil {
		return err
	}
	return s.markSenderAvatars(groupID, rows)
}

// nameMentions puts names where our own messages carry addresses, and lists
// who each message tags. A failure to read the members leaves the addresses,
// which are still correct.
func (s *WAGroupMessageService) nameMentions(groupID uuid.UUID, rows []models.WAGroupMessage) {
	var names map[string]string
	for i := range rows {
		jids := Mentions(&rows[i])
		if len(jids) == 0 {
			continue
		}
		if names == nil {
			names = memberNamesOf(s.db, groupID)
		}
		rows[i].Body = ResolveMentions(rows[i].Body, jids, names)
		for _, jid := range jids {
			if name, ok := names[jidUser(jid)]; ok {
				rows[i].MentionNames = append(rows[i].MentionNames, name)
			}
		}
	}
}

func messageIDs(rows []models.WAGroupMessage) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
}

func (s *WAGroupMessageService) attachReactions(rows []models.WAGroupMessage) error {
	var reactions []models.WAGroupReaction
	if err := s.db.Where("message_id IN ?", messageIDs(rows)).Order("updated_at").Find(&reactions).Error; err != nil {
		return fmt.Errorf("load reactions: %w", err)
	}
	byMessage := make(map[uuid.UUID][]models.WAGroupReaction)
	for _, r := range reactions {
		byMessage[r.MessageID] = append(byMessage[r.MessageID], r)
	}
	for i := range rows {
		rows[i].Reactions = summariseReactions(byMessage[rows[i].ID])
	}
	return nil
}

// summariseReactions groups a message's reactions by emoji, the most chosen
// first, the way WhatsApp draws the chip under a bubble.
func summariseReactions(reactions []models.WAGroupReaction) []models.ReactionSummary {
	if len(reactions) == 0 {
		return nil
	}
	index := map[string]int{}
	var out []models.ReactionSummary
	for _, r := range reactions {
		at, seen := index[r.Emoji]
		if !seen {
			at = len(out)
			index[r.Emoji] = at
			out = append(out, models.ReactionSummary{Emoji: r.Emoji})
		}
		out[at].Count++
		out[at].Reactors = append(out[at].Reactors, models.GroupActor{JID: r.ReactorJID, Name: r.ReactorName, IsSelf: r.IsSelf})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Count > out[b].Count })
	return out
}

type receiptCount struct {
	MessageID      uuid.UUID
	DeliveredCount int
	ReadCount      int
}

func (s *WAGroupMessageService) attachReceiptCounts(rows []models.WAGroupMessage) error {
	var counts []receiptCount
	err := s.db.Model(&models.WAGroupReceipt{}).
		Select("message_id, COUNT(delivered_at) AS delivered_count, COUNT(read_at) AS read_count").
		Where("message_id IN ?", messageIDs(rows)).Group("message_id").Scan(&counts).Error
	if err != nil {
		return fmt.Errorf("count receipts: %w", err)
	}
	byMessage := make(map[uuid.UUID]receiptCount, len(counts))
	for _, c := range counts {
		byMessage[c.MessageID] = c
	}
	for i := range rows {
		if rows[i].Direction != models.MessageOut || rows[i].Kind == models.MessageKindSystem {
			continue
		}
		c := byMessage[rows[i].ID]
		delivered, read := c.DeliveredCount, c.ReadCount
		rows[i].DeliveredCount, rows[i].ReadCount = &delivered, &read
	}
	return nil
}

func (s *WAGroupMessageService) markSenderAvatars(groupID uuid.UUID, rows []models.WAGroupMessage) error {
	var group models.WAGroup
	if err := s.db.Select("id", "wa_account_id").First(&group, "id = ?", groupID).Error; err != nil {
		return fmt.Errorf("load group for avatars: %w", err)
	}
	jids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.SenderJID != "" {
			jids = append(jids, row.SenderJID)
		}
	}
	if len(jids) == 0 {
		return nil
	}
	shown, err := avatarsShown(s.db, jids)
	if err != nil {
		return err
	}
	for i := range rows {
		_, rows[i].SenderHasAvatar = shown[avatarKey{group.WAAccountID, rows[i].SenderJID}]
	}
	return nil
}
