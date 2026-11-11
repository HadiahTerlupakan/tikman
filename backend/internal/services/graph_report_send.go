package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/librenms"
	"github.com/tikman/olt-provisioning/internal/models"
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

// GraphReportSender fetches a report's graphs and queues them to its group
// as one image.
type GraphReportSender struct {
	db        *gorm.DB
	settings  *SettingService
	messages  *WAGroupMessageService
	groups    *WAGroupService
	mediaRoot string
	notifier  GroupQueueNotifier
	now       func() time.Time
}

// NewGraphReportSender constructs a GraphReportSender. mediaRoot is the
// directory the wa process reads attachments from.
func NewGraphReportSender(
	db *gorm.DB, settings *SettingService, mediaRoot string, notifier GroupQueueNotifier,
) *GraphReportSender {
	return &GraphReportSender{
		db: db, settings: settings, messages: NewWAGroupMessageService(db), groups: NewWAGroupService(db),
		mediaRoot: mediaRoot, notifier: notifier, now: time.Now,
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

// SendNow sends a report on demand. It leaves last_sent_on alone, so a test
// send before the send time does not stand in for that day's report.
func (s *GraphReportSender) SendNow(ctx context.Context, id uuid.UUID) error {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return err
	}
	return s.attempt(ctx, &report, s.now(), false)
}

// attempt sends a report and records the outcome on it. daily marks the send
// as that day's report.
func (s *GraphReportSender) attempt(ctx context.Context, report *models.GraphReport, now time.Time, daily bool) error {
	err := s.deliver(ctx, report, now, daily)
	if err != nil {
		s.recordFailure(report.ID, now, err.Error())
	}
	return err
}

func (s *GraphReportSender) recordFailure(id uuid.UUID, now time.Time, reason string) {
	// A failure to record is not worth more than the failure it records; the
	// caller already has the original error.
	_ = s.db.Model(&models.GraphReport{}).Where("id = ?", id).
		Updates(map[string]any{"last_attempt_at": now, "last_error": reason}).Error
}

// deliver is all or nothing: every graph is fetched before anything is
// queued, so a group never receives half a report that reads as whole.
func (s *GraphReportSender) deliver(ctx context.Context, report *models.GraphReport, now time.Time, daily bool) error {
	group, err := s.groups.Get(report.WAGroupID)
	if err != nil {
		return fmt.Errorf("load group: %w", err)
	}
	if err := CheckGroupSendable(group); err != nil {
		return err
	}
	mentions, err := s.mentions(report)
	if err != nil {
		return err
	}
	client, err := s.LibreNMS()
	if err != nil {
		return err
	}
	var ports []models.GraphReportPort
	if err := s.db.Where("report_id = ?", report.ID).Order("position ASC").Find(&ports).Error; err != nil {
		return fmt.Errorf("load graph report ports: %w", err)
	}
	image, err := renderReport(ctx, client, ports, now)
	if err != nil {
		return err
	}
	file, err := s.storeImage(report.CustomerName, image, now)
	if err != nil {
		return err
	}
	return s.queue(ctx, report, file, mentions, now, daily)
}

// mentions answers who the report tags, read at send time so it follows the
// group as members come and go. The caption is left as written: WhatsApp
// notifies every tagged member whether or not the text names them.
func (s *GraphReportSender) mentions(report *models.GraphReport) ([]string, error) {
	if !report.MentionAll {
		return nil, nil
	}
	jids, err := s.groups.MemberJIDsOtherThanUs(report.WAGroupID)
	if err != nil {
		return nil, err
	}
	// Failing beats sending untagged: a report quietly missing its tags
	// reads as sent while nobody muted was told.
	if len(jids) > maxMentions {
		return nil, ErrGraphReportTooManyMentions
	}
	return jids, nil
}

// storeImage writes the report where the wa process reads attachments, under
// the <year>/<month>/<uuid> layout the inbox's uploads use.
func (s *GraphReportSender) storeImage(customer string, image []byte, now time.Time) (MediaFile, error) {
	dir := filepath.Join(now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(filepath.Join(s.mediaRoot, dir), 0o750); err != nil {
		return MediaFile{}, fmt.Errorf("create media directory: %w", err)
	}
	rel := filepath.Join(dir, uuid.NewString()+".png")
	if err := os.WriteFile(filepath.Join(s.mediaRoot, rel), image, 0o640); err != nil {
		return MediaFile{}, fmt.Errorf("write report image: %w", err)
	}
	name := customer + " " + now.In(wib).Format(graphFilenameDateLayout)
	return MediaFile{
		Path: rel, Mime: "image/png", Filename: clampRunes(name, graphFilenameRunes) + ".png",
		Size: int64(len(image)),
	}, nil
}

// queue writes the message and the report's success in one transaction, so a
// restart cannot see the message queued but the day still unsent.
func (s *GraphReportSender) queue(
	ctx context.Context, report *models.GraphReport, file MediaFile, mentions []string, now time.Time, daily bool,
) error {
	members, _ := s.groups.MembersOtherThanUs(report.WAGroupID)
	out := GroupOutgoing{
		GroupID: report.WAGroupID, SenderName: GraphReportSenderName, Kind: models.MessageKindImage,
		Body: GraphReportCaption(report.Message, report.CustomerName), Media: &file, MemberCount: members,
		Mentions: mentions,
	}
	fields := map[string]any{"last_attempt_at": now, "last_error": ""}
	if daily {
		fields["last_sent_on"] = wibDate(now)
	}
	msgs, err := s.messages.QueueBatch([]GroupOutgoing{out}, func(tx *gorm.DB) error {
		return tx.Model(&models.GraphReport{}).Where("id = ?", report.ID).Updates(fields).Error
	})
	if err != nil {
		_ = os.Remove(filepath.Join(s.mediaRoot, file.Path))
		return fmt.Errorf("queue graph report: %w", err)
	}
	if s.notifier != nil {
		s.notifier.GroupMessagesQueued(ctx, msgs)
	}
	return nil
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
