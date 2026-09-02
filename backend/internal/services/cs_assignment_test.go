package services

import (
	"context"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func waitingConversation(t *testing.T, svc *CSConversationService, accountID uuid.UUID, jid, phone string) *models.CSConversation {
	conv, err := svc.FindOrCreate(IncomingPeer{WAAccountID: accountID, JID: jid, Phone: phone, Name: "X"})
	require.NoError(t, err)
	return conv
}

func assignmentSetup(t *testing.T, online ...uuid.UUID) (*gorm.DB, *CSConversationService, *CSAssignmentService, models.WAAccount, *FakePresence) {
	db := setupTestDB(t)
	conversations := NewCSConversationService(db)
	presence := NewFakePresence(online...)
	return db, conversations, NewCSAssignmentService(db, conversations, presence), csAccount(t, db), presence
}

func TestAssignmentSharesThreadsRoundTheTeam(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	_, conversations, assignment, account, _ := assignmentSetup(t, a, b)
	ctx := context.Background()

	first := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	second := waitingConversation(t, conversations, account.ID, "628222@s.whatsapp.net", "628222333444")
	third := waitingConversation(t, conversations, account.ID, "628333@s.whatsapp.net", "628333444555")

	var got []uuid.UUID
	for _, conv := range []*models.CSConversation{first, second, third} {
		holder, err := assignment.AssignOne(ctx, conv.ID)
		require.NoError(t, err)
		require.NotNil(t, holder)
		got = append(got, *holder)
	}

	// Two agents, three threads: each agent gets at least one, and the third
	// wraps back round rather than piling onto whoever was picked first.
	assert.NotEqual(t, got[0], got[1], "consecutive threads must not land on the same agent")
	assert.Equal(t, got[0], got[2], "the rotation wraps")

	team := []uuid.UUID{a, b}
	sort.Slice(team, func(i, j int) bool { return team[i].String() < team[j].String() })
	assert.Contains(t, team, got[0])
}

// A thread arriving at night must wait, not disappear into the inbox of
// somebody who went home four hours ago.
func TestAssignmentLeavesAThreadWaitingWhenNobodyIsOnline(t *testing.T) {
	_, conversations, assignment, account, _ := assignmentSetup(t)
	ctx := context.Background()

	conv := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")

	holder, err := assignment.AssignOne(ctx, conv.ID)
	require.NoError(t, err)
	assert.Nil(t, holder)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversationUnassigned, after.Status)
	assert.Nil(t, after.AssignedUserID)
}

func TestAssignWaitingHandsOutEverythingLeftOvernight(t *testing.T) {
	a := uuid.New()
	_, conversations, assignment, account, presence := assignmentSetup(t)
	ctx := context.Background()

	first := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	second := waitingConversation(t, conversations, account.ID, "628222@s.whatsapp.net", "628222333444")

	// Nobody was online when these arrived.
	for _, conv := range []*models.CSConversation{first, second} {
		holder, err := assignment.AssignOne(ctx, conv.ID)
		require.NoError(t, err)
		require.Nil(t, holder)
	}

	presence.SetOnline(a)

	n, err := assignment.AssignWaiting(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	for _, conv := range []*models.CSConversation{first, second} {
		after, err := conversations.Get(conv.ID)
		require.NoError(t, err)
		assert.Equal(t, models.ConversationOpen, after.Status)
		require.NotNil(t, after.AssignedUserID)
		assert.Equal(t, a, *after.AssignedUserID)
	}
}

// A customer's second message must not move their thread to another agent.
func TestAssignOneLeavesAThreadThatAlreadyHasAHolder(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	_, conversations, assignment, account, _ := assignmentSetup(t, a, b)
	ctx := context.Background()

	conv := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")

	first, err := assignment.AssignOne(ctx, conv.ID)
	require.NoError(t, err)
	require.NotNil(t, first)

	again, err := assignment.AssignOne(ctx, conv.ID)
	require.NoError(t, err)
	assert.Nil(t, again, "a thread somebody already holds is not handed out again")

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotNil(t, after.AssignedUserID)
	assert.Equal(t, *first, *after.AssignedUserID, "and it stays with the agent who had it")

	// The refusal must also not have burned a rotation turn, or every message on
	// an open thread would skew the share-out for everyone else.
	next := waitingConversation(t, conversations, account.ID, "628222@s.whatsapp.net", "628222333444")
	holder, err := assignment.AssignOne(ctx, next.ID)
	require.NoError(t, err)
	require.NotNil(t, holder)
	assert.NotEqual(t, *first, *holder, "the rotation advanced by exactly one")
}

// A closed thread is finished; the morning sweep must not drag it back out.
func TestAssignWaitingIgnoresClosedThreads(t *testing.T) {
	a := uuid.New()
	_, conversations, assignment, account, _ := assignmentSetup(t, a)
	ctx := context.Background()

	conv := waitingConversation(t, conversations, account.ID, "628111@s.whatsapp.net", "628111222333")
	require.NoError(t, conversations.Close(conv.ID))

	n, err := assignment.AssignWaiting(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}
