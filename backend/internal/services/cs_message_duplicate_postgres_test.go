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

// The lookup in saveArrived can only see what was committed when it ran, so
// the partial unique index from migration 41 is what actually decides a
// duplicate. Only Postgres has that index, so only Postgres can show what
// happens when it fires — and it does fire in production, on a reply whose
// WhatsApp id MarkSent writes while the phone's echo of it is being stored.
func TestAMessageCommittedMidSaveIsStoredOnceAndIsNoErrorOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)

	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	const waID = "3EB0RACE"
	winner := insertRacingMessage(t, db, conv.ID, waID)

	msg, created, err := messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: waID,
		Kind: models.MessageKindText, Body: "internet mati",
		At: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err, "the row the index points at holds the message already")
	assert.False(t, created, "nobody should be told to come and read it twice")
	require.NotNil(t, msg)
	assert.Equal(t, winner, msg.ID, "the stored message is the one that won the race")

	var stored int64
	require.NoError(t, db.Model(&models.CSMessage{}).
		Where("wa_message_id = ?", waID).Count(&stored).Error)
	assert.EqualValues(t, 1, stored)
}

// insertRacingMessage arranges the interleaving the lookup cannot survive: the
// other delivery commits on its own connection after saveArrived has already
// looked and found nothing, leaving only the index to catch it. Raw SQL on
// purpose — a Create here would re-enter this very callback.
func insertRacingMessage(t *testing.T, db *gorm.DB, conversationID uuid.UUID, waMessageID string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	const name = "test:race_before_create"
	done := false
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		if done || tx.Statement.Table != "cs_messages" {
			return
		}
		done = true
		require.NoError(t, db.Exec(`
			INSERT INTO cs_messages
				(id, conversation_id, wa_message_id, direction, kind, body, status, wa_timestamp, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, conversationID, waMessageID, models.MessageIn, models.MessageKindText,
			"internet mati", models.MessageDelivered, time.Now().UTC(), time.Now().UTC(),
		).Error)
	}))
	t.Cleanup(func() {
		assert.NoError(t, db.Callback().Create().Remove(name))
	})
	return id
}
