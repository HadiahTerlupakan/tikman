package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// reportFixture plants waits the way the recording would have left them. Every
// time is UTC: SQLite compares stored times as text, so they must share a zone.
type reportFixture struct {
	t    *testing.T
	db   *gorm.DB
	svc  *CSPerformanceService
	conv *models.CSConversation
	ani  uuid.UUID
	budi uuid.UUID
}

func newReportFixture(t *testing.T) *reportFixture {
	t.Helper()
	db := setupTestDB(t)
	conv, err := NewCSConversationService(db).FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return &reportFixture{
		t: t, db: db, svc: NewCSPerformanceService(db), conv: conv,
		ani: reportUser(t, db, "ani"), budi: reportUser(t, db, "budi"),
	}
}

func reportUser(t *testing.T, db *gorm.DB, username string) uuid.UUID {
	t.Helper()
	user := models.User{
		ID: uuid.New(), Username: username, Email: username + "@example.test", Role: models.UserRoleCS,
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

// sept is a moment in September 2026, in UTC. WIB is seven hours ahead, so
// sept(10, 1, 0) is 08:00 WIB on the 10th.
func sept(day, hour, minute int) time.Time {
	return time.Date(2026, 9, day, hour, minute, 0, 0, time.UTC)
}

// planted describes one wait to plant. An empty thread means the fixture's own.
type planted struct {
	started, sent, ended time.Time
	reason               models.WaitEndReason
	by                   *uuid.UUID
	thread               uuid.UUID
}

func (f *reportFixture) plant(p planted) {
	f.t.Helper()
	if p.thread == uuid.Nil {
		p.thread = f.conv.ID
	}
	wait := models.CSWait{
		ConversationID: p.thread, WAAccountID: f.conv.WAAccountID,
		StartedAt: p.started, CustomerSentAt: p.sent, LastCustomerSentAt: p.sent,
		EndedAt: &p.ended, EndReason: &p.reason, EndedBy: p.by,
	}
	if p.reason == models.WaitReplied || p.reason == models.WaitPhone {
		message := uuid.New()
		wait.ReplyMessageID = &message
	}
	require.NoError(f.t, f.db.Create(&wait).Error)
}

func (f *reportFixture) plantOpen(thread uuid.UUID, started time.Time) {
	f.t.Helper()
	require.NoError(f.t, f.db.Create(&models.CSWait{
		ConversationID: thread, WAAccountID: f.conv.WAAccountID,
		StartedAt: started, CustomerSentAt: started, LastCustomerSentAt: started,
	}).Error)
}

func reportRangeFor(t *testing.T, from, to string) ReportRange {
	t.Helper()
	r, err := ReportRangeFromDates(from, to)
	require.NoError(t, err)
	return r
}

func TestTheTeamFiguresCountOnlyAnswersThatWereNotTheSystemsFault(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 3), reason: models.WaitPhone})
	f.plant(planted{started: sept(10, 3, 0), sent: sept(10, 2, 0), ended: sept(10, 3, 1), reason: models.WaitReplied, by: &f.budi})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})
	f.plant(planted{started: sept(10, 5, 0), sent: sept(10, 5, 0), ended: sept(10, 6, 0), reason: models.WaitAbandoned})
	// 01:00 WIB on the 11th, outside the report for the 10th.
	f.plant(planted{started: sept(10, 17, 55), sent: sept(10, 17, 55), ended: sept(10, 18, 0), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	assert.Equal(t, csReplyTargetMinutes, summary.TargetMinutes)
	assert.Equal(t, 2, summary.Team.Replies.Count, "one reply and one phone answer")
	require.NotNil(t, summary.Team.Replies.MedianMinutes)
	assert.InDelta(t, 4.0, *summary.Team.Replies.MedianMinutes, 1e-9)
	assert.Equal(t, 1, summary.Team.SystemDelayed)
	assert.Equal(t, 1, summary.Team.ClosedWithoutReply)
	assert.Equal(t, 1, summary.Team.Abandoned)
}

// Ani starts work at 08:00 WIB with a customer who wrote at 01:00 WIB. That
// first answer costs her nothing; the customer who wrote at 07:30 WIB is
// counted from 08:00, when she started.
func TestACSIsChargedFromWhenTheirStretchOfWorkBegan(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(9, 18, 0), sent: sept(9, 18, 0), ended: sept(10, 1, 0), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 0, 30), sent: sept(10, 0, 30), ended: sept(10, 1, 10), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, "ani", summary.Agents[0].Username)
	assert.Equal(t, 2, summary.Agents[0].Replies.Count)
	require.NotNil(t, summary.Agents[0].Replies.MedianMinutes)
	assert.InDelta(t, 5.0, *summary.Agents[0].Replies.MedianMinutes, 1e-9, "0 and 10 minutes")
	require.NotNil(t, summary.Team.Replies.MedianMinutes)
	assert.InDelta(t, 230.0, *summary.Team.Replies.MedianMinutes, 1e-9, "the team still waited 420 and 40 minutes")
}

