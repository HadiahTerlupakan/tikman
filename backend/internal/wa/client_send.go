package wa

import (
	"context"
	"fmt"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"os"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

// SendText sends one reply and answers with the id WhatsApp gave it.
func (c *Client) SendText(ctx context.Context, jid, body string, quote *Quote, preview *linkpreview.Preview) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("tujuan tidak valid %q: %w", jid, err)
	}

	resp, err := c.wa.SendMessage(ctx, to, buildTextMessage(body, buildContextInfo(quote, to, c.selfJID()), preview))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendMedia uploads an attachment and sends it. The file is read whole, which
// is only safe because the upload boundary caps it: SendMedia in the API
// wraps the request body in a MaxBytesReader before a byte is stored.
func (c *Client) SendMedia(
	ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string, quote *Quote,
) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("tujuan tidak valid %q: %w", jid, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca lampiran: %w", err)
	}
	uploaded, err := c.wa.Upload(ctx, data, uploadTypeFor(kind))
	if err != nil {
		return "", fmt.Errorf("unggah lampiran: %w", err)
	}

	quoted := buildContextInfo(quote, to, c.selfJID())
	resp, err := c.wa.SendMessage(ctx, to, buildMediaMessage(kind, uploaded, mime, filename, caption, quoted))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SetTyping shows or clears the "typing…" line on the customer's phone.
func (c *Client) SetTyping(ctx context.Context, chatJID string, typing bool) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("tujuan tidak valid %q: %w", chatJID, err)
	}
	state := types.ChatPresencePaused
	if typing {
		state = types.ChatPresenceComposing
	}
	return c.wa.SendChatPresence(ctx, chat, state, types.ChatPresenceMediaText)
}

// MarkRead sends a read receipt for the customer's messages, which is what
// turns their ticks blue.
//
// The sender JID is the chat JID: whatsmeow only reads it for group chats, and
// this inbox stores none. Whether the customer actually sees the blue ticks is
// not ours to decide — a number whose WhatsApp privacy settings have read
// receipts off sends a receipt only to its own devices, and whatsmeow quietly
// downgrades the type for us.
func (c *Client) MarkRead(ctx context.Context, chatJID string, ids []string, at time.Time) error {
	chat, err := types.ParseJID(chatJID)
	if err != nil {
		return fmt.Errorf("tujuan tidak valid %q: %w", chatJID, err)
	}
	stanzas := make([]types.MessageID, 0, len(ids))
	for _, id := range ids {
		stanzas = append(stanzas, types.MessageID(id))
	}
	return c.wa.MarkRead(ctx, stanzas, at, chat, chat)
}

// selfJID is this inbox's own number, needed to quote its own replies. It is
// the zero JID before pairing finishes, which buildContextInfo treats as
// "no participant" rather than naming an empty address.
func (c *Client) selfJID() types.JID {
	if c.wa.Store.ID == nil {
		return types.JID{}
	}
	return c.wa.Store.ID.ToNonAD()
}
