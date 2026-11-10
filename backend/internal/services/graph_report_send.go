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
// it, so the messages carry no sender user and nobody can edit them.
const GraphReportSenderName = "Laporan Otomatis"

// graphWindow is how far back each graph reaches.
const graphWindow = 24 * time.Hour

// graphFilenameRunes leaves room for the extension inside varchar(255).
const graphFilenameRunes = 200

var (
	// ErrLibreNMSNotConfigured is a send with no LibreNMS address or token.
	ErrLibreNMSNotConfigured = errors.New("LibreNMS belum diatur di Pengaturan")
	// ErrGraphFetch is LibreNMS failing to hand over a graph.
	ErrGraphFetch = errors.New("grafik gagal diambil dari LibreNMS")
)

// GroupQueueNotifier tells the browsers about queued group messages and wakes
// the wa process to send them. It lives outside this package because the wa
// package, which owns the channels, imports this one.
type GroupQueueNotifier interface {
	GroupMessagesQueued(ctx context.Context, msgs []models.WAGroupMessage)
}

// GraphReportSender fetches a report's graphs and queues them to its group.
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
// send before 06:00 does not stand in for that morning's report.
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
	client, err := s.LibreNMS()
	if err != nil {
		return err
	}
	var ports []models.GraphReportPort
	if err := s.db.Where("report_id = ?", report.ID).Order("position ASC").Find(&ports).Error; err != nil {
		return fmt.Errorf("load graph report ports: %w", err)
	}
	images, err := fetchGraphs(ctx, client, ports, now)
	if err != nil {
		return err
	}
	files, err := s.storeGraphs(ports, images, now)
	if err != nil {
		return err
	}
	return s.queue(ctx, report, ports, files, now, daily)
}

func fetchGraphs(ctx context.Context, client *librenms.Client, ports []models.GraphReportPort, now time.Time) ([][]byte, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("%w: laporan tidak punya port", ErrValidation)
	}
	images := make([][]byte, len(ports))
	for i, port := range ports {
		image, err := client.PortGraph(ctx, port.DeviceID, port.IfName, now.Add(-graphWindow), now)
		if err != nil {
			return nil, fmt.Errorf("%w: port %s (%s %s): %v", ErrGraphFetch, port.Label, port.DeviceName, port.IfName, err)
		}
		images[i] = image
	}
	return images, nil
}

// storeGraphs writes the images where the wa process reads attachments, under
// the <year>/<month>/<uuid> layout the inbox's uploads use.
func (s *GraphReportSender) storeGraphs(ports []models.GraphReportPort, images [][]byte, now time.Time) ([]MediaFile, error) {
	dir := filepath.Join(now.Format("2006"), now.Format("01"))
	if err := os.MkdirAll(filepath.Join(s.mediaRoot, dir), 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}
	files := make([]MediaFile, 0, len(images))
	for i, image := range images {
		rel := filepath.Join(dir, uuid.NewString()+".png")
		if err := os.WriteFile(filepath.Join(s.mediaRoot, rel), image, 0o640); err != nil {
			s.removeGraphs(files)
			return nil, fmt.Errorf("write graph: %w", err)
		}
		files = append(files, MediaFile{
			Path: rel, Mime: "image/png", Filename: clampRunes(ports[i].Label, graphFilenameRunes) + ".png",
			Size: int64(len(image)),
		})
	}
	return files, nil
}

func (s *GraphReportSender) removeGraphs(files []MediaFile) {
	for _, file := range files {
		_ = os.Remove(filepath.Join(s.mediaRoot, file.Path))
	}
}

// queue writes the messages and the report's success in one transaction, so a
// restart cannot see the messages queued but the day still unsent.
func (s *GraphReportSender) queue(
	ctx context.Context, report *models.GraphReport, ports []models.GraphReportPort, files []MediaFile,
	now time.Time, daily bool,
) error {
	members, _ := s.groups.MembersOtherThanUs(report.WAGroupID)
	outs := make([]GroupOutgoing, len(ports))
	for i, port := range ports {
		outs[i] = GroupOutgoing{
			GroupID: report.WAGroupID, SenderName: GraphReportSenderName, Kind: models.MessageKindImage,
			Body: graphCaption(report.CustomerName, port.Label, now, i == 0), Media: &files[i], MemberCount: members,
		}
	}
	fields := map[string]any{"last_attempt_at": now, "last_error": ""}
	if daily {
		fields["last_sent_on"] = wibDate(now)
	}
	msgs, err := s.messages.QueueBatch(outs, func(tx *gorm.DB) error {
		return tx.Model(&models.GraphReport{}).Where("id = ?", report.ID).Updates(fields).Error
	})
	if err != nil {
		s.removeGraphs(files)
		return fmt.Errorf("queue graph report: %w", err)
	}
	if s.notifier != nil {
		s.notifier.GroupMessagesQueued(ctx, msgs)
	}
	return nil
}

var indonesianMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// wibDay writes a date the way the captions do, "10 Nov".
func wibDay(t time.Time) string {
	local := t.In(wib)
	return fmt.Sprintf("%02d %s", local.Day(), indonesianMonths[local.Month()-1])
}

func wibStamp(t time.Time) string {
	return wibDay(t) + " " + t.In(wib).Format("15:04")
}

// graphCaption heads the first image with the report's title and window; the
// rest only name their port, since they arrive directly beneath it.
func graphCaption(customer, label string, now time.Time, first bool) string {
	port := "Port: " + label
	if !first {
		return port
	}
	return fmt.Sprintf("📊 Laporan Trafik Harian — %s\n%s – %s WIB\n%s",
		customer, wibStamp(now.Add(-graphWindow)), wibStamp(now), port)
}

// wibDate is the WIB calendar day of t at UTC midnight, the form last_sent_on
// is stored in (see models.GraphReport).
func wibDate(t time.Time) time.Time {
	local := t.In(wib)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
