package services

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// releaseMigration is the one-off correction that hands back the threads the
// removed round-robin gave out.
const releaseMigration = "../../migrations/51_release_unanswered_rotation_threads.sql"

func postgresAgent(t *testing.T, db *gorm.DB, username string) uuid.UUID {
	t.Helper()
	agent := models.User{
		ID:       uuid.New(),
		Username: username,
		Email:    username + "@example.test",
		Role:     models.UserRoleCS,
	}
	require.NoError(t, db.Create(&agent).Error)
	return agent.ID
}

// SQLite serialises every write, so only Postgres can show two first replies
// arriving together — and exactly one of them must end up holding the thread.
func TestConcurrentFirstRepliesLeaveOneHolderOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)

	const agents = 8
	ids := make([]uuid.UUID, agents)
	for i := range ids {
		ids[i] = postgresAgent(t, db, fmt.Sprintf("agent%d", i))
	}

	claimed := make([]bool, agents)
	errs := make([]error, agents)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			claimed[i], errs[i] = conversations.ClaimForReply(conv.ID, ids[i])
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i := range ids {
		if claimed[i] {
			winners++
			assert.NoError(t, errs[i])
			continue
		}
		assert.ErrorIs(t, errs[i], ErrNotHolder)
	}
	assert.Equal(t, 1, winners)
}

// The rotation put threads in front of whoever had the inbox open. Those its
// holder never answered go back to the shared queue; a thread someone replied
// in, or that a person handed over, stays where it is.
func TestReleaseMigrationFreesOnlyThreadsTheRotationGaveOutOnPostgres(t *testing.T) {
	db := setupPostgresTestDB(t)
	conversations := NewCSConversationService(db)
	messages := NewCSMessageService(db, conversations)
	audit := NewAuditService(db, zap.NewNop())
	account := csAccount(t, db)
	budi := postgresAgent(t, db, "budi")
	rina := postgresAgent(t, db, "rina")

	held := func(phone string, holder uuid.UUID) uuid.UUID {
		conv, err := conversations.FindOrCreate(IncomingPeer{
			WAAccountID: account.ID, JID: phone + "@s.whatsapp.net", Phone: phone, Name: "Pelanggan",
		})
		require.NoError(t, err)
		require.NoError(t, conversations.Assign(conv.ID, holder))
		return conv.ID
	}

	neverAnswered := held("62811", budi)
	answered := held("62812", budi)
	_, err := messages.Queue(answered, budi, models.MessageKindText, "sudah kami cek", nil, nil)
	require.NoError(t, err)
	handedOver := held("62813", rina)
	require.NoError(t, audit.Log(budi, "assign", "cs_conversation", handedOver,
		nil, map[string]interface{}{"assigned_user_id": rina.String()}, "127.0.0.1", "test"))
	answeredByAnother := held("62814", budi)
	_, err = messages.Queue(answeredByAnother, rina, models.MessageKindText, "halo", nil, nil)
	require.NoError(t, err)
	closed := held("62815", budi)
	require.NoError(t, conversations.Close(closed, uuid.New()))

	migration, err := os.ReadFile(releaseMigration)
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)

	expect := func(id uuid.UUID, status models.ConversationStatus, holder *uuid.UUID) {
		t.Helper()
		conv, err := conversations.Get(id)
		require.NoError(t, err)
		assert.Equal(t, status, conv.Status)
		assert.Equal(t, holder, conv.AssignedUserID)
	}
	expect(neverAnswered, models.ConversationUnassigned, nil)
	expect(answeredByAnother, models.ConversationUnassigned, nil)
	expect(answered, models.ConversationOpen, &budi)
	expect(handedOver, models.ConversationOpen, &rina)
	expect(closed, models.ConversationClosed, &budi)
}
