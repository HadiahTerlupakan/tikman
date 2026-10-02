package wa

import (
	"context"
	"regexp"
	"strings"
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

// contactSaver puts a customer who writes in, or a subscriber the billing app
// writes to, into the address book of the phone holding the number, the way
// "Add contact" on WhatsApp Web does with "sync to phone" ticked. A status posted to "My contacts" reaches only numbers saved
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

// billedGreeting is the salutation the billing app opens every message to a
// subscriber with ("Yth. Bapak/Ibu ooy,"). It is the one place those messages
// say who the number belongs to.
var billedGreeting = regexp.MustCompile(`(?i)Yth\.?\s+Bapak/Ibu\s+([^,\n]+),`)

// ensure saves the customer who sent evt unless the phone already has them.
func (s *contactSaver) ensure(ctx context.Context, evt *events.Message) {
	s.save(ctx, senderPhoneJID(evt.Info.MessageSource), func() string {
		conv, err := s.conversations.FindByPeer(s.accountID, evt.Info.Chat.ToNonAD().String())
		if err != nil || conv == nil {
			// No thread means this is not a customer of the inbox: a group, a
			// channel, or a message attachmentFor turned away.
			return ""
		}
		return contactName(s.db, conv, evt.Info.PushName)
	})
}

// ensureBilled saves the subscriber a billing message from another device on
// this number was addressed to, under the name its greeting uses. The billing
// app sends from its own linked device, so these reach TikMan only as the
// number's own messages, usually to people with no thread here.
func (s *contactSaver) ensureBilled(ctx context.Context, evt *events.Message) {
	if evt.Info.IsGroup || evt.Info.Chat.Server == types.NewsletterServer {
		return
	}
	name := billedName(textBody(evt.Message))
	if name == "" {
		return
	}
	s.save(ctx, recipientPhoneJID(evt.Info.MessageSource), func() string { return name })
}

// billedName answers the subscriber's name from a billing greeting, or "" when
// the text is not one.
func billedName(text string) string {
	m := billedGreeting.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// save adds phone to the address book under the name nameFor answers, unless
// the phone already has it. nameFor runs only once that check has passed, and
// answering "" means the number should not be saved after all.
func (s *contactSaver) save(ctx context.Context, phone types.JID, nameFor func() string) {
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

	name := nameFor()
	if name == "" {
		return
	}
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
	return phoneJID(src.Sender, src.SenderAlt)
}

// recipientPhoneJID is senderPhoneJID for a message this number sent.
func recipientPhoneJID(src types.MessageSource) types.JID {
	return phoneJID(src.Chat, src.RecipientAlt)
}

func phoneJID(addresses ...types.JID) types.JID {
	for _, jid := range addresses {
		if jid.Server == types.DefaultUserServer {
			return jid.ToNonAD()
		}
	}
	return types.EmptyJID
}
