package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func csTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrate(db))
	return db
}

func TestCSConversationGetsAnIDOnCreate(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)
	require.NotEqual(t, uuid.Nil, account.ID)

	conv := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.NoError(t, db.Create(&conv).Error)
	require.NotEqual(t, uuid.Nil, conv.ID)
}

// One customer may hold only one thread per number: a second row would split
// their history in two, and the CS reading one half would answer blind.
func TestOneThreadPerCustomerPerAccount(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)

	first := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.NoError(t, db.Create(&first).Error)

	second := CSConversation{
		WAAccountID:   account.ID,
		CustomerJID:   "628111@s.whatsapp.net",
		CustomerPhone: "628111",
		Status:        ConversationUnassigned,
	}
	require.Error(t, db.Create(&second).Error)
}

// An outbound message has no WhatsApp id until it is actually sent, so the
// column must hold many empty values at once. A plain unique index would let
// the second queued message collide with the first.
func TestManyMessagesMayWaitWithoutAWhatsAppID(t *testing.T) {
	db := csTestDB(t)

	account := WAAccount{Label: "CS Utama", Status: WAAccountDisconnected}
	require.NoError(t, db.Create(&account).Error)
	holder := uuid.New()
	conv := CSConversation{
		WAAccountID:    account.ID,
		CustomerJID:    "628111@s.whatsapp.net",
		CustomerPhone:  "628111",
		Status:         ConversationOpen,
		AssignedUserID: &holder,
	}
	require.NoError(t, db.Create(&conv).Error)

	for i := 0; i < 2; i++ {
		msg := CSMessage{
			ConversationID: conv.ID,
			Direction:      MessageOut,
			Kind:           MessageKindText,
			Body:           "halo",
			Status:         MessageQueued,
		}
		require.NoError(t, db.Create(&msg).Error)
		require.Nil(t, msg.WAMessageID)
	}
}
