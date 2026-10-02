package wa

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

// Without SaveOnPrimaryAddressbook the contact lives only inside WhatsApp and
// never reaches the Android address book, which is the whole point. The index,
// collection and version are what WhatsApp Web sends; any other shape is
// rejected or silently ignored by the phone.
func TestTheContactPatchAsksForThePhonesAddressBook(t *testing.T) {
	phone := types.JID{User: "628111222333", Server: types.DefaultUserServer}

	patch := buildContactPatch(phone, "Budi")

	assert.Equal(t, appstate.WAPatchCriticalUnblockLow, patch.Type)
	require.Len(t, patch.Mutations, 1)
	m := patch.Mutations[0]
	assert.Equal(t, []string{"contact", "628111222333@s.whatsapp.net"}, m.Index)
	assert.Equal(t, int32(2), m.Version)
	act := m.Value.GetContactAction()
	assert.True(t, act.GetSaveOnPrimaryAddressbook())
	assert.Equal(t, "Budi", act.GetFullName())
	assert.Equal(t, "628111222333@s.whatsapp.net", act.GetPnJID())
}

// The ISP knows a subscriber by the name on their ONT, not by whatever they
// typed into WhatsApp, so a thread tied to an ONT is saved under that.
func TestTheContactIsNamedAfterTheSubscribersONT(t *testing.T) {
	db, _, _, conv := drainSetup(t)
	ont := models.ONT{
		OLTID: uuid.New(), PortID: 1, ONTID: 1,
		SerialNumber: "ZTEG12345678", Name: "Budi Santoso - Blok A3", Phone: "628111222333",
	}
	require.NoError(t, db.Create(&ont).Error)
	conv.ONTID = &ont.ID

	assert.Equal(t, "Budi Santoso - Blok A3", contactName(db, conv, "budi 🙂"))
}

func TestTheContactFallsBackToTheWhatsAppNameThenTheNumber(t *testing.T) {
	db, _, _, conv := drainSetup(t)

	assert.Equal(t, "budi 🙂", contactName(db, conv, "budi 🙂"))
	assert.Equal(t, "+628111222333", contactName(db, conv, ""))
}

// WhatsApp increasingly names the sender by LID and puts the number in
// SenderAlt. Only a number can go into an address book.
func TestTheSendersNumberIsFoundBehindALID(t *testing.T) {
	number := types.JID{User: "628111222333", Server: types.DefaultUserServer}
	lid := types.JID{User: "111222333444555", Server: types.HiddenUserServer}

	assert.Equal(t, number, senderPhoneJID(types.MessageSource{Sender: lid, SenderAlt: number}))
	assert.Equal(t, number, senderPhoneJID(types.MessageSource{Sender: number, SenderAlt: lid}))
	assert.True(t, senderPhoneJID(types.MessageSource{Sender: lid}).IsEmpty())
}
