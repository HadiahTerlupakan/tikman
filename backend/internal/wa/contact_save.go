package wa

import (
	"cmp"
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
// "Add contact" on WhatsApp Web does with "sync to phone" ticked. A status
// posted to "My contacts" reaches only numbers saved there, and a CS on the
// phone sees a name instead of a bare number.
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
	// relinked holds the billed numbers this process has saved again, with
	// their LID and the billing name. The server's contact list cannot be
	// trusted to match the phone: a save without a LID, ours before the fix
	// or another linked device's, is recorded by the server and ignored by
	// the phone, which goes on showing a bare number. So a billed number the
	// server calls saved is sent once more per process, whatever it is saved
	// as; the billing name replacing one typed on the phone is the price the
	// owner chose over numbers that never get a name.
	relinked map[types.JID]bool
	// push sends a patch to WhatsApp: the client's SendAppState, replaced in
	// tests, which have no connection to send one over.
	push func(context.Context, appstate.PatchInfo) error
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
	s.saveBilled(ctx, recipientPhoneJID(evt.Info.MessageSource), name, evt.Info.Chat.User)
}

// saveBilled saves phone under the name a billing greeting gave it, replacing
// whatever the server has it saved as (see relinked). lid is what WhatsApp
// addressed the subscriber by, logged when it gave no number.
func (s *contactSaver) saveBilled(ctx context.Context, phone types.JID, name, lid string) {
	if phone.IsEmpty() {
		s.logger.Info("Skipped a billed subscriber: WhatsApp gave only a LID, no phone number",
			zap.String("account_id", s.accountID.String()), zap.String("name", name), zap.String("lid", lid))
		return
	}
	if savedAs := s.save(ctx, phone, func() string { return name }); savedAs != "" {
		s.relink(ctx, phone, name, savedAs)
	}
}

// relink sends phone's contact again under name, with its LID, unless this
// process already has; see relinked.
func (s *contactSaver) relink(ctx context.Context, phone types.JID, name, savedAs string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.relinked[phone] {
		return
	}
	if s.relinked == nil {
		s.relinked = map[types.JID]bool{}
	}
	s.relinked[phone] = true
	if savedAs != name {
		s.logger.Info("Replacing a billed subscriber's contact name with the billing name",
			zap.String("account_id", s.accountID.String()), zap.String("phone", phone.User),
			zap.String("name", name), zap.String("saved_as", savedAs))
	}
	s.send(ctx, phone, name)
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
// the phone already has it, and answers the name it is already saved under when
// it does. nameFor runs only once that check has passed, and answering "" means
// the number should not be saved after all.
func (s *contactSaver) save(ctx context.Context, phone types.JID, nameFor func() string) (savedAs string) {
	if phone.IsEmpty() {
		// A LID with no number behind it cannot go into an Android address
		// book: there is nothing to dial.
		return ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	known, err := s.wa.Store.Contacts.GetContact(ctx, phone)
	if err != nil {
		s.logger.Warn("Could not read the WhatsApp contact store", zap.Error(err))
		return ""
	}
	if known.FullName != "" || known.FirstName != "" {
		// Already in the address book, perhaps under a name somebody chose on
		// the phone. Overwriting it would undo their work.
		return cmp.Or(known.FullName, known.FirstName)
	}

	if name := nameFor(); name != "" {
		s.send(ctx, phone, name)
	}
	return ""
}

// send pushes the contact patch for phone. The caller holds mu.
func (s *contactSaver) send(ctx context.Context, phone types.JID, name string) {
	fields := []zap.Field{
		zap.String("account_id", s.accountID.String()),
		zap.String("phone", phone.User),
		zap.String("name", name),
	}
	lid := s.lidFor(ctx, phone)
	if lid.IsEmpty() {
		// Sent without one, the server records the name and the phone ignores
		// it, after which the number reads as saved and is never tried again.
		s.logger.Warn("Not saving the customer: WhatsApp gave no LID for the number", fields...)
		return
	}
	if err := s.push(ctx, buildContactPatch(phone, lid, name)); err != nil {
		s.logger.Warn("Could not save the customer to the phone's contacts", append(fields, zap.Error(err))...)
		return
	}
	// Logged on success too: the patch's shape is copied from WhatsApp's own
	// clients, not documented, and this line is the only record of how many
	// numbers it saved.
	s.logger.Info("Saved the customer to the phone's contacts", fields...)
}

// lidFor answers phone's LID: from the store, which has it for anyone this
// number has messaged, else from WhatsApp itself.
func (s *contactSaver) lidFor(ctx context.Context, phone types.JID) types.JID {
	if lid, err := s.wa.Store.LIDs.GetLIDForPN(ctx, phone); err == nil && !lid.IsEmpty() {
		return lid
	}
	info, err := s.wa.GetUserInfo(ctx, []types.JID{phone})
	if err != nil {
		s.logger.Warn("Could not ask WhatsApp for the customer's LID", zap.String("phone", phone.User), zap.Error(err))
		return types.EmptyJID
	}
	return info[phone].LID
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

// buildContactPatch is the app-state mutation the phone itself sends when a
// contact is saved on it: indexed by number, carrying the contact's LID and
// no pnJid. The LID is what the phone acts on; a patch without one is stored
// by the server and silently ignored by the phone. SaveOnPrimaryAddressbook
// carries it past WhatsApp into the phone's own contacts.
func buildContactPatch(phone, lid types.JID, name string) appstate.PatchInfo {
	return appstate.PatchInfo{
		Type: appstate.WAPatchCriticalUnblockLow,
		Mutations: []appstate.MutationInfo{{
			Index:   []string{appstate.IndexContact, phone.String()},
			Version: contactMutationVersion,
			Value: &waSyncAction.SyncActionValue{
				ContactAction: &waSyncAction.ContactAction{
					FullName:                 proto.String(name),
					FirstName:                proto.String(name),
					LidJID:                   proto.String(lid.String()),
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
