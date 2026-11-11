package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"go.uber.org/zap"
)

// PortAlertSenderName is who a group sees the notices from.
const PortAlertSenderName = "Notifikasi Port"

// telegramMessageLimit is the longest text Telegram takes in one message,
// counted in UTF-16 code units. A NOC notice for a device many reports share
// could pass it.
const telegramMessageLimit = 4096

// noticeTarget is where one notice goes: a WhatsApp group, a Telegram chat
// or topic, or both. name says whose it is in the logs.
type noticeTarget struct {
	name      string
	waGroupID *uuid.UUID
	chatID    *uuid.UUID
	threadID  *int64
}

// notify sends one notice per report and direction to the reports that ask
// for them, and one per direction to the NOC.
func (s *PortAlertService) notify(ctx context.Context, deviceID int, changed []portTransition) error {
	byName := make(map[string]portTransition, len(changed))
	names := make([]string, len(changed))
	for i, transition := range changed {
		byName[transition.ifName], names[i] = transition, transition.ifName
	}
	var rows []models.GraphReportPort
	err := s.db.Where("device_id = ? AND if_name IN ?", deviceID, names).Order("position ASC").Find(&rows).Error
	if err != nil {
		return fmt.Errorf("load changed ports' reports: %w", err)
	}
	byReport := map[uuid.UUID][]models.GraphReportPort{}
	reportIDs := []uuid.UUID{}
	for _, row := range rows {
		if byReport[row.ReportID] == nil {
			reportIDs = append(reportIDs, row.ReportID)
		}
		byReport[row.ReportID] = append(byReport[row.ReportID], row)
	}
	var reports []models.GraphReport
	if err := s.db.Where("id IN ?", reportIDs).Order("customer_name ASC").Find(&reports).Error; err != nil {
		return fmt.Errorf("load changed ports' reports: %w", err)
	}
	for _, status := range []string{models.PortLinkDown, models.PortLinkUp} {
		var noc []noticePort
		for _, report := range reports {
			ports := reportNoticePorts(report, byReport[report.ID], byName, status)
			if len(ports) == 0 {
				continue
			}
			noc = append(noc, ports...)
			if report.Enabled && report.NotifyPortStatus {
				s.deliver(ctx, reportTarget(report), customerNotice(report.CustomerName, status, ports))
			}
		}
		if len(noc) > 0 {
			s.deliverNOC(ctx, nocNotice(status, noc))
		}
	}
	return nil
}

// reportNoticePorts answers a report's ports that moved to status, each once
// even where the report graphs a port twice.
func reportNoticePorts(
	report models.GraphReport, rows []models.GraphReportPort, changed map[string]portTransition, status string,
) []noticePort {
	var ports []noticePort
	seen := map[string]bool{}
	for _, row := range rows {
		transition := changed[row.IfName]
		if transition.status != status || seen[row.IfName] {
			continue
		}
		seen[row.IfName] = true
		ports = append(ports, noticePort{
			customer: report.CustomerName, label: row.Label, device: row.DeviceName, transition: transition,
		})
	}
	return ports
}

func reportTarget(report models.GraphReport) noticeTarget {
	return noticeTarget{
		name: "report " + report.CustomerName, waGroupID: report.WAGroupID,
		chatID: report.TelegramChatID, threadID: report.TelegramThreadID,
	}
}

func (s *PortAlertService) deliverNOC(ctx context.Context, text string) {
	var noc models.PortAlertDestination
	if err := s.db.Limit(1).Find(&noc).Error; err != nil {
		s.logger.Warn("load NOC port alert destination", zap.Error(err))
		return
	}
	if noc.ID == uuid.Nil || !noc.Enabled {
		return
	}
	s.deliver(ctx, noticeTarget{
		name: "NOC", waGroupID: noc.WAGroupID, chatID: noc.TelegramChatID, threadID: noc.TelegramThreadID,
	}, text)
}

// deliver sends a notice to each of a target's destinations. A failure is
// logged and left: the status is recorded either way, so the next transition
// still notifies correctly, and one destination failing must not keep the
// notice from the other.
func (s *PortAlertService) deliver(ctx context.Context, target noticeTarget, text string) {
	if target.waGroupID != nil {
		if err := s.sendWAText(ctx, *target.waGroupID, text); err != nil {
			s.logger.Warn("port notice to WhatsApp failed", zap.String("destination", target.name), zap.Error(err))
		}
	}
	if target.chatID != nil {
		if err := s.sendTelegramText(ctx, *target.chatID, target.threadID, text); err != nil {
			s.logger.Warn("port notice to Telegram failed", zap.String("destination", target.name), zap.Error(err))
		}
	}
}

// sendWAText queues a notice through the group's number, as the reports are.
func (s *PortAlertService) sendWAText(ctx context.Context, groupID uuid.UUID, text string) error {
	group, err := s.sender.groups.Get(groupID)
	if err != nil {
		return fmt.Errorf("load group: %w", err)
	}
	if err := CheckGroupSendable(group); err != nil {
		return err
	}
	members, _ := s.sender.groups.MembersOtherThanUs(groupID)
	msg, err := s.sender.messages.Queue(GroupOutgoing{
		GroupID: groupID, SenderName: PortAlertSenderName, Kind: models.MessageKindText, Body: text,
		MemberCount: members,
	})
	if err != nil {
		return err
	}
	if s.sender.notifier != nil {
		s.sender.notifier.GroupMessagesQueued(ctx, []models.WAGroupMessage{*msg})
	}
	return nil
}

// sendTelegramText posts a notice, following a chat upgraded to a supergroup
// or marking one the bot lost as the reports' sends do.
func (s *PortAlertService) sendTelegramText(ctx context.Context, chatID uuid.UUID, threadID *int64, text string) error {
	tg, err := s.sender.prepareTelegramChat(chatID, threadID)
	if err != nil {
		return err
	}
	if utf16Len(text) > telegramMessageLimit {
		text = cutUTF16(text, telegramMessageLimit)
	}
	err = tg.client.SendMessage(ctx, tg.chat.ChatID, tg.threadID, text)
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		return s.sender.telegramRefusal(tg.chat, apiErr)
	}
	return err
}
