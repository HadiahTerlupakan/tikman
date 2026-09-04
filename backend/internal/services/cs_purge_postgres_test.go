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

// purgePostgres builds the CS tables with their real constraints and answers a
// number with one thread on it. SQLite gets none of these constraints —
// AutoMigrate writes the columns, migration 41 writes the foreign keys — so
// this is the only place the ordering below is actually under test.
func purgePostgres(t *testing.T) (*gorm.DB, models.WAAccount, *models.CSConversation) {
	t.Helper()
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("set TEST_POSTGRES_DSN to exercise the CS foreign keys")
	}

	db := setupPostgresTestDB(t)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)

	conv, err := NewCSConversationService(db).FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)
	return db, account, conv
}

// The control for the test below: cs_conversations references wa_accounts ON
// DELETE RESTRICT, so removing a number while a thread still points at it is
// refused by Postgres. If this ever stops failing, the constraint has been
// loosened and DeleteAccount's ordering has stopped being load-bearing.
func TestDeletingAWAAccountWithThreadsIsRefusedOnPostgres(t *testing.T) {
	db, account, _ := purgePostgres(t)

	err := db.Delete(&models.WAAccount{}, "id = ?", account.ID).Error
	require.Error(t, err, "the RESTRICT foreign key is what makes the ordering matter")
	assert.Contains(t, err.Error(), "fk_cs_conversations_account")
}

func TestDeleteAccountClearsItsThreadsFirstOnPostgres(t *testing.T) {
	db, account, conv := purgePostgres(t)

	messages := NewCSMessageService(db, NewCSConversationService(db))
	_, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0A",
		Kind: models.MessageKindText, Body: "halo", At: time.Now(),
	})
	require.NoError(t, err)

	require.NoError(t, NewCSPurgeService(db, t.TempDir()).DeleteAccount(account.ID))

	var accounts, conversations, stored int64
	require.NoError(t, db.Model(&models.WAAccount{}).Count(&accounts).Error)
	require.NoError(t, db.Model(&models.CSConversation{}).Count(&conversations).Error)
	require.NoError(t, db.Model(&models.CSMessage{}).Count(&stored).Error)
	assert.Equal(t, int64(0), accounts)
	assert.Equal(t, int64(0), conversations)
	assert.Equal(t, int64(0), stored)
}

// cs_messages.reply_to_id is ON DELETE SET NULL, so purging a quoted message
// leaves the reply itself in the thread with nothing to quote. SQLite has no
// such foreign key, so there the column would keep pointing at a row that is
// gone — same rendering, different data — and only Postgres shows this.
func TestPurgingAQuotedMessageNullsTheReplyPointerOnPostgres(t *testing.T) {
	db, _, conv := purgePostgres(t)

	messages := NewCSMessageService(db, NewCSConversationService(db))
	quoted, _, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0A",
		Kind: models.MessageKindText, Body: "internet mati", At: time.Now(),
	})
	require.NoError(t, err)

	reply, err := messages.Queue(conv.ID, uuid.New(), models.MessageKindText, "sudah kami cek", nil, &quoted.ID)
	require.NoError(t, err)

	_, err = NewCSPurgeService(db, t.TempDir()).Message(quoted.ID)
	require.NoError(t, err)

	var stored models.CSMessage
	require.NoError(t, db.First(&stored, "id = ?", reply.ID).Error)
	assert.Nil(t, stored.ReplyToID, "the reply outlives the message it answered")
}

// wa_broadcast_posts references wa_accounts ON DELETE RESTRICT for the same
// reason cs_conversations does, so a number that has ever broadcast is refused
// the same way a number with threads is. Dropping the history is deliberate,
// which is why the constraint stays and DeleteAccount clears it first.
func TestDeleteAccountClearsItsBroadcastHistoryOnPostgres(t *testing.T) {
	db, account, _ := purgePostgres(t)

	root := t.TempDir()
	rel := filepath.Join("2026", "09", "pengumuman.jpg")
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o600))

	_, err := NewCSBroadcastPostService(db).Queue(BroadcastPost{
		WAAccountID:  account.ID,
		ChannelJID:   "120363000000000001@newsletter",
		SenderUserID: uuid.New(),
		Kind:         models.MessageKindImage,
		Body:         "Ada pemeliharaan malam ini",
		Media:        &MediaFile{Path: rel, Mime: "image/jpeg", Filename: "pengumuman.jpg", Size: 1},
	})
	require.NoError(t, err)

	require.NoError(t, NewCSPurgeService(db, root).DeleteAccount(account.ID))

	var accounts, posts int64
	require.NoError(t, db.Model(&models.WAAccount{}).Count(&accounts).Error)
	require.NoError(t, db.Model(&models.WABroadcastPost{}).Count(&posts).Error)
	assert.Equal(t, int64(0), accounts)
	assert.Equal(t, int64(0), posts)

	_, statErr := os.Stat(filepath.Join(root, rel))
	assert.True(t, os.IsNotExist(statErr), "the attachment goes with the row it belonged to")
}
