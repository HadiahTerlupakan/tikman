package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fakeLibreNMS serves synthetic port graphs, refusing the ifNames in failing.
type fakeLibreNMS struct {
	mu      sync.Mutex
	failing map[string]bool
}

func (f *fakeLibreNMS) fail(ifName string, failing bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing[ifName] = failing
}

func (f *fakeLibreNMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	parts := strings.Split(r.URL.Path, "/")
	ifName := parts[len(parts)-2]
	if f.failing[ifName] {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(buf.Bytes())
}

// recordingNotifier keeps what it was told was queued.
type recordingNotifier struct {
	queued []models.WAGroupMessage
}

func (n *recordingNotifier) GroupMessagesQueued(_ context.Context, msgs []models.WAGroupMessage) {
	n.queued = append(n.queued, msgs...)
}

type graphReportFixture struct {
	db        *gorm.DB
	reports   *GraphReportService
	sender    *GraphReportSender
	scheduler *GraphReportScheduler
	librenms  *fakeLibreNMS
	notifier  *recordingNotifier
	group     *models.WAGroup
	mediaRoot string
}

func newGraphReportFixture(t *testing.T) graphReportFixture {
	t.Helper()
	db := setupTestDB(t)
	account := csAccount(t, db)
	group, err := NewWAGroupService(db).Upsert(account.ID, groupSnap("9@g.us", "TEST MRTG", "62811"))
	require.NoError(t, err)

	fake := &fakeLibreNMS{failing: map[string]bool{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	settings := NewSettingService(db, settingServiceTestEncryptionKey)
	require.NoError(t, settings.Set(models.SettingLibreNMSURL, server.URL, uuid.Nil))
	require.NoError(t, settings.Set(models.SettingLibreNMSAPIToken, "token", uuid.Nil))

	notifier := &recordingNotifier{}
	mediaRoot := t.TempDir()
	sender := NewGraphReportSender(db, settings, mediaRoot, notifier)
	return graphReportFixture{
		db: db, reports: NewGraphReportService(db), sender: sender,
		scheduler: NewGraphReportScheduler(db, sender, zap.NewNop()), librenms: fake,
		notifier: notifier, group: group, mediaRoot: mediaRoot,
	}
}

func reportInput(groupID uuid.UUID, labels ...string) GraphReportInput {
	in := GraphReportInput{WAGroupID: groupID, CustomerName: "PT Contoh", Enabled: true}
	for _, label := range labels {
		in.Ports = append(in.Ports, GraphReportPortInput{
			DeviceID: 7, DeviceName: "core", IfName: "if-" + label, Label: label,
		})
	}
	return in
}

// createReport registers a report created the day before the tests' mornings,
// so the 09:00 rule treats it as one that was due.
func (f graphReportFixture) createReport(t *testing.T, labels ...string) *GraphReportView {
	t.Helper()
	report, err := f.reports.Create(reportInput(f.group.ID, labels...), uuid.New())
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).
		Update("created_at", time.Date(2026, 11, 1, 0, 0, 0, 0, wib)).Error)
	return report
}

func (f graphReportFixture) load(t *testing.T, id uuid.UUID) models.GraphReport {
	t.Helper()
	var report models.GraphReport
	require.NoError(t, f.db.First(&report, "id = ?", id).Error)
	return report
}

func (f graphReportFixture) queuedCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, f.db.Model(&models.WAGroupMessage{}).Where("status = ?", models.MessageQueued).Count(&count).Error)
	return count
}

// atWIB is a moment on the tests' calendar.
func atWIB(day, hour, minute int) time.Time {
	return time.Date(2026, 11, day, hour, minute, 0, 0, wib)
}
