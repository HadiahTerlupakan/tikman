package wa

import (
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Quote is the message a reply answers, as much of it as WhatsApp needs to
// draw the grey block above the reply on the customer's phone.
//
// FromMe rather than a JID because only the process holding the connection
// knows its own number, and getting the participant wrong is not a cosmetic
// failure: WhatsApp uses it to find the quoted message, and an unresolvable
// one arrives as an empty box.
type Quote struct {
	StanzaID string
	FromMe   bool
	Body     string
	Kind     models.MessageKind
}

// buildContextInfo turns a quote into the field WhatsApp reads it from. chat is
// the customer, self is this inbox's own number.
func buildContextInfo(q *Quote, chat, self types.JID) *waE2E.ContextInfo {
	if q == nil {
		return nil
	}
	participant := chat.ToNonAD()
	if q.FromMe {
		participant = self.ToNonAD()
	}
	ctx := &waE2E.ContextInfo{
		StanzaID:      proto.String(q.StanzaID),
		QuotedMessage: quotedMessageFor(q),
	}
	// Only when there is a number to name. Sending "@s.whatsapp.net" as the
	// participant is worse than sending none: it names nobody, out loud.
	if participant.User != "" {
		ctx.Participant = proto.String(participant.String())
	}
	return ctx
}

// buildTextMessage picks the shape a reply has to take. A plain Conversation
// has nowhere to carry a quote, so one that quotes something is sent in the
// extended form WhatsApp itself uses for exactly that.
func buildTextMessage(body string, ctx *waE2E.ContextInfo) *waE2E.Message {
	if ctx == nil {
		return &waE2E.Message{Conversation: proto.String(body)}
	}
	return &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String(body), ContextInfo: ctx,
	}}
}

// quotedMessageFor rebuilds enough of the quoted message for the grey block to
// say something.
//
// The original protobuf is not kept — only what a CS reads — so a quoted
// attachment is rebuilt as its own shape carrying the caption but no media
// keys. WhatsApp draws that as "Photo" or "Video" with the caption beside it
// rather than the thumbnail; rebuilding the thumbnail would mean storing the
// encrypted original, which is a lot of disk for a 40-pixel square.
func quotedMessageFor(q *Quote) *waE2E.Message {
	switch q.Kind {
	case models.MessageKindImage:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String(q.Body)}}
	case models.MessageKindVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{Caption: proto.String(q.Body)}}
	case models.MessageKindAudio:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{}}
	case models.MessageKindDocument:
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: proto.String(q.Body)}}
	default:
		return &waE2E.Message{Conversation: proto.String(q.Body)}
	}
}

// quotedStanzaID answers the WhatsApp id an incoming message quotes, empty when
// it quotes nothing. Every shape this inbox stores carries its own copy of the
// context, so each has to be asked.
func quotedStanzaID(msg *waE2E.Message) string {
	for _, ctx := range []*waE2E.ContextInfo{
		msg.GetExtendedTextMessage().GetContextInfo(),
		msg.GetImageMessage().GetContextInfo(),
		msg.GetVideoMessage().GetContextInfo(),
		msg.GetAudioMessage().GetContextInfo(),
		msg.GetDocumentMessage().GetContextInfo(),
	} {
		if id := ctx.GetStanzaID(); id != "" {
			return id
		}
	}
	return ""
}
