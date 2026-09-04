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

const infoGangguan = "120363000000000001@newsletter"

func postSetup(t *testing.T) (*CSChannelPostService, models.WAAccount, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	return NewCSChannelPostService(db), csAccount(t, db), db
}

func queued(t *testing.T, s *CSChannelPostService, account models.WAAccount, body string) *models.WAChannelPost {
	t.Helper()
	post, err := s.Queue(ChannelPost{
		WAAccountID:  account.ID,
		ChannelJID:   infoGangguan,
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindText,
		Body:         body,
	})
	require.NoError(t, err)
	return post
}

// A queued row is the outbox. Nothing has reached WhatsApp yet, and the row
// must say so rather than read as delivered.
func TestQueueStoresAnUpdateAsWaiting(t *testing.T) {
	posts, account, _ := postSetup(t)

	post := queued(t, posts, account, "Ada pemeliharaan malam ini")

	assert.Equal(t, models.ChannelPostQueued, post.Status)
	assert.Nil(t, post.WAMessageID)
	assert.Nil(t, post.SentAt)
}

// The drainer sends oldest first, so two announcements written in order do not
// reach subscribers reversed.
func TestClaimQueuedAnswersOldestFirst(t *testing.T) {
	posts, account, db := postSetup(t)

	first := queued(t, posts, account, "pertama")
	second := queued(t, posts, account, "kedua")
	// created_at is written by the database on insert; SQLite resolves both to
	// the same instant often enough that ordering has to be made deterministic.
	require.NoError(t, db.Model(&models.WAChannelPost{}).Where("id = ?", second.ID).
		Update("created_at", second.CreatedAt.Add(time.Second)).Error)

	waiting, err := posts.ClaimQueued(account.ID, 10)
	require.NoError(t, err)
	require.Len(t, waiting, 2)
	assert.Equal(t, first.ID, waiting[0].ID)
}

// A claim is scoped to the number, the same way the message outbox is: the
// session holding one number must never send another number's update.
func TestClaimQueuedIsScopedToItsNumber(t *testing.T) {
	posts, account, db := postSetup(t)
	other := models.WAAccount{Label: "CS Kedua", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&other).Error)

	queued(t, posts, account, "punya nomor pertama")

	waiting, err := posts.ClaimQueued(other.ID, 10)
	require.NoError(t, err)
	assert.Empty(t, waiting)
}

// A refusal must leave a reason on the row. Without it the sender watches an
// announcement disappear with nothing to act on.
func TestMarkFailedRecordsTheReason(t *testing.T) {
	posts, account, _ := postSetup(t)
	post := queued(t, posts, account, "Ada pemeliharaan malam ini")

	require.NoError(t, posts.MarkFailed(post.ID, "not authorized to post"))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostFailed, history[0].Status)
	assert.Equal(t, "not authorized to post", history[0].FailReason)
}

// A retry that succeeds must not leave the previous refusal on screen beside
// a "sent" badge.
func TestMarkSentClearsAnEarlierFailure(t *testing.T) {
	posts, account, _ := postSetup(t)
	post := queued(t, posts, account, "Ada pemeliharaan malam ini")
	require.NoError(t, posts.MarkFailed(post.ID, "not authorized to post"))

	require.NoError(t, posts.MarkSent(post.ID, "3EB0F1"))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, models.ChannelPostSent, history[0].Status)
	assert.Empty(t, history[0].FailReason)
	require.NotNil(t, history[0].WAMessageID)
	assert.Equal(t, "3EB0F1", *history[0].WAMessageID)
	assert.NotNil(t, history[0].SentAt)
}

// History is keyed by the JID, not by the wa_channels row, so a sync that
// rebuilds the mirror does not take the record of what was announced with it.
func TestHistorySurvivesTheChannelRowBeingRebuilt(t *testing.T) {
	posts, account, db := postSetup(t)
	channels := NewCSChannelService(db)
	require.NoError(t, channels.Replace(account.ID, []models.WAChannel{
		{JID: infoGangguan, Name: "Info Gangguan", Role: models.ChannelRoleOwner},
	}))
	queued(t, posts, account, "Ada pemeliharaan malam ini")

	require.NoError(t, channels.Replace(account.ID, nil))

	history, err := posts.ListFor(infoGangguan, 10)
	require.NoError(t, err)
	assert.Len(t, history, 1)
}
