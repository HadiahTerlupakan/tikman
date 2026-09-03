package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

type purgeFixture struct {
	db            *gorm.DB
	root          string
	conversations *CSConversationService
	messages      *CSMessageService
	purge         *CSPurgeService
}

func newPurgeFixture(t *testing.T) *purgeFixture {
	t.Helper()
	db := setupTestDB(t)
	root := t.TempDir()
	conversations := NewCSConversationService(db)
	return &purgeFixture{
		db:            db,
		root:          root,
		conversations: conversations,
		messages:      NewCSMessageService(db, conversations),
		purge:         NewCSPurgeService(db, root),
	}
}

func (f *purgeFixture) thread(t *testing.T, account models.WAAccount, phone string) *models.CSConversation {
	t.Helper()
	conv, err := f.conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: phone + "@s.whatsapp.net", Phone: phone, Name: "Budi",
	})
	require.NoError(t, err)
	return conv
}

// file writes an attachment under the media root and answers its relative path.
func (f *purgeFixture) file(t *testing.T, name string) string {
	t.Helper()
	rel := filepath.Join("2026", "09", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(f.root, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(f.root, rel), []byte("x"), 0o644))
	return rel
}

func (f *purgeFixture) inbound(t *testing.T, conv uuid.UUID, waID string, media *MediaFile) *models.CSMessage {
	t.Helper()
	kind := models.MessageKindText
	if media != nil {
		kind = models.MessageKindImage
	}
	msg, _, err := f.messages.SaveInbound(InboundMessage{
		ConversationID: conv, WAMessageID: waID, Kind: kind,
		Body: "halo", Media: media, At: time.Now(),
	})
	require.NoError(t, err)
	return msg
}

func (f *purgeFixture) exists(t *testing.T, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(f.root, rel))
	return err == nil
}

func (f *purgeFixture) messageCount(t *testing.T, conv uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Model(&models.CSMessage{}).Where("conversation_id = ?", conv).Count(&n).Error)
	return n
}

// The attachment is the whole point: deleting only the row leaves the file on
// the VPS disk with nothing left pointing at it, so nothing can ever clean it.
func TestPurgeMessageTakesTheFileOffTheDiskToo(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")

	rel := f.file(t, "photo.jpg")
	msg := f.inbound(t, conv.ID, "3EB0A", &MediaFile{
		Path: rel, Mime: "image/jpeg", Filename: "photo.jpg", Size: 1,
	})

	removed, err := f.purge.Message(msg.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	assert.False(t, f.exists(t, rel), "the attachment is off the disk")
	assert.Equal(t, int64(0), f.messageCount(t, conv.ID))
}

// A file somebody already removed by hand must not block the purge: the row is
// pointing at nothing, and refusing would leave it there forever.
func TestPurgeMessageSucceedsWhenTheFileIsAlreadyGone(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")

	rel := f.file(t, "photo.jpg")
	msg := f.inbound(t, conv.ID, "3EB0A", &MediaFile{
		Path: rel, Mime: "image/jpeg", Filename: "photo.jpg", Size: 1,
	})
	require.NoError(t, os.Remove(filepath.Join(f.root, rel)))

	removed, err := f.purge.Message(msg.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
}

func TestPurgeConversationEmptiesTheThreadButKeepsIt(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")
	f.inbound(t, conv.ID, "3EB0A", nil)
	f.inbound(t, conv.ID, "3EB0B", nil)

	removed, err := f.purge.Conversation(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, removed)

	assert.Equal(t, int64(0), f.messageCount(t, conv.ID))
	_, err = f.conversations.Get(conv.ID)
	assert.NoError(t, err, "the thread itself stays in the inbox")
}

// Without this the badge goes on claiming unread messages that no longer
// exist, and a CS opening an empty thread still sees "2 belum dibaca".
func TestPurgeLeavesNoBadgeOnAnEmptiedThread(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")
	f.inbound(t, conv.ID, "3EB0A", nil)
	f.inbound(t, conv.ID, "3EB0B", nil)

	before, err := f.conversations.Get(conv.ID)
	require.NoError(t, err)
	require.Equal(t, 2, before.UnreadCount, "the fixture really did leave a badge to clear")

	_, err = f.purge.Conversation(conv.ID)
	require.NoError(t, err)

	after, err := f.conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, after.UnreadCount)
}

// The badge must never outnumber what is left to read.
func TestPurgeClampsTheBadgeToTheMessagesStillThere(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")
	first := f.inbound(t, conv.ID, "3EB0A", nil)
	f.inbound(t, conv.ID, "3EB0B", nil)
	f.inbound(t, conv.ID, "3EB0C", nil)

	_, err := f.purge.Message(first.ID)
	require.NoError(t, err)

	after, err := f.conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, after.UnreadCount, "three unread, one purged, two left to read")
}

// "Belum dipegang" reads last_message_direction. An emptied thread has nothing
// to reply to, so it must drop out of that tab rather than sit there forever.
func TestPurgeTakesAnEmptiedThreadOutOfTheAwaitingReplyTab(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")
	f.inbound(t, conv.ID, "3EB0A", nil)

	waiting, err := f.conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1, "the fixture really did leave it waiting")

	_, err = f.purge.Conversation(conv.ID)
	require.NoError(t, err)

	waiting, err = f.conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}

