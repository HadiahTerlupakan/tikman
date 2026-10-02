package wa

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// contactMutationVersion is the version WhatsApp's own clients stamp on a
// "contact" mutation. whatsmeow has no builder for it, so it is copied from
// what WhatsApp Web sends (Baileys' addOrEditContact uses the same).
const contactMutationVersion = 2

// contactSaver puts a customer who writes in into the address book of the phone
// holding the number, the way "Add contact" on WhatsApp Web does with "sync to
// phone" ticked. A status posted to "My contacts" reaches only numbers saved
// there, and a CS on the phone sees a name instead of a bare number.
type contactSaver struct {
	wa            *whatsmeow.Client
	db            *gorm.DB
	accountID     uuid.UUID
	conversations *services.CSConversationService
	logger        *zap.Logger
	// mu keeps one save in flight per number. A customer's first burst arrives
	// as several messages, and without it each would push the same patch while
	// the store still says "unsaved"; all but one would bounce off WhatsApp as
	// a 409 conflict. Behind the lock the second one finds the name the first
	// one's post-send fetch wrote, and stops.
	mu sync.Mutex
}

// ensure saves the customer behind evt unless the phone already has them.
func (s *contactSaver) ensure(ctx context.Context, evt *events.Message) {
	phone := senderPhoneJID(evt.Info.MessageSource)
	if phone.IsEmpty() {
		// A LID with no number behind it cannot go into an Android address
		// book: there is nothing to dial.
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	known, err := s.wa.Store.Contacts.GetContact(ctx, phone)
	if err != nil {
		s.logger.Warn("Could not read the WhatsApp contact store", zap.Error(err))
		return
	}
	if known.FullName != "" || known.FirstName != "" {
		// Already in the address book, perhaps under a name somebody chose on
		// the phone. Overwriting it would undo their work.
		return
	}

	conv, err := s.conversations.FindByPeer(s.accountID, evt.Info.Chat.ToNonAD().String())
	if err != nil || conv == nil {
		// No thread means this is not a customer of the inbox: a group, a
		// channel, or a message attachmentFor turned away.
		return
	}

	name := contactName(s.db, conv, evt.Info.PushName)
	if err := s.wa.SendAppState(ctx, buildContactPatch(phone, name)); err != nil {
		s.logger.Warn("Could not save the customer to the phone's contacts",
			zap.String("phone", phone.User), zap.Error(err))
	}
}

// contactName picks what the customer is saved as: the subscriber name on
// their ONT when the thread is tied to one, since that is who the ISP knows
// them as; otherwise the name they gave WhatsApp; otherwise their number.
func contactName(db *gorm.DB, conv *models.CSConversation, pushName string) string {
	if conv.ONTID != nil {
		var ont models.ONT
		if err := db.Select("name").First(&ont, "id = ?", *conv.ONTID).Error; err == nil && ont.Name != "" {
			return ont.Name
		}
	}
	if pushName != "" {
		return pushName
	}
	return "+" + conv.CustomerPhone
}

// buildContactPatch is the app-state mutation WhatsApp Web sends for "Add
// contact". SaveOnPrimaryAddressbook is what carries it past WhatsApp into the
// phone's own contacts; without it the contact exists only inside WhatsApp.
func buildContactPatch(phone types.JID, name string) appstate.PatchInfo {
	return appstate.PatchInfo{
		Type: appstate.WAPatchCriticalUnblockLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexContact, phone.String()},
			Version: contactMutationVersion,
			Value: &waSyncAction.SyncActionValue{
				ContactAction: &waSyncAction.ContactAction{
					FullName:                 proto.String(name),
					FirstName:                proto.String(name),
					PnJID:                    proto.String(phone.String()),
					SaveOnPrimaryAddressbook: proto.Bool(true),
				},
			},
		}},
	}
}

// senderPhoneJID answers the sender's phone-number address, whichever of the
// two WhatsApp put it in, and the empty JID when it gave only a LID.
func senderPhoneJID(src types.MessageSource) types.JID {
	for _, jid := range []types.JID{src.Sender, src.SenderAlt} {
		if jid.Server == types.DefaultUserServer {
			return jid.ToNonAD()
		}
	}
	return types.EmptyJID
}
