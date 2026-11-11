package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestDailyGraphActionAtACustomTime(t *testing.T) {
	before := wibDate(atWIB(9, 12, 0))
	yesterday := wibDate(atWIB(10, 12, 0))
	today := wibDate(atWIB(11, 12, 0))
	created := atWIB(1, 0, 0)
	at := func(day, hour, minute int) *time.Time { v := atWIB(day, hour, minute); return &v }
	afternoon := func(r models.GraphReport) models.GraphReport { r.SendTime, r.Enabled = "14:30", true; return r }
	late := func(r models.GraphReport) models.GraphReport { r.SendTime, r.Enabled = "23:30", true; return r }

	cases := []struct {
		name   string
		report models.GraphReport
		now    time.Time
		want   graphReportAction
	}{
		{"14:29", afternoon(models.GraphReport{CreatedAt: created, LastSentOn: &yesterday}), atWIB(11, 14, 29), graphReportIdle},
		{"14:30 sends", afternoon(models.GraphReport{CreatedAt: created, LastSentOn: &yesterday}), atWIB(11, 14, 30), graphReportDue},
		{"14:30 failed 5 minutes ago", afternoon(models.GraphReport{CreatedAt: created, LastAttemptAt: at(11, 14, 30)}), atWIB(11, 14, 35), graphReportIdle},
		{"14:30 failed 10 minutes ago", afternoon(models.GraphReport{CreatedAt: created, LastAttemptAt: at(11, 14, 30)}), atWIB(11, 14, 40), graphReportDue},
		{"attempt at 06:00 does not block 14:30", afternoon(models.GraphReport{CreatedAt: created, LastAttemptAt: at(11, 6, 0)}), atWIB(11, 14, 30), graphReportDue},
		{"14:30 open at 17:29", afternoon(models.GraphReport{CreatedAt: created}), atWIB(11, 17, 29), graphReportDue},
		{"14:30 missed at 17:30", afternoon(models.GraphReport{CreatedAt: created}), atWIB(11, 17, 30), graphReportMissed},
		{"14:30 failed before 17:30", afternoon(models.GraphReport{CreatedAt: created, LastAttemptAt: at(11, 17, 25)}), atWIB(11, 17, 30), graphReportIdle},
		{"14:30 created after 14:30", afternoon(models.GraphReport{CreatedAt: atWIB(11, 15, 0)}), atWIB(11, 17, 30), graphReportIdle},
		{"23:30 open at 23:59", late(models.GraphReport{CreatedAt: created, LastSentOn: &yesterday}), atWIB(11, 23, 59), graphReportDue},
		{"23:30 missed after midnight", late(models.GraphReport{CreatedAt: created, LastSentOn: &before}), atWIB(12, 0, 5), graphReportMissed},
		{"23:30 failed before midnight", late(models.GraphReport{CreatedAt: created, LastAttemptAt: at(11, 23, 55)}), atWIB(12, 0, 5), graphReportIdle},
		{"23:30 sent, next day before 23:30", late(models.GraphReport{CreatedAt: created, LastSentOn: &today}), atWIB(12, 0, 5), graphReportIdle},
		{"23:30 next day at 23:30", late(models.GraphReport{CreatedAt: created, LastSentOn: &today}), atWIB(12, 23, 30), graphReportDue},
		{"23:30 created after yesterday's window", late(models.GraphReport{CreatedAt: atWIB(11, 23, 45)}), atWIB(12, 0, 5), graphReportIdle},
		{"23:30 re-enabled after yesterday's window", late(models.GraphReport{CreatedAt: created, UpdatedAt: atWIB(12, 0, 1)}), atWIB(12, 0, 5), graphReportIdle},
		{"23:30 edited before yesterday's window", late(models.GraphReport{CreatedAt: created, UpdatedAt: atWIB(11, 12, 0)}), atWIB(12, 0, 5), graphReportMissed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dailyGraphAction(&tc.report, tc.now))
		})
	}
}

// createReportAt is createReport sending at sendTime, in a group of its own
// named jid so several can share one fixture.
func (f graphReportFixture) createReportAt(t *testing.T, jid, sendTime, label string) *GraphReportView {
	t.Helper()
	group, err := NewWAGroupService(f.db).Upsert(f.group.WAAccountID, groupSnap(jid, "Grup "+jid, "62811"))
	require.NoError(t, err)
	in := reportInput(group.ID, label)
	in.SendTime = sendTime
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	// UpdateColumns keeps updated_at from becoming the real clock, which the
	// late-window rule would read as an edit made after the fixture's dates.
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).
		UpdateColumns(map[string]any{"created_at": atWIB(1, 0, 0), "updated_at": atWIB(1, 0, 0)}).Error)
	return report
}