// A reply must outlive the message it answered — losing the grey quote block is
// a far smaller loss than taking a CS's own words out of the thread.
func TestPurgeKeepsAReplyWhoseQuotedMessageWasDeleted(t *testing.T) {
	f := newPurgeFixture(t)
	conv := f.thread(t, csAccount(t, f.db), "628111222333")
	quoted := f.inbound(t, conv.ID, "3EB0A", nil)

	sender := uuid.New()
	reply, err := f.messages.Queue(conv.ID, sender, models.MessageKindText, "sudah kami cek", nil, &quoted.ID)
	require.NoError(t, err)

	_, err = f.purge.Message(quoted.ID)
	require.NoError(t, err)

	history, err := f.messages.History(conv.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, reply.ID, history[0].ID, "the reply is still in the thread")
	assert.Nil(t, history[0].ReplyTo, "with no quote block to draw")
}

func TestPurgeAccountEmptiesOnlyThatNumbersThreads(t *testing.T) {
	f := newPurgeFixture(t)
	mine := csAccount(t, f.db)
	other := models.WAAccount{Label: "CS Teknis", Status: models.WAAccountConnected}
	require.NoError(t, f.db.Create(&other).Error)

	ours := f.thread(t, mine, "628111222333")
	theirs := f.thread(t, other, "628111222444")
	f.inbound(t, ours.ID, "3EB0A", nil)
	f.inbound(t, theirs.ID, "3EB0B", nil)

	removed, err := f.purge.Account(mine.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	assert.Equal(t, int64(0), f.messageCount(t, ours.ID))
	assert.Equal(t, int64(1), f.messageCount(t, theirs.ID), "the other number is untouched")
	_, err = f.conversations.Get(ours.ID)
	assert.NoError(t, err, "clearing a number keeps its threads")
}

func TestPurgeInboxEmptiesEveryNumber(t *testing.T) {
	f := newPurgeFixture(t)
	mine := csAccount(t, f.db)
	other := models.WAAccount{Label: "CS Teknis", Status: models.WAAccountConnected}
	require.NoError(t, f.db.Create(&other).Error)

	ours := f.thread(t, mine, "628111222333")
	theirs := f.thread(t, other, "628111222444")
	f.inbound(t, ours.ID, "3EB0A", nil)
	f.inbound(t, theirs.ID, "3EB0B", nil)

	removed, err := f.purge.Inbox()
	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	assert.Equal(t, int64(0), f.messageCount(t, ours.ID))
	assert.Equal(t, int64(0), f.messageCount(t, theirs.ID))
}

// SQLite has none of migration 41's foreign keys, so this asserts the rows and
// files actually go, not the ordering that gets them there. The ordering is
// under test in TestDeleteAccountClearsItsThreadsFirstOnPostgres, where the
// RESTRICT constraint that makes it matter actually exists.
func TestDeleteAccountRemovesItsThreadsMessagesAndFiles(t *testing.T) {
	f := newPurgeFixture(t)
	account := csAccount(t, f.db)
	conv := f.thread(t, account, "628111222333")

	rel := f.file(t, "photo.jpg")
	f.inbound(t, conv.ID, "3EB0A", &MediaFile{
		Path: rel, Mime: "image/jpeg", Filename: "photo.jpg", Size: 1,
	})

	require.NoError(t, f.purge.DeleteAccount(account.ID))

	assert.False(t, f.exists(t, rel), "the attachment is off the disk")
	assert.Equal(t, int64(0), f.messageCount(t, conv.ID))

	_, err := f.conversations.Get(conv.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "the thread goes with the number")

	_, err = NewCSAccountService(f.db).Get(account.ID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// The customer's profile photo is stored under the media root like any
// attachment, and nothing else would ever come back for it.
func TestDeleteAccountRemovesTheCustomerAvatarsToo(t *testing.T) {
	f := newPurgeFixture(t)
	account := csAccount(t, f.db)
	conv := f.thread(t, account, "628111222333")

	rel := f.file(t, "avatar.jpg")
	_, err := f.conversations.SetAvatar(conv.ID, "PIC1", rel)
	require.NoError(t, err)

	require.NoError(t, f.purge.DeleteAccount(account.ID))
	assert.False(t, f.exists(t, rel))
}

func TestDeleteAccountLeavesTheOtherNumbersAlone(t *testing.T) {
	f := newPurgeFixture(t)
	doomed := csAccount(t, f.db)
	kept := models.WAAccount{Label: "CS Teknis", Status: models.WAAccountConnected}
	require.NoError(t, f.db.Create(&kept).Error)

	keptThread := f.thread(t, kept, "628111222444")
	f.inbound(t, keptThread.ID, "3EB0B", nil)

	require.NoError(t, f.purge.DeleteAccount(doomed.ID))

	_, err := NewCSAccountService(f.db).Get(kept.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), f.messageCount(t, keptThread.ID))
}

func TestDeleteAccountRefusesANumberThatIsNotThere(t *testing.T) {
	f := newPurgeFixture(t)
	assert.ErrorIs(t, f.purge.DeleteAccount(uuid.New()), gorm.ErrRecordNotFound)
}
