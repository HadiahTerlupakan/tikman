package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/tikman/olt-provisioning/internal/models"
)

// MemberNames indexes a group's members by the user part of each of their
// addresses, which is also what a mention writes after the "@".
func MemberNames(members []models.WAGroupParticipant) map[string]string {
	names := make(map[string]string, len(members)*2)
	for _, member := range members {
		label := member.Name
		if label == "" {
			label = member.Phone
		}
		if label == "" {
			continue
		}
		for _, address := range []string{member.JID, member.LID} {
			if user := jidUser(address); user != "" {
				names[user] = label
			}
		}
		if member.Phone != "" {
			names[member.Phone] = label
		}
	}
	return names
}

func jidUser(address string) string {
	user, _, found := strings.Cut(address, "@")
	if !found {
		return ""
	}
	return user
}

// ResolveMentions rewrites "@628123…" or "@<LID>" into "@Name". WhatsApp
// carries a mention as the address in the text, and in a LID-addressed group
// that is a long number that is nobody's phone: read as is, it tells the team
// nothing about who was asked.
func ResolveMentions(body string, mentioned []string, names map[string]string) string {
	for _, jid := range mentioned {
		user := jidUser(jid)
		if name, ok := names[user]; ok && user != "" {
			body = strings.ReplaceAll(body, "@"+user, "@"+name)
		}
	}
	return body
}

// Mentions answers who an outgoing message tags.
func Mentions(msg *models.WAGroupMessage) []string {
	if msg.Mentions == "" {
		return nil
	}
	var jids []string
	if err := json.Unmarshal([]byte(msg.Mentions), &jids); err != nil {
		return nil
	}
	return jids
}

// showMentions puts names where our own messages carry addresses. A failure
// to read the members leaves the addresses, which are still correct.
func showMentions(db *gorm.DB, groupID uuid.UUID, rows []models.WAGroupMessage) {
	var names map[string]string
	for i := range rows {
		jids := Mentions(&rows[i])
		if len(jids) == 0 {
			continue
		}
		if names == nil {
			names = memberNamesOf(db, groupID)
		}
		rows[i].Body = ResolveMentions(rows[i].Body, jids, names)
	}
}

// withNames is body as the group list should preview it.
func withNames(db *gorm.DB, groupID uuid.UUID, body string, jids []string) string {
	if len(jids) == 0 {
		return body
	}
	return ResolveMentions(body, jids, memberNamesOf(db, groupID))
}

func memberNamesOf(db *gorm.DB, groupID uuid.UUID) map[string]string {
	var members []models.WAGroupParticipant
	if err := db.Where("group_id = ?", groupID).Find(&members).Error; err != nil {
		return map[string]string{}
	}
	return MemberNames(members)
}

// maxMentions bounds one message; WhatsApp groups hold 1024 members.
const maxMentions = 1024

// CheckMentions refuses a mention of anyone who is not in the group. A tag
// naming a stranger reaches nobody, and would show the team a name the
// group never saw.
func (s *WAGroupService) CheckMentions(groupID uuid.UUID, jids []string) error {
	if len(jids) > maxMentions {
		return fmt.Errorf("%w: terlalu banyak anggota yang di-mention", ErrValidation)
	}
	if len(jids) == 0 {
		return nil
	}
	var found int64
	err := s.db.Model(&models.WAGroupParticipant{}).
		Where("group_id = ? AND jid IN ?", groupID, jids).Count(&found).Error
	if err != nil {
		return fmt.Errorf("check mentions: %w", err)
	}
	if int(found) != len(uniqueStrings(jids)) {
		return fmt.Errorf("%w: yang di-mention harus anggota grup ini", ErrValidation)
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, dup := seen[v]; !dup {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}
