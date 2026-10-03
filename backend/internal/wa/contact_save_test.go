package wa

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
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

// The billing app's receipt is the only place that names the subscriber when
// TikMan has never heard from them.
func TestTheSubscriberIsNamedFromTheBillingGreeting(t *testing.T) {
	receipt := "Terima Kasih atas Pembayaran Anda\n\n" +
		"Yth. Bapak/Ibu ooy,\n" +
		"Pembayaran invoice *INV-992633031813* telah berhasil kami terima."

	assert.Equal(t, "ooy", billedName(receipt))
	assert.Equal(t, "Siti Aminah", billedName("Yth. Bapak/Ibu  Siti Aminah ,\ntagihan bulan ini"))
	assert.Empty(t, billedName("sudah kami cek, Bapak/Ibu"))
}

// The billing app sends from its own linked device, so its message arrives as
// ours, addressed by Chat — or by LID with the number in RecipientAlt.
func TestTheBilledNumberIsTheRecipientNotTheSender(t *testing.T) {
	number := types.JID{User: "628111222333", Server: types.DefaultUserServer}
	lid := types.JID{User: "111222333444555", Server: types.HiddenUserServer}
	us := types.JID{User: "6285133124350", Server: types.DefaultUserServer}

	assert.Equal(t, number, recipientPhoneJID(types.MessageSource{Chat: number, Sender: us}))
	assert.Equal(t, number, recipientPhoneJID(types.MessageSource{Chat: lid, Sender: us, RecipientAlt: number}))
	assert.True(t, recipientPhoneJID(types.MessageSource{Chat: lid, Sender: us}).IsEmpty())
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

// knownContacts is a contact store holding the numbers in names. Only
// GetContact is implemented: a skip never reaches anything else.
type knownContacts struct {
	store.ContactStore
	names map[string]string
}

func (k knownContacts) GetContact(_ context.Context, jid types.JID) (types.ContactInfo, error) {
	name, ok := k.names[jid.User]
	return types.ContactInfo{Found: ok, FullName: name}, nil
}

func billedSaver(t *testing.T, names map[string]string) (*contactSaver, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	wac := &whatsmeow.Client{Store: &store.Device{Contacts: knownContacts{names: names}}}
	return &contactSaver{wa: wac, accountID: uuid.New(), logger: zap.New(core)}, logs
}

func billingSends(chat, recipientAlt types.JID) *events.Message {
	return phoneSends("3EB0BILL", "Invoice Telah Tersedia\n\nYth. Bapak/Ibu Eti Sulastri,\n", chat, recipientAlt)
}

// A billed subscriber the phone already has is the commonest skip, and without
// a line saying so nobody can tell it from a save that never happened.
func TestABilledSubscriberAlreadySavedIsLoggedWithTheNameTheyHave(t *testing.T) {
	saver, logs := billedSaver(t, map[string]string{customerNumber.User: "Bu Eti RT 03"})

	saver.ensureBilled(context.Background(), billingSends(customerNumber, types.EmptyJID))

	entries := logs.FilterMessage("Skipped a billed subscriber already in the phone's contacts").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	assert.Equal(t, "Eti Sulastri", fields["name"])
	assert.Equal(t, "Bu Eti RT 03", fields["saved_as"])
	assert.Equal(t, customerNumber.User, fields["phone"])
}

func TestABilledSubscriberKnownOnlyByLIDIsLogged(t *testing.T) {
	saver, logs := billedSaver(t, nil)

	saver.ensureBilled(context.Background(), billingSends(customerLID, types.EmptyJID))

	entries := logs.FilterMessage("Skipped a billed subscriber: WhatsApp gave only a LID, no phone number").All()
	require.Len(t, entries, 1)
	assert.Equal(t, customerLID.User, entries[0].ContextMap()["lid"])
}
