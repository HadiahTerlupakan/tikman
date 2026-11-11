package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/librenms"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/telegram"
	"gorm.io/gorm"
)

// GraphReportSenderName is who the group sees the report from. No user wrote
// it, so the message carries no sender user and nobody can edit it.
const GraphReportSenderName = "Laporan Otomatis"

// graphWindow is how far back each graph reaches.
const graphWindow = 24 * time.Hour

// graphFilenameRunes leaves room for the extension inside varchar(255).
const graphFilenameRunes = 200

// graphFilenameDateLayout dates the image's filename, "PT Contoh 11-11-2026".
const graphFilenameDateLayout = "02-01-2006"

// graphFilenamePartFormat numbers the images of a report sent as several,
// "PT Contoh 11-11-2026 (2 dari 5)".
const graphFilenamePartFormat = "%s (%d dari %d)"

var (
	// ErrLibreNMSNotConfigured is a send with no LibreNMS address or token.
	ErrLibreNMSNotConfigured = errors.New("LibreNMS belum diatur di Pengaturan")
	// ErrGraphFetch is LibreNMS failing to hand over a graph.
	ErrGraphFetch = errors.New("grafik gagal diambil dari LibreNMS")
	// ErrGraphReportTooManyMentions is a tag-everyone report for a group past
	// what one message may tag.
	ErrGraphReportTooManyMentions = fmt.Errorf("grup terlalu besar untuk tag semua (maks %d anggota)", maxMentions)
)

// GroupQueueNotifier tells the browsers about queued group messages and wakes
// the wa process to send them. It lives outside this package because the wa
// package, which owns the channels, imports this one.
type GroupQueueNotifier interface {
	GroupMessagesQueued(ctx context.Context, msgs []models.WAGroupMessage)
}

// GraphReportSender fetches a report's graphs and sends them, as images of
// graphPortsPerImage ports each, to its WhatsApp group and its Telegram chat.
type GraphReportSender struct {
	db        *gorm.DB
	settings  *SettingService
	messages  *WAGroupMessageService
	groups    *WAGroupService
	chats     *TelegramChatService
	mediaRoot string
	notifier  GroupQueueNotifier
	telegram  func(token string) *telegram.Client
	now       func() time.Time
}

// NewGraphReportSender constructs a GraphReportSender. mediaRoot is the
// directory the wa process reads attachments from.
func NewGraphReportSender(
	db *gorm.DB, settings *SettingService, mediaRoot string, notifier GroupQueueNotifier,
) *GraphReportSender {
	return &GraphReportSender{
		db: db, settings: settings, messages: NewWAGroupMessageService(db), groups: NewWAGroupService(db),
		chats: NewTelegramChatService(db), mediaRoot: mediaRoot, notifier: notifier, telegram: telegram.New,
		now: time.Now,
	}
}

// LibreNMS answers a client for the configured server.
func (s *GraphReportSender) LibreNMS() (*librenms.Client, error) {
	base, err := s.settings.Value(models.SettingLibreNMSURL)
	if errors.Is(err, ErrSettingNotConfigured) {
		return nil, ErrLibreNMSNotConfigured
	}
	if err != nil {
		return nil, err
	}
	token, err := s.settings.Value(models.SettingLibreNMSAPIToken)
	if errors.Is(err, ErrSettingNotConfigured) {
		return nil, ErrLibreNMSNotConfigured
	}
	if err != nil {
		return nil, err
	}
	return librenms.New(base, token), nil
}

// SendNow sends a report on demand to every destination it has. It leaves
// the sent-on dates alone, so a test send before the send time does not stand
// in for that day's report.
func (s *GraphReportSender) SendNow(ctx context.Context, id uuid.UUID) error {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return err
	}
	return s.attempt(ctx, &report, s.now(), false)
}

// attempt sends a report and records the outcome on it. daily marks the send
// as that day's report: a success means every destination has it, the ones
// skipped as already sent included.
func (s *GraphReportSender) attempt(ctx context.Context, report *models.GraphReport, now time.Time, daily bool) error {
	err := s.deliver(ctx, report, now, daily)
	var failures sendFailures
	uncertain := errors.As(err, &failures) && failures.onlyUncertain()
	if err != nil && !uncertain {
		s.recordFailure(report.ID, now, err.Error())
		return err
	}
	// A send that may have arrived ends the day like a success, keeping its
	// warning: a retry would find nothing owed and wipe it.
	fields := map[string]any{"last_attempt_at": now, "last_error": ""}
	if uncertain {
		fields["last_error"] = err.Error()
	}
	if daily {
		fields["last_sent_on"] = wibDate(now)
	}
	// As with a failure, the send itself is what the caller needs to hear of.
	_ = s.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).Updates(fields).Error
	return err
}

