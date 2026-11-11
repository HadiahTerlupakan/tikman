package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DefaultGraphReportSendTime is when a report goes out unless the admin picks
// another time, in WIB.
const DefaultGraphReportSendTime = "06:00"

// sendTimeLayout is the canonical "HH:MM" a send time is stored in.
const sendTimeLayout = "15:04"

// graphReportWindowLength is how long after its send time a report may still
// go out. A report not out by then waits for the next day: a "last 24 hours"
// graph arriving hours late reads as a mistake.
const graphReportWindowLength = 3 * time.Hour

// graphReportRetry spaces the attempts after a failure, so a LibreNMS that is
// down is not asked for every graph every minute.
const graphReportRetry = 10 * time.Minute

type graphReportAction int

const (
	graphReportIdle graphReportAction = iota
	graphReportDue
	// graphReportMissed is a report the window closed on without a single
	// attempt, typically because the api was down throughout it.
	graphReportMissed
)

// GraphReportScheduler sends each enabled report once a day, at its own time. All of
// its state lives on the report rows, so a restart neither skips nor repeats
// a day.
type GraphReportScheduler struct {
	db     *gorm.DB
	sender *GraphReportSender
	logger *zap.Logger
	now    func() time.Time
}

// NewGraphReportScheduler constructs a GraphReportScheduler.
func NewGraphReportScheduler(db *gorm.DB, sender *GraphReportSender, logger *zap.Logger) *GraphReportScheduler {
	return &GraphReportScheduler{db: db, sender: sender, logger: logger, now: time.Now}
}

// Run ticks until ctx ends.
func (s *GraphReportScheduler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		s.Tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick sends what is due, one report at a time; one report failing does not
// hold up the others. The clock is read again for each report, because the
// ones before it may have spent minutes waiting on LibreNMS.
func (s *GraphReportScheduler) Tick(ctx context.Context) {
	var reports []models.GraphReport
	if err := s.db.Where("enabled = ?", true).Order("created_at ASC").Find(&reports).Error; err != nil {
		s.logger.Warn("Could not read the graph reports", zap.Error(err))
		return
	}
	for i := range reports {
		report, now := &reports[i], s.now()
		switch dailyGraphAction(report, now) {
		case graphReportDue:
			if err := s.sender.attempt(ctx, report, now, true); err != nil {
				s.logger.Warn("Daily graph report failed",
					zap.String("report_id", report.ID.String()), zap.Error(err))
			}
		case graphReportMissed:
			s.sender.recordFailure(report.ID, now, s.missedReason(report, now))
		}
	}
}

func (s *GraphReportScheduler) missedReason(report *models.GraphReport, now time.Time) string {
	if _, err := s.sender.LibreNMS(); errors.Is(err, ErrLibreNMSNotConfigured) {
		return err.Error()
	}
	w := currentWindow(report, now)
	return fmt.Sprintf("laporan %s tidak terkirim: tidak ada percobaan kirim antara %s dan %s WIB",
		wibDay(w.day), w.clock(w.start), w.clock(w.end))
}

// dailyGraphAction decides what a report needs at now.
func dailyGraphAction(report *models.GraphReport, now time.Time) graphReportAction {
	w := currentWindow(report, now)
	if !report.Enabled || now.Before(w.start) || sentOn(report, w.day) {
		return graphReportIdle
	}
	attempted := report.LastAttemptAt != nil && !report.LastAttemptAt.Before(w.start)
	if !now.Before(w.end) {
		if attempted || !report.CreatedAt.Before(w.start) {
			return graphReportIdle
		}
		return graphReportMissed
	}
	if attempted && now.Sub(*report.LastAttemptAt) < graphReportRetry {
		return graphReportIdle
	}
	return graphReportDue
}

// sendWindow is the span of one WIB day in which a report may go out.
type sendWindow struct {
	day, start, end time.Time
}

// windowOn is the report's window on the WIB day starting at midnight day. It
// ends at that day's midnight at the latest, so a late send time never makes
// the report count for the next date.
func windowOn(report *models.GraphReport, day time.Time) sendWindow {
	start := day.Add(sendTimeOffset(report.SendTime))
	end := start.Add(graphReportWindowLength)
	if next := day.AddDate(0, 0, 1); end.After(next) {
		end = next
	}
	return sendWindow{day: day, start: start, end: end}
}

// currentWindow is today's window, or yesterday's while today's has not begun
// if that one ran up to midnight: its end is only ever seen on the next date,
// and a report missed in it would otherwise never be marked.
//
// Yesterday's window is only judged for a row untouched since it began. Every
// scheduler write sets last_attempt_at, so a later updated_at without an
// attempt is an admin edit, possibly the re-enable of a report that was off
// all through that window; blaming that day on it would be a false "missed".
func currentWindow(report *models.GraphReport, now time.Time) sendWindow {
	local := now.In(wib)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, wib)
	w := windowOn(report, today)
	if !now.Before(w.start) {
		return w
	}
	prev := windowOn(report, today.AddDate(0, 0, -1))
	if prev.end.Equal(today) && report.UpdatedAt.Before(prev.start) {
		return prev
	}
	return w
}

// clock writes a moment of the window as the admins read it, with the end of
// the day as 24:00 rather than the next day's 00:00.
func (w sendWindow) clock(t time.Time) string {
	if t.Equal(w.day.AddDate(0, 0, 1)) {
		return "24:00"
	}
	return t.In(wib).Format(sendTimeLayout)
}

// sendTimeOffset is how far into the day a send time falls. A value that does
// not parse, which only a row written outside the service can hold, sends at
// the default time rather than never.
func sendTimeOffset(value string) time.Duration {
	at, err := parseSendTime(value)
	if err != nil {
		at, _ = parseSendTime(DefaultGraphReportSendTime)
	}
	return time.Duration(at.Hour())*time.Hour + time.Duration(at.Minute())*time.Minute
}

// parseSendTime accepts exactly "HH:MM" on a 24-hour clock. The length check
// is what rejects "6:05": the layout alone would take a one-digit hour.
func parseSendTime(value string) (time.Time, error) {
	at, err := time.Parse(sendTimeLayout, value)
	if err != nil || len(value) != len(sendTimeLayout) {
		return time.Time{}, fmt.Errorf("%w: jam kirim harus berformat HH:MM", ErrValidation)
	}
	return at, nil
}

func sentOn(report *models.GraphReport, day time.Time) bool {
	return report.LastSentOn != nil && report.LastSentOn.UTC().Format(dateLayout) == day.Format(dateLayout)
}
