package wa

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// locationExtra is what a location stores in WAGroupMessage.Extra.
type locationExtra struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
	URL       string  `json:"url"`
}

// contactCard is one card of a contact message.
type contactCard struct {
	DisplayName string   `json:"display_name"`
	Phones      []string `json:"phones"`
}

type contactExtra struct {
	Contacts []contactCard `json:"contacts"`
}

// groupShape reads a group message, including the three shapes the customer
// inbox cannot hold — locations, contact cards and polls — which a group
// gets far more often than a customer sends them.
func (h *inboundHandler) groupShape(evt *events.Message) (attachment, bool) {
	if att, ok := structuredShape(evt.Message); ok {
		return att, true
	}
	return h.readable(evt)
}

func structuredShape(msg *waE2E.Message) (attachment, bool) {
	switch {
	case msg.GetLocationMessage() != nil:
		return locationShape(msg.GetLocationMessage()), true
	case msg.GetLiveLocationMessage() != nil:
		live := msg.GetLiveLocationMessage()
		return encodeShape(models.MessageKindLocation, "Lokasi terkini", locationExtra{
			Latitude: live.GetDegreesLatitude(), Longitude: live.GetDegreesLongitude(), Name: live.GetCaption(),
		}), true
	case msg.GetContactMessage() != nil:
		return contactShape([]*waE2E.ContactMessage{msg.GetContactMessage()}), true
	case msg.GetContactsArrayMessage() != nil:
		return contactShape(msg.GetContactsArrayMessage().GetContacts()), true
	}
	if poll := pollCreation(msg); poll != nil {
		return pollShape(poll), true
	}
	return attachment{}, false
}

// pollCreation finds a poll whichever of WhatsApp's versions carries it.
func pollCreation(msg *waE2E.Message) *waE2E.PollCreationMessage {
	for _, poll := range []*waE2E.PollCreationMessage{
		msg.GetPollCreationMessage(), msg.GetPollCreationMessageV2(), msg.GetPollCreationMessageV3(),
		msg.GetPollCreationMessageV5(), msg.GetPollCreationMessageV6(),
	} {
		if poll != nil {
			return poll
		}
	}
	return nil
}

func locationShape(loc *waE2E.LocationMessage) attachment {
	label := loc.GetName()
	if label == "" {
		label = loc.GetAddress()
	}
	if label == "" {
		label = fmt.Sprintf("%.5f, %.5f", loc.GetDegreesLatitude(), loc.GetDegreesLongitude())
	}
	return encodeShape(models.MessageKindLocation, "Lokasi: "+label, locationExtra{
		Latitude: loc.GetDegreesLatitude(), Longitude: loc.GetDegreesLongitude(),
		Name: loc.GetName(), Address: loc.GetAddress(), URL: loc.GetURL(),
	})
}

func contactShape(cards []*waE2E.ContactMessage) attachment {
	extra := contactExtra{Contacts: []contactCard{}}
	names := make([]string, 0, len(cards))
	for _, card := range cards {
		extra.Contacts = append(extra.Contacts, contactCard{
			DisplayName: card.GetDisplayName(), Phones: vcardPhones(card.GetVcard()),
		})
		names = append(names, card.GetDisplayName())
	}
	return encodeShape(models.MessageKindContact, "Kontak: "+strings.Join(names, ", "), extra)
}

// vcardPhones reads the numbers out of a vCard. Only the TEL lines matter
// here; the parameters before the colon say what kind of number it is.
func vcardPhones(vcard string) []string {
	phones := []string{}
	for _, line := range strings.Split(vcard, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToUpper(line), "TEL") {
			continue
		}
		if _, number, ok := strings.Cut(line, ":"); ok && number != "" {
			phones = append(phones, strings.TrimSpace(number))
		}
	}
	return phones
}

func pollShape(poll *waE2E.PollCreationMessage) attachment {
	names := make([]string, 0, len(poll.GetOptions()))
	for _, option := range poll.GetOptions() {
		names = append(names, option.GetOptionName())
	}
	extra, err := services.NewPollExtra(poll.GetName(), names, int(poll.GetSelectableOptionsCount()))
	if err != nil {
		// A poll from someone's phone is stored whatever shape it has; the
		// rules NewPollExtra enforces are for polls written here.
		raw, _ := json.Marshal(services.PollExtra{Question: poll.GetName(), Options: pollOptions(names)})
		extra = string(raw)
	}
	return attachment{kind: models.MessageKindPoll, caption: "Polling: " + poll.GetName(), extra: extra}
}

func pollOptions(names []string) []services.PollOption {
	out := make([]services.PollOption, len(names))
	for i, name := range names {
		out[i] = services.PollOption{Name: name, Voters: []models.GroupActor{}}
	}
	return out
}

func encodeShape(kind models.MessageKind, summary string, extra any) attachment {
	raw, err := json.Marshal(extra)
	if err != nil {
		return attachment{kind: kind, caption: summary}
	}
	return attachment{kind: kind, caption: summary, extra: string(raw)}
}

// isForwarded says whether the sender's phone marked a message forwarded.
func isForwarded(msg *waE2E.Message) bool {
	for _, ctx := range []*waE2E.ContextInfo{
		msg.GetExtendedTextMessage().GetContextInfo(), msg.GetImageMessage().GetContextInfo(),
		msg.GetVideoMessage().GetContextInfo(), msg.GetAudioMessage().GetContextInfo(),
		msg.GetDocumentMessage().GetContextInfo(), msg.GetStickerMessage().GetContextInfo(),
		msg.GetLocationMessage().GetContextInfo(), msg.GetContactMessage().GetContextInfo(),
	} {
		if ctx.GetIsForwarded() {
			return true
		}
	}
	return false
}
