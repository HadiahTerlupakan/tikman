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

// The patch must match what the phone itself sends when a contact is saved on
// it. The first version carried pnJid and no lidJid: the server recorded every
// one and the phone showed none of them.
func TestTheContactPatchMatchesWhatThePhoneSends(t *testing.T) {
	phone := types.JID{User: "628111222333", Server: types.DefaultUserServer}

	patch := buildContactPatch(phone, customerLID, "Budi")

	assert.Equal(t, appstate.WAPatchCriticalUnblockLow, patch.Type)
	require.Len(t, patch.Mutations, 1)
	m := patch.Mutations[0]
	assert.Equal(t, []string{"contact", "628111222333@s.whatsapp.net"}, m.Index)
	assert.Equal(t, int32(2), m.Version)
	act := m.Value.GetContactAction()
	assert.True(t, act.GetSaveOnPrimaryAddressbook())
	assert.Equal(t, "Budi", act.GetFullName())
	assert.Equal(t, customerLID.String(), act.GetLidJID())
	assert.Nil(t, act.PnJID)
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
// GetContact is implemented: nothing a billed save does reaches the rest.
type knownContacts struct {
	store.ContactStore
	names map[string]string
}

func (k knownContacts) GetContact(_ context.Context, jid types.JID) (types.ContactInfo, error) {
	name, ok := k.names[jid.User]
	return types.ContactInfo{Found: ok, FullName: name}, nil
}

// lidOf is a LID store that knows customerNumber as customerLID.
type lidOf struct{ store.LIDStore }

func (lidOf) GetLIDForPN(_ context.Context, pn types.JID) (types.JID, error) {
	if pn.User == customerNumber.User {
		return customerLID, nil
	}
	return types.EmptyJID, nil
}

// billedSaver answers a saver over a store holding names, and the patches it
// pushes to WhatsApp.
func billedSaver(t *testing.T, names map[string]string) (*contactSaver, *[]appstate.PatchInfo, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	wac := &whatsmeow.Client{Store: &store.Device{Contacts: knownContacts{names: names}, LIDs: lidOf{}}}
	var pushed []appstate.PatchInfo
	push := func(_ context.Context, p appstate.PatchInfo) error {
		pushed = append(pushed, p)
		return nil
	}
	return &contactSaver{wa: wac, push: push, accountID: uuid.New(), logger: zap.New(core)}, &pushed, logs
}

func billingSends(chat, recipientAlt types.JID) *events.Message {
	return phoneSends("3EB0BILL", "Invoice Telah Tersedia\n\nYth. Bapak/Ibu Bayu Alif Anggoro,\n", chat, recipientAlt)
}

// The server can call a number saved that the phone shows bare: a save without
// a LID is recorded and ignored. So a billed number saved under another name is
// sent again under the billing name, once per process.
func TestABilledSubscriberSavedUnderAnotherNameIsSavedAgainUnderTheBillingName(t *testing.T) {
	saver, pushed, logs := billedSaver(t, map[string]string{customerNumber.User: "Bayu Anggoro"})

	saver.ensureBilled(context.Background(), billingSends(customerNumber, types.EmptyJID))
	saver.ensureBilled(context.Background(), billingSends(customerNumber, types.EmptyJID))

	require.Len(t, *pushed, 1)
	act := (*pushed)[0].Mutations[0].Value.GetContactAction()
	assert.Equal(t, "Bayu Alif Anggoro", act.GetFullName())
	assert.Equal(t, customerLID.String(), act.GetLidJID())

	entries := logs.FilterMessage("Replacing a billed subscriber's contact name with the billing name").All()
	require.Len(t, entries, 1)
	assert.Equal(t, "Bayu Anggoro", entries[0].ContextMap()["saved_as"])
}

func TestABilledSubscriberKnownOnlyByLIDIsLogged(t *testing.T) {
	saver, pushed, logs := billedSaver(t, nil)

	saver.ensureBilled(context.Background(), billingSends(customerLID, types.EmptyJID))

	assert.Empty(t, *pushed)
	entries := logs.FilterMessage("Skipped a billed subscriber: WhatsApp gave only a LID, no phone number").All()
	require.Len(t, entries, 1)
	assert.Equal(t, customerLID.User, entries[0].ContextMap()["lid"])
}
