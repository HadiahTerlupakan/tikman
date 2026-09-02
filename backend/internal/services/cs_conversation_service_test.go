package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

func csAccount(t *testing.T, db *gorm.DB) models.WAAccount {
	account := models.WAAccount{Label: "CS Utama", Status: models.WAAccountConnected}
	require.NoError(t, db.Create(&account).Error)
	return account
}

func peer(accountID uuid.UUID) IncomingPeer {
	return IncomingPeer{
		WAAccountID: accountID,
		JID:         "6281234567890@s.whatsapp.net",
		Phone:       "081234567890",
		Name:        "Pak Budi",
	}
}

func TestFindOrCreateStartsAThreadNobodyHolds(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, models.ConversationUnassigned, conv.Status)
	assert.Nil(t, conv.AssignedUserID)
	assert.Equal(t, "6281234567890", conv.CustomerPhone, "the number is stored in one form regardless of how it arrived")
}

func TestFindOrCreateReturnsTheSameThreadTwice(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	first, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)
	second, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
}

// A customer whose ONT is known should arrive with it already attached, so the
// CS sees the light levels without hunting for the subscriber first.
func TestFindOrCreateAttachesTheONTThatOwnsTheNumber(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	ont := models.ONT{
		OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "ZTEG12345678", Name: "Budi", Phone: "6281234567890",
	}
	require.NoError(t, db.Create(&ont).Error)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	require.NotNil(t, conv.ONTID)
	assert.Equal(t, ont.ID, *conv.ONTID)
}

// A customer who writes again after their case was closed is a new problem, not
// a footnote to the old one — and it must land in somebody's queue again.
func TestFindOrCreateReopensAClosedThreadAndDropsItsFormerHolder(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	holder := uuid.New()
	require.NoError(t, svc.Assign(conv.ID, holder))
	require.NoError(t, svc.Close(conv.ID))

	reopened, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.Equal(t, conv.ID, reopened.ID)
	assert.Equal(t, models.ConversationUnassigned, reopened.Status)
	assert.Nil(t, reopened.AssignedUserID)
}

func TestEnsureHolderRefusesSomeoneElsesThread(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	holder, intruder := uuid.New(), uuid.New()
	require.NoError(t, svc.Assign(conv.ID, holder))

	require.NoError(t, svc.EnsureHolder(conv.ID, holder))
	assert.ErrorIs(t, svc.EnsureHolder(conv.ID, intruder), ErrNotHolder)
}

func TestEnsureHolderRefusesAThreadNobodyHolds(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)

	conv, err := svc.FindOrCreate(peer(account.ID))
	require.NoError(t, err)

	assert.ErrorIs(t, svc.EnsureHolder(conv.ID, uuid.New()), ErrNotHolder)
}

func TestListSeparatesMineFromUnheld(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCSConversationService(db)
	account := csAccount(t, db)
	me := uuid.New()

	mine, err := svc.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628111@s.whatsapp.net", Phone: "628111222333", Name: "A",
	})
	require.NoError(t, err)
	require.NoError(t, svc.Assign(mine.ID, me))

	_, err = svc.FindOrCreate(IncomingPeer{
		WAAccountID: account.ID, JID: "628222@s.whatsapp.net", Phone: "628222333444", Name: "B",
	})
	require.NoError(t, err)

	held, err := svc.List(ConversationFilter{Mine: &me})
	require.NoError(t, err)
	require.Len(t, held, 1)
	assert.Equal(t, mine.ID, held[0].ID)

	waiting, err := svc.List(ConversationFilter{Unassigned: true})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, "628222333444", waiting[0].CustomerPhone)
}