func (f graphReportFixture) tickAt(t *testing.T, day, hour, minute int) int64 {
	t.Helper()
	f.scheduler.now = func() time.Time { return atWIB(day, hour, minute) }
	f.scheduler.Tick(context.Background())
	return f.queuedCount(t)
}

func TestSchedulerSendsEachReportAtItsOwnTime(t *testing.T) {
	f := newGraphReportFixture(t)
	morning := f.createReport(t, "Uplink")
	afternoon := f.createReportAt(t, "8@g.us", "14:30", "Backup")

	assert.Equal(t, int64(1), f.tickAt(t, 11, 6, 0), "only the 06:00 report")
	assert.NotNil(t, f.load(t, morning.ID).LastSentOn)
	assert.Equal(t, int64(1), f.tickAt(t, 11, 14, 29))
	assert.Equal(t, int64(2), f.tickAt(t, 11, 14, 30), "the 14:30 report joins it")
	assert.NotNil(t, f.load(t, afternoon.ID).LastSentOn)
}

func TestMissedReasonNamesTheSendTime(t *testing.T) {
	f := newGraphReportFixture(t)
	afternoon := f.createReportAt(t, "8@g.us", "14:30", "Uplink")
	evening := f.createReportAt(t, "7@g.us", "22:00", "Backup")

	f.tickAt(t, 11, 17, 30)
	assert.Equal(t, "laporan 11 Nov tidak terkirim: tidak ada percobaan kirim antara 14:30 dan 17:30 WIB",
		f.load(t, afternoon.ID).LastError)
	assert.Zero(t, f.tickAt(t, 12, 0, 0), "a window clipped at midnight is marked on the next date")
	assert.Equal(t, "laporan 11 Nov tidak terkirim: tidak ada percobaan kirim antara 22:00 dan 24:00 WIB",
		f.load(t, evening.ID).LastError)
}

func TestChangingTheSendTime(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")
	in := reportInput(f.group.ID, "Uplink")
	reschedule := func(sendTime string) {
		in.SendTime = sendTime
		_, err := f.reports.Update(report.ID, in)
		require.NoError(t, err)
	}

	assert.Equal(t, int64(1), f.tickAt(t, 11, 6, 0))
	reschedule("14:30")
	assert.Equal(t, int64(1), f.tickAt(t, 11, 14, 30), "already sent today")

	assert.Equal(t, int64(1), f.tickAt(t, 12, 14, 0), "not yet at the new time")
	reschedule("13:00")
	assert.Equal(t, int64(2), f.tickAt(t, 12, 14, 1), "13:00's window is still open")

	assert.Equal(t, int64(2), f.tickAt(t, 13, 5, 0))
	reschedule("01:00")
	assert.Equal(t, int64(2), f.tickAt(t, 13, 5, 1), "01:00's window has passed")
	assert.Contains(t, f.load(t, report.ID).LastError, "13 Nov tidak terkirim")
}

func TestGraphReportSendTimeValidation(t *testing.T) {
	f := newGraphReportFixture(t)
	for _, bad := range []string{"6:05", "06:5", "24:00", "12:60", "1200", "ab:cd", "+6:00", "06:00:00"} {
		in := reportInput(f.group.ID, "Uplink")
		in.SendTime = bad
		_, err := f.reports.Create(in, uuid.New())
		require.Error(t, err, bad)
		assert.True(t, errors.Is(err, ErrValidation), bad)
		assert.Contains(t, err.Error(), "jam kirim harus berformat HH:MM", bad)
	}

	report, err := f.reports.Create(reportInput(f.group.ID, "Uplink"), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, DefaultGraphReportSendTime, report.SendTime, "left blank")

	in := reportInput(f.group.ID, "Uplink")
	in.SendTime = " 23:55 "
	updated, err := f.reports.Update(report.ID, in)
	require.NoError(t, err)
	assert.Equal(t, "23:55", updated.SendTime)
	assert.Equal(t, "23:55", f.load(t, report.ID).SendTime)
}

func TestReenablingALateReportDoesNotBlameADayItWasOff(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReportAt(t, "7@g.us", "22:00", "Uplink")
	// Turned back on at 10:00 after being off since 9 Nov.
	require.NoError(t, f.db.Model(&models.GraphReport{}).Where("id = ?", report.ID).
		UpdateColumn("updated_at", atWIB(11, 10, 0)).Error)

	assert.Zero(t, f.tickAt(t, 11, 10, 1))
	assert.Empty(t, f.load(t, report.ID).LastError, "it was off throughout 10 Nov's window")
	assert.Equal(t, int64(1), f.tickAt(t, 11, 22, 0))
}
