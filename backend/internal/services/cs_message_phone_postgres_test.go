package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// The ordering guard compares timestamps in SQL. SQLite compares them as text,
// so only Postgres says whether the guard holds where it actually runs.
func TestAPhoneReplyAnswersOnlyWhenItIsTheNewestMessageOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	conv, err := conversations.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "Budi",
	})
	require.NoError(t, err)

	customerAt := time.Date(2026, 9, 15, 3, 0, 0, 0, time.UTC)
	_, _, err = messages.SaveInbound(InboundMessage{
		ConversationID: conv.ID, WAMessageID: "3EB0CUSTOMER",
		Kind: models.MessageKindText, Body: "internet mati", At: customerAt,
	})
	require.NoError(t, err)

	_, _, err = messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0LATE", customerAt.Add(-time.Minute)))
	require.NoError(t, err)
	waiting, err := conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1, "a reply older than the customer's message answers nothing")

	_, _, err = messages.SaveFromPhone(phoneMessage(conv.ID, "3EB0ANSWER", customerAt.Add(time.Minute)))
	require.NoError(t, err)
	waiting, err = conversations.List(ConversationFilter{AwaitingReply: true})
	require.NoError(t, err)
	assert.Empty(t, waiting)
}
