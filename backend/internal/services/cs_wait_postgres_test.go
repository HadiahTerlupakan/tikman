package services

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// assertWaitMatchesThread checks the rule the recording keeps: a thread whose
// last word is the customer's has exactly one open wait, and a thread we
// answered last has none. A message folded into a wait somebody closed at the
// same moment would break it, and nobody would be recorded as waiting.
func assertWaitMatchesThread(t *testing.T, db *gorm.DB, conversationID uuid.UUID) {
	t.Helper()
	var conv models.CSConversation
	require.NoError(t, db.First(&conv, "id = ?", conversationID).Error)
	var open int64
	require.NoError(t, db.Model(&models.CSWait{}).
		Where("conversation_id = ? AND ended_at IS NULL", conversationID).Count(&open).Error)

	if conv.LastMessageDirection == models.MessageIn {
		assert.Equal(t, int64(1), open, "a customer with the last word is waiting")
		return
	}
	assert.Equal(t, int64(0), open, "a thread we answered last waits on nobody")
}

// SQLite serialises every write, so only Postgres can show a customer's message
// and a CS's reply landing together.
func TestAMessageAndAReplyArrivingTogetherNeverLoseAWaitOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	account := csAccount(t, db)
	agent := postgresAgent(t, db, "waitagent")

	for i := 0; i < 20; i++ {
		number := fmt.Sprintf("62811100%04d", i)
		conv, err := conversations.FindOrCreate(IncomingPeer{
			WAAccountID: account.ID, JID: number + "@s.whatsapp.net", Phone: number, Name: "Pelanggan",
		})
		require.NoError(t, err)
		_, _, err = messages.SaveInbound(InboundMessage{
			ConversationID: conv.ID, WAMessageID: fmt.Sprintf("3EB0FIRST%d", i),
			Kind: models.MessageKindText, Body: "halo", At: time.Now(),
		})
		require.NoError(t, err)

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, _, err := messages.SaveInbound(InboundMessage{
				ConversationID: conv.ID, WAMessageID: fmt.Sprintf("3EB0AGAIN%d", i),
				Kind: models.MessageKindText, Body: "masih mati", At: time.Now(),
			})
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, err := messages.Queue(conv.ID, agent, models.MessageKindText, "sudah kami cek", nil, nil)
			assert.NoError(t, err)
		}()
		close(start)
		wg.Wait()

		assertWaitMatchesThread(t, db, conv.ID)
	}
}