// Ani's stretch began at 23:30 WIB on the 9th, before the report for the 10th
// starts. Her answer at 00:10 WIB still counts from when the customer wrote.
func TestAStretchOfWorkThatBeganBeforeTheReportIsStillRecognised(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(9, 16, 20), sent: sept(9, 16, 20), ended: sept(9, 16, 30), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(9, 16, 40), sent: sept(9, 16, 40), ended: sept(9, 17, 10), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	require.NotNil(t, summary.Agents[0].Replies.MedianMinutes)
	assert.InDelta(t, 30.0, *summary.Agents[0].Replies.MedianMinutes, 1e-9,
		"without the lookback her stretch would start at this very reply, and the answer would cost nothing")
}

func TestACSSeesTheWholeTeamButOnlyTheirOwnRow(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 5), reason: models.WaitReplied, by: &f.budi})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), &f.ani, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, f.ani, summary.Agents[0].UserID)
	assert.Equal(t, 2, summary.Team.Replies.Count)
}

// Closing hard threads instead of answering them must show up somewhere.
func TestACSWhoOnlyClosedThreadsStillHasARow(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Agents, 1)
	assert.Equal(t, "budi", summary.Agents[0].Username)
	assert.Equal(t, 1, summary.Agents[0].ClosedWithoutReply)
	assert.Equal(t, 0, summary.Agents[0].Replies.Count)
	assert.Nil(t, summary.Agents[0].Replies.MedianMinutes)
}

func TestEveryDayOfTheReportHasARowEvenWithNothingInIt(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-09", "2026-09-11"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	require.Len(t, summary.Days, 3)
	assert.Equal(t, "2026-09-09", summary.Days[0].Date)
	assert.Equal(t, 0, summary.Days[0].Replies.Count)
	assert.Nil(t, summary.Days[0].Replies.WithinTargetPct)
	assert.Equal(t, "2026-09-10", summary.Days[1].Date)
	assert.Equal(t, 1, summary.Days[1].Replies.Count)
}

// A wait on a thread that was deleted can never be answered, so it is not
// somebody still waiting.
func TestWaitingNowCountsOpenWaitsOnThreadsThatStillExist(t *testing.T) {
	f := newReportFixture(t)
	f.plantOpen(f.conv.ID, sept(10, 11, 40))
	f.plantOpen(uuid.New(), sept(10, 10, 0))

	summary, err := f.svc.Summary(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, sept(10, 12, 0))
	require.NoError(t, err)

	assert.Equal(t, 1, summary.Waiting.Count)
	require.NotNil(t, summary.Waiting.LongestMinutes)
	assert.InDelta(t, 20.0, *summary.Waiting.LongestMinutes, 1e-9)
}

func TestTheWaitListShowsNewestFirstWithItsThreadAndMinutes(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 2, 0), sent: sept(10, 2, 0), ended: sept(10, 2, 3), reason: models.WaitPhone})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 3, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi, thread: uuid.New()})

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, 10, 0)
	require.NoError(t, err)

	assert.EqualValues(t, 3, list.Total)
	require.Len(t, list.Items, 3)
	closed, phone, replied := list.Items[0], list.Items[1], list.Items[2]

	assert.True(t, closed.ConversationDeleted)
	assert.Equal(t, "budi", closed.EndedByUsername)
	assert.Nil(t, closed.TeamMinutes, "a thread closed without a reply has no answer to time")
	assert.True(t, closed.SystemDelayed)

	assert.False(t, phone.ConversationDeleted)
	assert.Equal(t, f.conv.CustomerName, phone.CustomerName)
	require.NotNil(t, phone.TeamMinutes)
	assert.InDelta(t, 3.0, *phone.TeamMinutes, 1e-9)
	assert.Nil(t, phone.CountedMinutes, "nobody in TikMan is charged for an answer from the phone")

	require.NotNil(t, replied.TeamMinutes)
	assert.InDelta(t, 5.0, *replied.TeamMinutes, 1e-9)
	require.NotNil(t, replied.CountedMinutes)
	assert.InDelta(t, 0.0, *replied.CountedMinutes, 1e-9, "her first answer of the stretch")
}

func TestTheWaitListForOneCSHoldsOnlyWhatTheyEnded(t *testing.T) {
	f := newReportFixture(t)
	f.plant(planted{started: sept(10, 1, 0), sent: sept(10, 1, 0), ended: sept(10, 1, 5), reason: models.WaitReplied, by: &f.ani})
	f.plant(planted{started: sept(10, 4, 0), sent: sept(10, 4, 0), ended: sept(10, 4, 30), reason: models.WaitClosed, by: &f.budi})

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), &f.budi, 10, 0)
	require.NoError(t, err)

	assert.EqualValues(t, 1, list.Total)
	require.Len(t, list.Items, 1)
	assert.Equal(t, models.WaitClosed, list.Items[0].EndReason)
}

func TestTheWaitListPages(t *testing.T) {
	f := newReportFixture(t)
	for minute := 0; minute < 3; minute++ {
		f.plant(planted{
			started: sept(10, 1, minute), sent: sept(10, 1, minute), ended: sept(10, 2, minute),
			reason: models.WaitPhone,
		})
	}

	list, err := f.svc.Waits(reportRangeFor(t, "2026-09-10", "2026-09-10"), nil, 2, 2)
	require.NoError(t, err)

	assert.EqualValues(t, 3, list.Total)
	assert.Len(t, list.Items, 1)
}
