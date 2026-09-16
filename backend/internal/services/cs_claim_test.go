package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func claimSetup(t *testing.T) (*CSConversationService, *models.CSConversation, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	conv, err := conversations.FindOrCreate(peer(csAccount(t, db).ID))
	require.NoError(t, err)

	users := NewUserService(db)
	budi, err := users.Create("budi", "budi@example.com", "password123", "", models.UserRoleCS)
	require.NoError(t, err)
	rina, err := users.Create("rina", "rina@example.com", "password123", "", models.UserRoleCS)
	require.NoError(t, err)
	return conversations, conv, budi.ID, rina.ID
}

// Having the inbox open used to make a CS the holder of whatever arrived.
// Answering is what should: the first reply decides who serves the customer.
func TestClaimForReplyMakesTheFirstReplierTheHolder(t *testing.T) {
	conversations, conv, budi, _ := claimSetup(t)

	claimed, err := conversations.ClaimForReply(conv.ID, budi)

	require.NoError(t, err)
	assert.True(t, claimed)
	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationOpen, after.Status)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, budi, *after.AssignedUserID)
}

// Two CS answering one customer is the collision the inbox exists to prevent.
func TestClaimForReplyRefusesSomeoneWhoDoesNotHoldTheThread(t *testing.T) {
	conversations, conv, budi, rina := claimSetup(t)
	_, err := conversations.ClaimForReply(conv.ID, budi)
	require.NoError(t, err)

	claimed, err := conversations.ClaimForReply(conv.ID, rina)

	assert.ErrorIs(t, err, ErrNotHolder)
	assert.False(t, claimed)
	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, budi, *after.AssignedUserID)
}

func TestClaimForReplyLetsTheHolderKeepAnswering(t *testing.T) {
	conversations, conv, budi, _ := claimSetup(t)
	_, err := conversations.ClaimForReply(conv.ID, budi)
	require.NoError(t, err)

	claimed, err := conversations.ClaimForReply(conv.ID, budi)

	require.NoError(t, err)
	assert.False(t, claimed, "only the first reply is a claim")
}

// A finished thread keeps its holder for the record. Picking it back up is a
// takeover, not something a reply does on the side.
func TestClaimForReplyDoesNotTakeAClosedThread(t *testing.T) {
	conversations, conv, budi, rina := claimSetup(t)
	_, err := conversations.ClaimForReply(conv.ID, budi)
	require.NoError(t, err)
	require.NoError(t, conversations.Close(conv.ID, uuid.New()))

	_, err = conversations.ClaimForReply(conv.ID, rina)

	assert.ErrorIs(t, err, ErrNotHolder)
	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationClosed, after.Status)
	assert.Equal(t, budi, *after.AssignedUserID)
}
