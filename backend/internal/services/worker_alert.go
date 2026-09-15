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

// workerAlertStaleAfter is how long the poller may go without a heartbeat
// before the admins are told. It sits above the three missed beats /health
// already reports as a dead worker: a deploy stops the worker for about forty
// seconds, and an alert that fires on every deploy teaches people to ignore it.
const workerAlertStaleAfter = 5 * time.Minute

// workerAlertRemindEvery repeats the alert while the worker stays down. One
// notification is easy to miss: after the reboot of 2026-09-12 the poller was
// dead for more than two days before anyone looked.
const workerAlertRemindEvery = 3 * time.Hour

// workerAlertURL is where tapping the notification lands. Without it the
// service worker opens the CS inbox, which is what every other push is about.
const workerAlertURL = "/"

// wib is the zone the admins read times in. A fixed offset rather than
// time.LoadLocation: Indonesia keeps no daylight saving, and the api image
// ships without tzdata.
var wib = time.FixedZone("WIB", 7*60*60)

// WorkerAlertService pushes a notification to the admins when the polling
// worker stops beating, reminds them while it stays down, and says when it is
// back. It runs inside the API because the API outlives the worker: the
// failure it exists for is a worker that died while everything else kept
// answering.
//
// Its state lives in memory, so an API restart during an outage repeats the
// alert — erring toward being heard.
type WorkerAlertService struct {
	db            *gorm.DB
	sender        PushSender
	subscriptions *PushService
	logger        *zap.Logger

	// downSince is the last beat before the worker went quiet, zero while it is
	// beating; lastAlert is when the admins were last told it is down.
	downSince time.Time
	lastAlert time.Time
}

// NewWorkerAlertService creates the alerter. sender must not be nil: without
// push notifications there is nobody to tell.
func NewWorkerAlertService(db *gorm.DB, sender PushSender, subscriptions *PushService, logger *zap.Logger) *WorkerAlertService {
	return &WorkerAlertService{db: db, sender: sender, subscriptions: subscriptions, logger: logger}
}

// Run checks the heartbeat every interval until ctx ends.
func (s *WorkerAlertService) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := s.Check(ctx, now); err != nil {
				s.logger.Warn("Could not alert on the worker heartbeat", zap.Error(err))
			}
		}
	}
}

// Check compares the poller's last heartbeat with now and tells the admins
// whatever changed. A worker that has never beaten is left alone: nothing has
// stopped.
func (s *WorkerAlertService) Check(ctx context.Context, now time.Time) error {
	var beat models.WorkerHeartbeat
	err := s.db.WithContext(ctx).First(&beat, "name = ?", models.WorkerHeartbeatPoller).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read worker heartbeat: %w", err)
	}

	stale := now.Sub(beat.BeatAt) > workerAlertStaleAfter
	switch {
	case stale && s.lastAlert.IsZero():
		return s.alertDown(ctx, now, beat.BeatAt, "Polling OLT berhenti")
	case stale && now.Sub(s.lastAlert) >= workerAlertRemindEvery:
		return s.alertDown(ctx, now, beat.BeatAt, "Polling OLT masih berhenti")
	case !stale && !s.downSince.IsZero():
		body := fmt.Sprintf("Worker sempat berhenti %s.", durationInIndonesian(beat.BeatAt.Sub(s.downSince)))
		if err := s.notify(ctx, "Polling OLT jalan lagi", body); err != nil {
			return err
		}
		s.downSince, s.lastAlert = time.Time{}, time.Time{}
	}
	return nil
}

func (s *WorkerAlertService) alertDown(ctx context.Context, now, lastBeat time.Time, title string) error {
	body := fmt.Sprintf("Heartbeat worker terakhir %s (%s lalu). Status ONT tidak diperbarui sampai worker jalan lagi.",
		lastBeat.In(wib).Format("02/01 15:04 MST"), durationInIndonesian(now.Sub(lastBeat)))
	if err := s.notify(ctx, title, body); err != nil {
		return err
	}
	s.downSince, s.lastAlert = lastBeat, now
	return nil
}

func (s *WorkerAlertService) notify(ctx context.Context, title, body string) error {
	fids, err := s.subscriptions.FIDsForRoles(models.UserRoleAdmin)
	if err != nil {
		return fmt.Errorf("list admin push FIDs: %w", err)
	}
	devices := 0
	if len(fids) > 0 {
		if devices, err = pushAndPrune(ctx, s.sender, s.subscriptions, fids, title, body, map[string]string{"url": workerAlertURL}); err != nil {
			return err
		}
	}
	s.logger.Info("Sent worker heartbeat alert", zap.String("title", title), zap.Int("devices", devices))
	return nil
}

// durationInIndonesian renders a duration the way the notification reads, to
// the minute: "7 menit", "3 jam 5 menit", "2 hari 13 jam".
func durationInIndonesian(d time.Duration) string {
	const day = 24 * time.Hour
	days, hours, minutes := int(d/day), int(d%day/time.Hour), int(d%time.Hour/time.Minute)
	switch {
	case days > 0:
		return fmt.Sprintf("%d hari %d jam", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d jam %d menit", hours, minutes)
	default:
		return fmt.Sprintf("%d menit", minutes)
	}
}
