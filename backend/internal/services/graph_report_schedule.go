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

// The daily window, in WIB. A report not out by the cutoff waits for the next
// morning: a "last 24 hours" graph arriving at noon reads as a mistake.
const (
	graphReportStartHour  = 6
	graphReportCutoffHour = 9
)

// graphReportRetry spaces the attempts after a failure, so a LibreNMS that is
// down is not asked for every graph every minute.
const graphReportRetry = 10 * time.Minute

type graphReportAction int

const (
	graphReportIdle graphReportAction = iota
	graphReportDue
	// graphReportMissed is a report the window closed on without a single
	// attempt, typically because the api was down all morning.
	graphReportMissed
)

// GraphReportScheduler sends each enabled report once every morning. All of
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
			s.sender.recordFailure(report.ID, now, s.missedReason(now))
		}
	}
}

func (s *GraphReportScheduler) missedReason(now time.Time) string {
	if _, err := s.sender.LibreNMS(); errors.Is(err, ErrLibreNMSNotConfigured) {
		return err.Error()
	}
	return fmt.Sprintf("laporan %s tidak terkirim: tidak ada percobaan kirim antara %02d:00 dan %02d:00 WIB",
		wibDay(now), graphReportStartHour, graphReportCutoffHour)
}

// dailyGraphAction decides what a report needs at now.
func dailyGraphAction(report *models.GraphReport, now time.Time) graphReportAction {
	local := now.In(wib)
	start := time.Date(local.Year(), local.Month(), local.Day(), graphReportStartHour, 0, 0, 0, wib)
	cutoff := time.Date(local.Year(), local.Month(), local.Day(), graphReportCutoffHour, 0, 0, 0, wib)
	if !report.Enabled || local.Before(start) || sentOn(report, now) {
		return graphReportIdle
	}
	attemptedToday := report.LastAttemptAt != nil && !report.LastAttemptAt.Before(start)
	if !local.Before(cutoff) {
		if attemptedToday || !report.CreatedAt.Before(start) {
			return graphReportIdle
		}
		return graphReportMissed
	}
	if attemptedToday && now.Sub(*report.LastAttemptAt) < graphReportRetry {
		return graphReportIdle
	}
	return graphReportDue
}

func sentOn(report *models.GraphReport, now time.Time) bool {
	return report.LastSentOn != nil &&
		report.LastSentOn.UTC().Format(dateLayout) == wibDate(now).Format(dateLayout)
}
