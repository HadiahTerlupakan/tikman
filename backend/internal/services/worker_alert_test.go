package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type sentPush struct {
	fids  []string
	title string
	body  string
	data  map[string]string
}

// recordingPushSender keeps every send, where FakePushSender keeps only the
// last: an alert is about how many notifications went out over time.
type recordingPushSender struct {
	sent []sentPush
	err  error
}

func (r *recordingPushSender) SendEach(_ context.Context, fids []string, title, body string, data map[string]string) ([]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.sent = append(r.sent, sentPush{fids: fids, title: title, body: body, data: data})
	return nil, nil
}

// lastBeat is 12 Sep 17:33 WIB, the last heartbeat before the outage these
// alerts exist for.
var lastBeat = time.Date(2026, 9, 12, 10, 33, 0, 0, time.UTC)

func workerAlertSetup(t *testing.T) (*WorkerAlertService, *recordingPushSender, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	users := NewUserService(db)
	subscriptions := NewPushService(db)

	admin, err := users.Create("admin1", "admin1@example.com", "password123", "", models.UserRoleAdmin)
	require.NoError(t, err)
	require.NoError(t, subscriptions.Subscribe(admin.ID, "admin-fid"))
	cs, err := users.Create("cs1", "cs1@example.com", "password123", "", models.UserRoleCS)
	require.NoError(t, err)
	require.NoError(t, subscriptions.Subscribe(cs.ID, "cs-fid"))

	sender := &recordingPushSender{}
	return NewWorkerAlertService(db, sender, subscriptions, zap.NewNop()), sender, db
}

func beatAt(t *testing.T, db *gorm.DB, at time.Time) {
	t.Helper()
	require.NoError(t, db.Save(&models.WorkerHeartbeat{Name: models.WorkerHeartbeatPoller, BeatAt: at}).Error)
}

func check(t *testing.T, alerts *WorkerAlertService, at time.Time) {
	t.Helper()
	require.NoError(t, alerts.Check(context.Background(), at))
}

// A deploy stops the worker for about forty seconds, so a short gap must stay
// quiet; past five minutes the admins hear it once, not every minute after.
func TestWorkerAlertTellsTheAdminsOnceWhenTheWorkerStops(t *testing.T) {
	alerts, sender, db := workerAlertSetup(t)
	beatAt(t, db, lastBeat)

	check(t, alerts, lastBeat.Add(4*time.Minute))
	assert.Empty(t, sender.sent, "a gap a deploy can cause must not alert")

	check(t, alerts, lastBeat.Add(6*time.Minute))
	check(t, alerts, lastBeat.Add(7*time.Minute))

	require.Len(t, sender.sent, 1)
	alert := sender.sent[0]
	assert.Equal(t, []string{"admin-fid"}, alert.fids, "only admins are told")
	assert.Equal(t, "Polling OLT berhenti", alert.title)
	assert.Contains(t, alert.body, "12/09 17:33 WIB")
	assert.Equal(t, "/", alert.data["url"], "the tap opens the dashboard, not the CS inbox")
}

// One notification is easy to miss: the worker that stopped on 2026-09-12 was
// dead for more than two days before anyone looked.
func TestWorkerAlertRemindsWhileTheWorkerStaysDown(t *testing.T) {
	alerts, sender, db := workerAlertSetup(t)
	beatAt(t, db, lastBeat)
	firstAlert := lastBeat.Add(6 * time.Minute)

	check(t, alerts, firstAlert)
	check(t, alerts, firstAlert.Add(2*time.Hour+59*time.Minute))
	require.Len(t, sender.sent, 1)

	check(t, alerts, firstAlert.Add(3*time.Hour))
	require.Len(t, sender.sent, 2)
	assert.Equal(t, "Polling OLT masih berhenti", sender.sent[1].title)
	assert.Contains(t, sender.sent[1].body, "12/09 17:33 WIB")
}

func TestWorkerAlertSaysOnceWhenTheWorkerIsBack(t *testing.T) {
	alerts, sender, db := workerAlertSetup(t)
	beatAt(t, db, lastBeat)
	check(t, alerts, lastBeat.Add(6*time.Minute))

	back := lastBeat.Add(61*time.Hour + 3*time.Minute)
	beatAt(t, db, back)
	check(t, alerts, back.Add(10*time.Second))
	check(t, alerts, back.Add(time.Minute))

	require.Len(t, sender.sent, 2)
	assert.Equal(t, "Polling OLT jalan lagi", sender.sent[1].title)
	assert.Contains(t, sender.sent[1].body, "2 hari 13 jam")
}

// A push that failed to go out has told nobody, so the next check must try
// again rather than count it as sent.
func TestWorkerAlertRetriesAnAlertThatDidNotGoOut(t *testing.T) {
	alerts, sender, db := workerAlertSetup(t)
	beatAt(t, db, lastBeat)

	sender.err = errors.New("fcm unreachable")
	assert.Error(t, alerts.Check(context.Background(), lastBeat.Add(6*time.Minute)))

	sender.err = nil
	check(t, alerts, lastBeat.Add(7*time.Minute))
	require.Len(t, sender.sent, 1)
	assert.Equal(t, "Polling OLT berhenti", sender.sent[0].title)
}

func TestWorkerAlertIgnoresAWorkerThatHasNeverRun(t *testing.T) {
	alerts, sender, _ := workerAlertSetup(t)

	check(t, alerts, lastBeat)

	assert.Empty(t, sender.sent)
}

func TestDurationInIndonesian(t *testing.T) {
	cases := map[time.Duration]string{
		7*time.Minute + 40*time.Second: "7 menit",
		3*time.Hour + 5*time.Minute:    "3 jam 5 menit",
		61*time.Hour + 3*time.Minute:   "2 hari 13 jam",
	}
	for duration, want := range cases {
		assert.Equal(t, want, durationInIndonesian(duration))
	}
}
