package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// waitFixture is one thread whose messages are stored at chosen moments. Every
// time is UTC: SQLite compares stored times as text, so they must share a zone.
type waitFixture struct {
	t    *testing.T
	db   *gorm.DB
	conv *models.CSConversation
}

func newWaitFixture(t *testing.T) *waitFixture {
	t.Helper()
	db := setupTestDB(t)
	conv, err := NewCSConversationService(db).FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)
	return &waitFixture{t: t, db: db, conv: conv}
}

// waitClock is a moment a number of minutes after 01:00 UTC on 10 September.
func waitClock(minutes int) time.Time {
	return time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC).Add(time.Duration(minutes) * time.Minute)
}

// customer stores a customer's message sent at sentMin that reached TikMan at
// arrivedMin.
func (f *waitFixture) customer(sentMin, arrivedMin int) *models.CSMessage {
	return f.store(models.MessageIn, nil, sentMin, arrivedMin)
}

// reply stores an outbound message: queued by sender through TikMan, or typed
// on the phone when sender is nil.
func (f *waitFixture) reply(sender *uuid.UUID, sentMin, arrivedMin int) *models.CSMessage {
	return f.store(models.MessageOut, sender, sentMin, arrivedMin)
}

func (f *waitFixture) store(dir models.MessageDirection, sender *uuid.UUID, sentMin, arrivedMin int) *models.CSMessage {
	f.t.Helper()
	msg := &models.CSMessage{
		ConversationID: f.conv.ID, Direction: dir, SenderUserID: sender,
		Kind: models.MessageKindText, Body: "pesan", Status: models.MessageDelivered,
		WATimestamp: waitClock(sentMin), CreatedAt: waitClock(arrivedMin),
	}
	require.NoError(f.t, f.db.Create(msg).Error)
	return msg
}

// step runs one bookkeeping function in a transaction of its own.
func (f *waitFixture) step(fn func(*gorm.DB) error) {
	f.t.Helper()
	require.NoError(f.t, f.db.Transaction(fn))
}

func (f *waitFixture) wrote(msg *models.CSMessage) {
	f.t.Helper()
	f.step(func(tx *gorm.DB) error { return customerWrote(tx, msg) })
}

func (f *waitFixture) waits() []models.CSWait {
	f.t.Helper()
	var waits []models.CSWait
	require.NoError(f.t, f.db.Order("started_at ASC").Find(&waits).Error)
	return waits
}

func TestAFirstCustomerMessageStartsAWait(t *testing.T) {
	f := newWaitFixture(t)

	f.wrote(f.customer(0, 1))

	waits := f.waits()
	require.Len(t, waits, 1)
	assert.WithinDuration(t, waitClock(1), waits[0].StartedAt, 0, "a wait starts when TikMan held the message")
	assert.WithinDuration(t, waitClock(0), waits[0].CustomerSentAt, 0)
	assert.Equal(t, f.conv.WAAccountID, waits[0].WAAccountID)
	assert.Nil(t, waits[0].EndedAt)
}

func TestMessagesBeforeAnAnswerStayInOneWait(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))

	f.wrote(f.customer(10, 10))

	waits := f.waits()
	require.Len(t, waits, 1)
	assert.WithinDuration(t, waitClock(0), waits[0].StartedAt, 0)
	assert.WithinDuration(t, waitClock(10), waits[0].LastCustomerSentAt, 0)
}

// "ok makasih" and a new question days later are two waits, not one long one.
func TestWritingAgainAfterADayOfSilenceAbandonsTheOldWait(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	later := 24*60 + 1

	f.wrote(f.customer(later, later))

	waits := f.waits()
	require.Len(t, waits, 2)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitAbandoned, *waits[0].EndReason)
	assert.WithinDuration(t, waitClock(later), *waits[0].EndedAt, 0)
	assert.Nil(t, waits[1].EndedAt)
	assert.WithinDuration(t, waitClock(later), waits[1].StartedAt, 0)
}

func TestAnOlderMessageArrivingLateDoesNotMoveTheWaitBack(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(10, 10))

	f.wrote(f.customer(5, 12))

	assert.WithinDuration(t, waitClock(10), f.waits()[0].LastCustomerSentAt, 0)
}

