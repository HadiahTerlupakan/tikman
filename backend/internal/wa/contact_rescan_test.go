package wa

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

func storedMessage(t *testing.T, db *gorm.DB, convID uuid.UUID, dir models.MessageDirection, body string, at time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&models.CSMessage{
		ID: uuid.New(), ConversationID: convID, Direction: dir, Kind: models.MessageKindText,
		Body: body, Status: models.MessageSent, CreatedAt: at,
	}).Error)
}

// A subscriber billed twice is saved under the newer name, once; customers'
// own messages and outgoing ones without a greeting name nobody.
func TestTheRescanNamesEachSubscriberFromTheirNewestBillingMessage(t *testing.T) {
	db, _, _, conv := drainSetup(t)
	now := time.Now()
	storedMessage(t, db, conv.ID, models.MessageOut, "Yth. Bapak/Ibu Budi,\ntagihan Agustus", now.Add(-48*time.Hour))
	storedMessage(t, db, conv.ID, models.MessageOut, "Yth. Bapak/Ibu Budi Santoso,\ntagihan September", now.Add(-time.Hour))
	storedMessage(t, db, conv.ID, models.MessageIn, "Yth. Bapak/Ibu Palsu, ini pelanggan", now)
	storedMessage(t, db, conv.ID, models.MessageOut, "baik Bapak/Ibu, sudah kami cek", now)

	subscribers, err := billedSubscribers(db, conv.WAAccountID)
	require.NoError(t, err)

	require.Len(t, subscribers, 1)
	assert.Equal(t, "Budi Santoso", subscribers[0].name)
	assert.Equal(t, types.NewJID("628111", types.DefaultUserServer), subscribers[0].phone)
}

func TestTheRescanReadsOnlyItsOwnNumber(t *testing.T) {
	db, _, _, conv := drainSetup(t)
	storedMessage(t, db, conv.ID, models.MessageOut, "Yth. Bapak/Ibu Budi,\n", time.Now())

	subscribers, err := billedSubscribers(db, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, subscribers)
}

// A thread opened under a LID holds the number in customer_phone when WhatsApp
// gave one, and the LID itself when it did not.
func TestAThreadsPhoneIsFoundBehindItsLID(t *testing.T) {
	assert.Equal(t, types.NewJID("628111222333", types.DefaultUserServer),
		threadPhone(customerLID, "628111222333"))
	assert.True(t, threadPhone(customerLID, customerLID.User).IsEmpty())
	assert.Equal(t, customerNumber, threadPhone(customerNumber, "628111222333"))
}
