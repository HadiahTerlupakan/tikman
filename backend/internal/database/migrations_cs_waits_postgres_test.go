package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// openThreadWait is a wait still open, on a thread of its own: these tests
// share one schema (see freshPostgres), so no two may use one conversation id.
func openThreadWait(conversationID uuid.UUID) *models.CSWait {
	now := time.Now()
	return &models.CSWait{
		ConversationID: conversationID, WAAccountID: uuid.New(),
		StartedAt: now, CustomerSentAt: now, LastCustomerSentAt: now,
	}
}

// endedAs marks a wait ended for a reason, by a CS, with a message.
func endedAs(w *models.CSWait, reason models.WaitEndReason, by, message *uuid.UUID) *models.CSWait {
	at := time.Now()
	w.EndedAt, w.EndReason, w.EndedBy, w.ReplyMessageID = &at, &reason, by, message
	return w
}

// A thread waits on one thing at a time. The recording never opens a second
// wait, and the database refuses one regardless: a second open wait would be
// answered twice and counted twice.
func TestDatabaseRefusesASecondOpenWaitOnOneThread(t *testing.T) {
	db := freshPostgres(t)
	thread := uuid.New()
	require.NoError(t, db.Create(openThreadWait(thread)).Error)

	err := db.Create(openThreadWait(thread)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_cs_waits_one_open_per_thread")
}

func TestDatabaseAllowsANewWaitOnceTheLastOneEnded(t *testing.T) {
	db := freshPostgres(t)
	thread := uuid.New()
	require.NoError(t, db.Create(endedAs(openThreadWait(thread), models.WaitAbandoned, nil, nil)).Error)

	assert.NoError(t, db.Create(openThreadWait(thread)).Error)
}

func TestDatabaseRefusesAReplyThatNamesNoCS(t *testing.T) {
	db := freshPostgres(t)
	message := uuid.New()

	err := db.Create(endedAs(openThreadWait(uuid.New()), models.WaitReplied, nil, &message)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAPhoneAnswerThatNamesACS(t *testing.T) {
	db := freshPostgres(t)
	someone, message := uuid.New(), uuid.New()

	err := db.Create(endedAs(openThreadWait(uuid.New()), models.WaitPhone, &someone, &message)).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAnOpenWaitThatNamesACS(t *testing.T) {
	db := freshPostgres(t)
	wait := openThreadWait(uuid.New())
	someone := uuid.New()
	wait.EndedBy = &someone

	err := db.Create(wait).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_actor_matches_reason")
}

func TestDatabaseRefusesAReasonWithoutAnEndTime(t *testing.T) {
	db := freshPostgres(t)
	wait := openThreadWait(uuid.New())
	reason := models.WaitAbandoned
	wait.EndReason = &reason

	err := db.Create(wait).Error

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cs_waits_ended_together")
}