func TestAReplyEndsTheWaitAndNamesTheCS(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	cs := uuid.New()
	answer := f.reply(&cs, 7, 7)

	f.step(func(tx *gorm.DB) error { return replied(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitReplied, *w.EndReason)
	assert.WithinDuration(t, waitClock(7), *w.EndedAt, 0)
	assert.Equal(t, &cs, w.EndedBy)
	assert.Equal(t, &answer.ID, w.ReplyMessageID)
}

func TestAReplyWithNoOneWaitingChangesNothing(t *testing.T) {
	f := newWaitFixture(t)
	cs := uuid.New()
	answer := f.reply(&cs, 7, 7)

	f.step(func(tx *gorm.DB) error { return replied(tx, answer) })

	assert.Empty(t, f.waits())
}

func TestAPhoneReplyEndsTheWaitWithNobodyNamed(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	answer := f.reply(nil, 5, 5)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitPhone, *w.EndReason)
	assert.WithinDuration(t, waitClock(5), *w.EndedAt, 0)
	assert.Nil(t, w.EndedBy)
	assert.Equal(t, &answer.ID, w.ReplyMessageID)
}

// A phone reply sent before this wait began answered an earlier one.
func TestAPhoneReplyOlderThanTheWaitChangesNothing(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(10, 10))
	answer := f.reply(nil, 5, 11)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	assert.Nil(t, f.waits()[0].EndedAt)
}

// The phone answered at minute 10, but WhatsApp handed that over only at
// minute 30, after the customer had written again at minute 20. The message
// at 20 is still unanswered.
func TestALatePhoneReplyLeavesTheCustomersLaterMessagesWaiting(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	f.wrote(f.customer(20, 20))
	answer := f.reply(nil, 10, 30)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	waits := f.waits()
	require.Len(t, waits, 2)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitPhone, *waits[0].EndReason)
	assert.WithinDuration(t, waitClock(10), *waits[0].EndedAt, 0)
	assert.Nil(t, waits[1].EndedAt)
	assert.WithinDuration(t, waitClock(20), waits[1].StartedAt, 0)
}

// The customer's message reached TikMan at minute 40; the phone had answered
// at minute 5. Nobody in TikMan waited, so the wait ends the moment it starts.
func TestAPhoneReplySentBeforeTheMessageArrivedEndsTheWaitAtOnce(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 40))
	answer := f.reply(nil, 5, 41)

	f.step(func(tx *gorm.DB) error { return answeredFromPhone(tx, answer) })

	w := f.waits()[0]
	require.NotNil(t, w.EndedAt)
	assert.WithinDuration(t, w.StartedAt, *w.EndedAt, 0)
}

func TestClosingEndsTheWaitAsClosedByTheCS(t *testing.T) {
	f := newWaitFixture(t)
	f.wrote(f.customer(0, 0))
	cs := uuid.New()

	f.step(func(tx *gorm.DB) error { return closedWithoutReply(tx, f.conv.ID, cs, waitClock(15)) })

	w := f.waits()[0]
	require.NotNil(t, w.EndReason)
	assert.Equal(t, models.WaitClosed, *w.EndReason)
	assert.Equal(t, &cs, w.EndedBy)
	assert.Nil(t, w.ReplyMessageID)
}

// openThenFail writes a wait and then fails, the way a step can fail partway.
func openThenFail(conversationID uuid.UUID) func(*gorm.DB) error {
	return func(tx *gorm.DB) error {
		if err := openWait(tx, conversationID, waitClock(0), waitClock(0), waitClock(0)); err != nil {
			return err
		}
		return errors.New("the step fails after writing")
	}
}

// A step that fails is undone on its own; what the transaction wrote before it
// still commits.
func TestAFailedStepRollsBackAloneAndTheTransactionCommits(t *testing.T) {
	f := newWaitFixture(t)
	waits := csWaits{logger: zap.NewNop()}

	f.step(func(tx *gorm.DB) error {
		msg := &models.CSMessage{
			ConversationID: f.conv.ID, Direction: models.MessageIn, Kind: models.MessageKindText,
			Body: "tetap tersimpan", Status: models.MessageDelivered, WATimestamp: waitClock(0),
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		waits.record(tx, f.conv.ID, "test", openThenFail(f.conv.ID))
		return nil
	})

	assert.Empty(t, f.waits(), "the failed step's wait is rolled back")
	var stored int64
	require.NoError(t, f.db.Model(&models.CSMessage{}).Count(&stored).Error)
	assert.Equal(t, int64(1), stored, "the message written before it commits")
}
