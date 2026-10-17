package wa

import (
	"context"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

// SendGroupText sends a message to a group, tagging the members named.
func (c *Client) SendGroupText(
	ctx context.Context, jid, body string, quote *Quote, mentions []string, preview *linkpreview.Preview,
) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("grup tidak valid %q: %w", jid, err)
	}
	info := withMentions(buildContextInfo(quote, to, c.selfJID()), mentions)
	resp, err := c.wa.SendMessage(ctx, to, buildTextMessage(body, info, preview))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendGroupMedia sends an attachment to a group, its caption tagging the
// members named.
func (c *Client) SendGroupMedia(
	ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string,
	quote *Quote, mentions []string,
) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("grup tidak valid %q: %w", jid, err)
	}
	info := withMentions(buildContextInfo(quote, to, c.selfJID()), mentions)
	return c.sendMedia(ctx, to, kind, path, mime, filename, caption, info)
}

// withMentions adds the tagged members to a message's context. The text
// alone is not a mention: WhatsApp highlights the name and notifies the
// member only for the JIDs listed here.
func withMentions(info *waE2E.ContextInfo, mentions []string) *waE2E.ContextInfo {
	if len(mentions) == 0 {
		return info
	}
	if info == nil {
		info = &waE2E.ContextInfo{}
	}
	info.MentionedJID = mentions
	return info
}
