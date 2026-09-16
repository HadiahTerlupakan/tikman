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

func TestQueuingAReplyEndsTheWaitAsRepliedByItsSender(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	cs := uuid.New()

	reply, err := messages.Queue(conv.ID, cs, models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitReplied, *waits[0].EndReason)
	assert.Equal(t, &cs, waits[0].EndedBy)
	assert.Equal(t, &reply.ID, waits[0].ReplyMessageID)
}

// Closing a hard thread instead of answering it is the one way out that leaves
// no reply, so the report has to name whoever took it.
func TestClosingAThreadEndsTheWaitAsClosedByWhoeverClosedIt(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	cs := uuid.New()

	require.NoError(t, conversations.Close(conv.ID, cs))

	waits := storedWaits(t, db)
	require.Len(t, waits, 1)
	require.NotNil(t, waits[0].EndReason)
	assert.Equal(t, models.WaitClosed, *waits[0].EndReason)
	assert.Equal(t, &cs, waits[0].EndedBy)
}

func TestAReplyAndACloseStillHappenWhenTheirWaitCannotBeRecorded(t *testing.T) {
	db, messages, conversations, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable(&models.CSWait{}))

	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	require.NoError(t, conversations.Close(conv.ID, uuid.New()))

	got, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationClosed, got.Status)
}

func TestClosingAThreadThatIsNotThereStillSaysSo(t *testing.T) {
	_, _, conversations, _ := recordingSetup(t)

	err := conversations.Close(uuid.New(), uuid.New())

	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// The figures must survive the deletions the inbox allows, or a slow answer
// could be erased by deleting what it was measured from — and removing a
// number must not be refused by rows that outlive it, the way the broadcast
// history once refused it.
func TestDeletingMessagesAThreadOrANumberLeavesTheWaitsAlone(t *testing.T) {
	db, messages, _, conv := recordingSetup(t)
	_, _, err := messages.SaveInbound(customerSent(conv.ID, "3EB0A", time.Now()))
	require.NoError(t, err)
	_, err = messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	purge := NewCSPurgeService(db, t.TempDir())

	_, err = purge.Conversation(conv.ID)
	require.NoError(t, err)
	require.NoError(t, purge.DeleteAccount(conv.WAAccountID))

	assert.Len(t, storedWaits(t, db), 1, "the wait outlives the messages and the number")
	var threads int64
	require.NoError(t, db.Model(&models.CSConversation{}).Count(&threads).Error)
	assert.Zero(t, threads, "the number and its threads really are gone")
}
