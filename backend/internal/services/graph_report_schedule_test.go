package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestDailyGraphAction(t *testing.T) {
	yesterday := wibDate(atWIB(10, 12, 0))
	today := wibDate(atWIB(11, 12, 0))
	created := atWIB(1, 0, 0)
	at := func(day, hour, minute int) *time.Time { v := atWIB(day, hour, minute); return &v }

	cases := []struct {
		name   string
		report models.GraphReport
		now    time.Time
		want   graphReportAction
	}{
		{"before 06:00", models.GraphReport{Enabled: true, CreatedAt: created}, atWIB(11, 5, 59), graphReportIdle},
		{"06:00 sends", models.GraphReport{Enabled: true, CreatedAt: created, LastSentOn: &yesterday}, atWIB(11, 6, 0), graphReportDue},
		{"disabled", models.GraphReport{CreatedAt: created}, atWIB(11, 6, 0), graphReportIdle},
		{"already sent today", models.GraphReport{Enabled: true, CreatedAt: created, LastSentOn: &today}, atWIB(11, 6, 1), graphReportIdle},
		{"failed 5 minutes ago", models.GraphReport{Enabled: true, CreatedAt: created, LastAttemptAt: at(11, 6, 0)}, atWIB(11, 6, 5), graphReportIdle},
		{"failed 10 minutes ago", models.GraphReport{Enabled: true, CreatedAt: created, LastAttemptAt: at(11, 6, 0)}, atWIB(11, 6, 10), graphReportDue},
		{"manual send before 06:00", models.GraphReport{Enabled: true, CreatedAt: created, LastAttemptAt: at(11, 5, 55)}, atWIB(11, 6, 0), graphReportDue},
		{"09:00 after a failure", models.GraphReport{Enabled: true, CreatedAt: created, LastAttemptAt: at(11, 8, 55)}, atWIB(11, 9, 5), graphReportIdle},
		{"09:00 with no attempt", models.GraphReport{Enabled: true, CreatedAt: created}, atWIB(11, 9, 0), graphReportMissed},
		{"created after 06:00", models.GraphReport{Enabled: true, CreatedAt: atWIB(11, 10, 0)}, atWIB(11, 10, 1), graphReportIdle},
		{"next day", models.GraphReport{Enabled: true, CreatedAt: created, LastSentOn: &today}, atWIB(12, 6, 0), graphReportDue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dailyGraphAction(&tc.report, tc.now))
		})
	}
}

// One report through a morning with a LibreNMS outage, the cutoff and the
// next day, counting what reached the queue.
func TestSchedulerSendsOncePerMorning(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	ctx := context.Background()
	tick := func(day, hour, minute int) int64 {
		f.scheduler.now = func() time.Time { return atWIB(day, hour, minute) }
		f.scheduler.Tick(ctx)
		return f.queuedCount(t)
	}

	assert.Zero(t, tick(11, 5, 59), "nothing before 06:00")

	f.librenms.fail("if-Uplink", true)
	assert.Zero(t, tick(11, 6, 0), "LibreNMS down")
	assert.Contains(t, f.load(t, report.ID).LastError, "Uplink")
	f.librenms.fail("if-Uplink", false)
	assert.Zero(t, tick(11, 6, 9), "no retry within 10 minutes")
	assert.Equal(t, int64(1), tick(11, 6, 10), "retried after 10 minutes")
	stored := f.load(t, report.ID)
	require.NotNil(t, stored.LastSentOn)
	assert.Equal(t, "2026-11-11", stored.LastSentOn.UTC().Format(dateLayout))
	assert.Empty(t, stored.LastError)

	assert.Equal(t, int64(1), tick(11, 6, 30), "no second send the same day")
	assert.Equal(t, int64(2), tick(12, 6, 0), "next morning sends again")
}

func TestSchedulerLeavesAReportUnsentPastTheCutoff(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	ctx := context.Background()

	f.scheduler.now = func() time.Time { return atWIB(11, 9, 0) }
	f.scheduler.Tick(ctx)
	assert.Zero(t, f.queuedCount(t))
	missed := f.load(t, report.ID)
	assert.Contains(t, missed.LastError, "11 Nov tidak terkirim")

	f.scheduler.now = func() time.Time { return atWIB(11, 9, 30) }
	f.scheduler.Tick(ctx)
	assert.Zero(t, f.queuedCount(t))
	assert.Equal(t, missed.LastAttemptAt.Unix(), f.load(t, report.ID).LastAttemptAt.Unix(), "recorded once")
}

func TestManualSendDoesNotStandInForTheMorningReport(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	f.sender.now = func() time.Time { return atWIB(11, 5, 30) }

	require.NoError(t, f.sender.SendNow(context.Background(), report.ID))
	assert.Nil(t, f.load(t, report.ID).LastSentOn)

	f.scheduler.now = func() time.Time { return atWIB(11, 6, 0) }
	f.scheduler.Tick(context.Background())
	assert.Equal(t, int64(2), f.queuedCount(t))
}

// A tick that starts at 08:59 but reaches a report after 09:00, because the
// reports before it waited on LibreNMS, must treat that report as past the
// cutoff rather than send it late under an 08:59 caption.
func TestTickReadsTheClockForEachReport(t *testing.T) {
	f := newGraphReportFixture(t)
	first := f.createReport(t, "Uplink")
	second, err := NewWAGroupService(f.db).Upsert(f.group.WAAccountID, groupSnap("8@g.us", "Kedua", "62811"))
	require.NoError(t, err)
	late, err := f.reports.Create(reportInput(second.ID, "Backup"), uuid.New())
	require.NoError(t, err)
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", late.ID).
		Updates(map[string]any{"created_at": atWIB(2, 0, 0)}).Error)

	clock := []time.Time{atWIB(11, 8, 59), atWIB(11, 9, 20)}
	f.scheduler.now = func() time.Time {
		now := clock[0]
		if len(clock) > 1 {
			clock = clock[1:]
		}
		return now
	}
	f.scheduler.Tick(context.Background())

	assert.Equal(t, int64(1), f.queuedCount(t), "only the report reached before 09:00 is sent")
	assert.NotNil(t, f.load(t, first.ID).LastSentOn)
	missed := f.load(t, late.ID)
	assert.Nil(t, missed.LastSentOn)
	require.NotNil(t, missed.LastAttemptAt)
	assert.Equal(t, atWIB(11, 9, 20).Unix(), missed.LastAttemptAt.Unix())
}
