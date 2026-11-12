package services

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// views attaches the groups, their numbers, the Telegram chats and topics,
// and the ports in five queries for the whole page.
func (s *GraphReportService) views(reports []models.GraphReport) ([]GraphReportView, error) {
	ids := make([]uuid.UUID, len(reports))
	var groupIDs, chatIDs []uuid.UUID
	for i, report := range reports {
		ids[i] = report.ID
		if report.WAGroupID != nil {
			groupIDs = append(groupIDs, *report.WAGroupID)
		}
		if report.TelegramChatID != nil {
			chatIDs = append(chatIDs, *report.TelegramChatID)
		}
	}
	var ports []models.GraphReportPort
	if err := s.db.Where("report_id IN ?", ids).Order("position ASC").Find(&ports).Error; err != nil {
		return nil, fmt.Errorf("load graph report ports: %w", err)
	}
	groups, numbers, err := reportGroups(s.db, groupIDs)
	if err != nil {
		return nil, err
	}
	chats, err := reportChats(s.db, chatIDs)
	if err != nil {
		return nil, err
	}
	topics, err := reportTopics(s.db, chatIDs)
	if err != nil {
		return nil, err
	}
	byReport := make(map[uuid.UUID][]models.GraphReportPort, len(reports))
	for _, port := range ports {
		byReport[port.ReportID] = append(byReport[port.ReportID], port)
	}
	views := make([]GraphReportView, len(reports))
	for i, report := range reports {
		views[i] = graphReportView(report, byReport[report.ID])
		if report.WAGroupID != nil {
			group := groups[*report.WAGroupID]
			views[i].GroupName, views[i].WAGroupJID = group.Name, group.JID
			views[i].WAAccountID, views[i].WAAccountLabel = &group.WAAccountID, numbers[group.WAAccountID]
		}
		if report.TelegramChatID != nil {
			views[i].TelegramChatTitle = chats[*report.TelegramChatID]
		}
		if report.TelegramThreadID != nil {
			views[i].TelegramTopicName = topics[topicKey{*report.TelegramChatID, *report.TelegramThreadID}]
		}
	}
	return views, nil
}

// reportGroups answers the reports' WhatsApp groups by ID, and what each
// group's number is called.
func reportGroups(db *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]models.WAGroup, map[uuid.UUID]string, error) {
	var groups []models.WAGroup
	if err := db.Select("id", "name", "jid", "wa_account_id").Where("id IN ?", ids).Find(&groups).Error; err != nil {
		return nil, nil, fmt.Errorf("load graph report groups: %w", err)
	}
	byID := make(map[uuid.UUID]models.WAGroup, len(groups))
	accountIDs := make([]uuid.UUID, len(groups))
	for i, group := range groups {
		byID[group.ID], accountIDs[i] = group, group.WAAccountID
	}
	numbers, err := accountNames(db, accountIDs)
	if err != nil {
		return nil, nil, err
	}
	return byID, numbers, nil
}

// reportChats answers the reports' Telegram chats' titles by ID.
func reportChats(db *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	var chats []models.TelegramChat
	if err := db.Select("id", "title").Where("id IN ?", ids).Find(&chats).Error; err != nil {
		return nil, fmt.Errorf("load graph report Telegram chats: %w", err)
	}
	titles := make(map[uuid.UUID]string, len(chats))
	for _, chat := range chats {
		titles[chat.ID] = chat.Title
	}
	return titles, nil
}

// topicKey names a forum topic: thread IDs are only unique within a chat.
type topicKey struct {
	chatID   uuid.UUID
	threadID int64
}

// reportTopics answers the names of the topics in the reports' chats.
func reportTopics(db *gorm.DB, chatIDs []uuid.UUID) (map[topicKey]string, error) {
	var topics []models.TelegramTopic
	err := db.Select("telegram_chat_id", "thread_id", "name").Where("telegram_chat_id IN ?", chatIDs).Find(&topics).Error
	if err != nil {
		return nil, fmt.Errorf("load graph report Telegram topics: %w", err)
	}
	names := make(map[topicKey]string, len(topics))
	for _, topic := range topics {
		names[topicKey{topic.TelegramChatID, topic.ThreadID}] = topic.Name
	}
	return names, nil
}

func graphReportView(report models.GraphReport, ports []models.GraphReportPort) GraphReportView {
	view := GraphReportView{
		ID: report.ID, WAGroupID: report.WAGroupID, TelegramChatID: report.TelegramChatID,
		TelegramThreadID: report.TelegramThreadID,
		CustomerName:     report.CustomerName, Message: report.Message, Enabled: report.Enabled,
		MentionAll: report.MentionAll, SendTime: report.SendTime, LastAttemptAt: report.LastAttemptAt,
		LastError: report.LastError, Ports: ports,
	}
	if view.Ports == nil {
		view.Ports = []models.GraphReportPort{}
	}
	if report.LastSentOn != nil {
		day := report.LastSentOn.UTC().Format(dateLayout)
		view.LastSentOn = &day
	}
	return view
}
