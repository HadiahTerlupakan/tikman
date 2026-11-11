package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// MaxGraphReportPorts keeps one morning's report to a handful of images; past
// this a group reads it as spam.
const MaxGraphReportPorts = 10

// graphReportTextRunes matches the varchar(255) the names are stored in.
const graphReportTextRunes = 255

// MaxGraphReportMessageRunes keeps the caption to a greeting; WhatsApp would
// take far more, but a report's text is not the place for a letter.
const MaxGraphReportMessageRunes = 1000

// dateLayout is how last_sent_on travels to the browser.
const dateLayout = "2006-01-02"

// ErrGraphReportDuplicate is a second report for a group that has one.
var ErrGraphReportDuplicate = errors.New("grup ini sudah memiliki laporan grafik")

// GraphReportPortInput is one port as the admin picked it.
type GraphReportPortInput struct {
	DeviceID   int
	DeviceName string
	IfName     string
	Label      string
}

// GraphReportInput is a report as the admin submitted it. Ports are sent in
// the order given.
type GraphReportInput struct {
	// WAGroupID and TelegramChatID are the destinations; nil is none, and a
	// report needs at least one.
	WAGroupID      *uuid.UUID
	TelegramChatID *uuid.UUID
	// TelegramThreadID is the forum topic within the chat; nil is General,
	// or the chat itself when it is no forum.
	TelegramThreadID *int64
	CustomerName     string
	Message          string
	Enabled          bool
	MentionAll       bool
	// SendTime is "HH:MM" in WIB; blank means DefaultGraphReportSendTime.
	SendTime string
	Ports    []GraphReportPortInput
}

// GraphReportView is a report as the management page shows it.
// The WhatsApp fields are null or empty for a report with no WhatsApp group,
// and the Telegram ones for a report with no Telegram chat.
type GraphReportView struct {
	ID        uuid.UUID  `json:"id"`
	WAGroupID *uuid.UUID `json:"wa_group_id"`
	GroupName string     `json:"group_name"`
	// WAGroupJID is the WhatsApp group itself, the same through every number,
	// so the form can tell a group already reported through another number.
	WAGroupJID  string     `json:"wa_group_jid"`
	WAAccountID *uuid.UUID `json:"wa_account_id"`
	// WAAccountLabel is the number that sends the report, by its label or,
	// unlabelled, the number itself.
	WAAccountLabel    string     `json:"wa_account_label"`
	TelegramChatID    *uuid.UUID `json:"telegram_chat_id"`
	TelegramChatTitle string     `json:"telegram_chat_title"`
	TelegramThreadID  *int64     `json:"telegram_thread_id"`
	// TelegramTopicName is empty for General and for a topic never named.
	TelegramTopicName string                   `json:"telegram_topic_name"`
	CustomerName      string                   `json:"customer_name"`
	Message           string                   `json:"message"`
	Enabled           bool                     `json:"enabled"`
	MentionAll        bool                     `json:"mention_all"`
	SendTime          string                   `json:"send_time"`
	LastSentOn        *string                  `json:"last_sent_on"`
	LastAttemptAt     *time.Time               `json:"last_attempt_at"`
	LastError         string                   `json:"last_error"`
	Ports             []models.GraphReportPort `json:"ports"`
}

// GraphReportService stores the daily graph reports.
type GraphReportService struct {
	db *gorm.DB
}

// NewGraphReportService constructs a GraphReportService.
func NewGraphReportService(db *gorm.DB) *GraphReportService {
	return &GraphReportService{db: db}
}

// List answers every report, by customer.
func (s *GraphReportService) List() ([]GraphReportView, error) {
	var reports []models.GraphReport
	if err := s.db.Order("customer_name ASC").Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("list graph reports: %w", err)
	}
	return s.views(reports)
}

// Get answers one report.
func (s *GraphReportService) Get(id uuid.UUID) (*GraphReportView, error) {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return nil, err
	}
	views, err := s.views([]models.GraphReport{report})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// Create registers a report for a group that has none.
func (s *GraphReportService) Create(in GraphReportInput, actor uuid.UUID) (*GraphReportView, error) {
	if err := s.validate(&in, nil); err != nil {
		return nil, err
	}
	report := models.GraphReport{
		WAGroupID: in.WAGroupID, TelegramChatID: in.TelegramChatID, TelegramThreadID: in.TelegramThreadID,
		CustomerName: strings.TrimSpace(in.CustomerName), Message: strings.TrimSpace(in.Message),
		Enabled: in.Enabled, MentionAll: in.MentionAll, SendTime: in.SendTime, CreatedBy: &actor,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := claimDestinations(tx, in, uuid.Nil); err != nil {
			return err
		}
		// Enabled is written explicitly: GORM skips a false bool on create
		// and the column default would turn the report on.
		if err := tx.Create(&report).Error; err != nil {
			return fmt.Errorf("create graph report: %w", err)
		}
		if err := tx.Model(&report).Update("enabled", in.Enabled).Error; err != nil {
			return fmt.Errorf("create graph report: %w", err)
		}
		return replacePorts(tx, report.ID, in.Ports)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(report.ID)
}

// Update rewrites a report and replaces its ports.
func (s *GraphReportService) Update(id uuid.UUID, in GraphReportInput) (*GraphReportView, error) {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := s.validate(&in, &report); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := claimDestinations(tx, in, id); err != nil {
			return err
		}
		err := tx.Model(&report).Updates(map[string]any{
			"wa_group_id": in.WAGroupID, "telegram_chat_id": in.TelegramChatID, "telegram_thread_id": in.TelegramThreadID,
			"customer_name": strings.TrimSpace(in.CustomerName), "message": strings.TrimSpace(in.Message),
			"enabled": in.Enabled, "mention_all": in.MentionAll, "send_time": in.SendTime,
		}).Error
		if err != nil {
			return fmt.Errorf("update graph report: %w", err)
		}
		return replacePorts(tx, id, in.Ports)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Delete removes a report with its ports. The ports are deleted here as well
// as by the foreign key because SQLite does not enforce it.
func (s *GraphReportService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("report_id = ?", id).Delete(&models.GraphReportPort{}).Error; err != nil {
			return fmt.Errorf("delete graph report ports: %w", err)
		}
		result := tx.Delete(&models.GraphReport{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("delete graph report: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func replacePorts(tx *gorm.DB, reportID uuid.UUID, ports []GraphReportPortInput) error {
	if err := tx.Where("report_id = ?", reportID).Delete(&models.GraphReportPort{}).Error; err != nil {
		return fmt.Errorf("clear graph report ports: %w", err)
	}
	rows := portRows(ports)
	for i := range rows {
		rows[i].ReportID = reportID
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("store graph report ports: %w", err)
	}
	return nil
}