func (s *GraphReportSender) recordFailure(id uuid.UUID, now time.Time, reason string) {
	// A failure to record is not worth more than the failure it records; the
	// caller already has the original error. UpdateColumns leaves updated_at
	// alone: the schedule reads it as the admin's last edit, and a recorded
	// failure is not one.
	_ = s.db.Model(&models.GraphReport{}).Where("id = ?", id).
		UpdateColumns(map[string]any{"last_attempt_at": now, "last_error": reason}).Error
}

// deliver renders the report once and sends it to each destination still
// owed it; a daily send skips one that already has the day's report, so a
// retry after Telegram failed does not post into the WhatsApp group twice.
// One destination failing does not hold up the other, but every graph is
// fetched before anything is sent, so no destination receives half a report
// that reads as whole.
func (s *GraphReportSender) deliver(ctx context.Context, report *models.GraphReport, now time.Time, daily bool) error {
	today := wibDate(now)
	failures := sendFailures{named: report.WAGroupID != nil && report.TelegramChatID != nil}
	var wa *waTarget
	if owed(report.WAGroupID, report.WASentOn, today, daily) {
		target, err := s.prepareWA(report)
		if err != nil {
			failures.add(destinationWhatsApp, err)
		}
		wa = target
	}
	var tg *telegramTarget
	if owed(report.TelegramChatID, report.TelegramSentOn, today, daily) {
		target, err := s.prepareTelegram(report)
		if err != nil {
			failures.add(destinationTelegram, err)
		}
		tg = target
	}
	if wa == nil && tg == nil {
		return failures.orNil()
	}
	images, err := s.render(ctx, report, now)
	if err != nil {
		failures.add("", err)
		return failures
	}
	if wa != nil {
		failures.record(destinationWhatsApp, s.sendWA(ctx, report, wa, images, now, daily))
	}
	if tg != nil {
		failures.record(destinationTelegram, s.sendTelegram(ctx, report, tg, images, now, daily))
	}
	return failures.orNil()
}

// owed says whether a destination is to get this send: any it has on a
// manual send, and on a daily one only if it lacks today's report.
func owed(destination *uuid.UUID, sentOn *time.Time, today time.Time, daily bool) bool {
	return destination != nil && (!daily || !sameDate(sentOn, today))
}

func sameDate(day *time.Time, date time.Time) bool {
	return day != nil && day.UTC().Format(dateLayout) == date.Format(dateLayout)
}

func (s *GraphReportSender) render(ctx context.Context, report *models.GraphReport, now time.Time) ([]ReportImage, error) {
	client, err := s.LibreNMS()
	if err != nil {
		return nil, err
	}
	var ports []models.GraphReportPort
	if err := s.db.Where("report_id = ?", report.ID).Order("position ASC").Find(&ports).Error; err != nil {
		return nil, fmt.Errorf("load graph report ports: %w", err)
	}
	return renderReport(ctx, client, ports, now)
}

// reportFileBase is what a report's images are named for, "PT Contoh 11-11-2026".
func reportFileBase(customer string, now time.Time) string {
	return customer + " " + now.In(wib).Format(graphFilenameDateLayout)
}

// imageFilename numbers the image only when the report is sent as several,
// so a short report keeps the plain "PT Contoh 11-11-2026.png".
func imageFilename(base string, index, total int) string {
	if total > 1 {
		base = fmt.Sprintf(graphFilenamePartFormat, base, index+1, total)
	}
	return clampRunes(base, graphFilenameRunes) + ".png"
}

var indonesianMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// wibDay writes a date the way the scheduler's messages do, "10 Nov".
func wibDay(t time.Time) string {
	local := t.In(wib)
	return fmt.Sprintf("%02d %s", local.Day(), indonesianMonths[local.Month()-1])
}

// wibDate is the WIB calendar day of t at UTC midnight, the form last_sent_on
// is stored in (see models.GraphReport).
func wibDate(t time.Time) time.Time {
	local := t.In(wib)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
