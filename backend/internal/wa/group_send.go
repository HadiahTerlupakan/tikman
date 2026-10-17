package wa

import (
	"context"
	"fmt"

	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// GroupSend is what a group message carries besides its content.
type GroupSend struct {
	Quote     *Quote
	Mentions  []string
	Forwarded bool
}

// SendGroupText sends a message to a group.
func (c *Client) SendGroupText(
	ctx context.Context, jid, body string, opts GroupSend, preview *linkpreview.Preview,
) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("grup tidak valid %q: %w", jid, err)
	}
	resp, err := c.wa.SendMessage(ctx, to, buildTextMessage(body, c.groupContext(to, opts), preview))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendGroupMedia sends an attachment to a group.
func (c *Client) SendGroupMedia(
	ctx context.Context, jid string, kind models.MessageKind, path, mime, filename, caption string, opts GroupSend,
) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("grup tidak valid %q: %w", jid, err)
	}
	return c.sendMedia(ctx, to, kind, path, mime, filename, caption, c.groupContext(to, opts))
}

// SendGroupPoll posts a poll. whatsmeow gives it the secret that votes are
// encrypted with and keeps it, so the votes that come back can be read.
func (c *Client) SendGroupPoll(ctx context.Context, jid, question string, options []string, selectable int) (string, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("grup tidak valid %q: %w", jid, err)
	}
	resp, err := c.wa.SendMessage(ctx, to, c.wa.BuildPollCreation(question, options, selectable))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) groupContext(to types.JID, opts GroupSend) *waE2E.ContextInfo {
	info := withMentions(buildContextInfo(opts.Quote, to, c.selfJID()), opts.Mentions)
	if !opts.Forwarded {
		return info
	}
	if info == nil {
		info = &waE2E.ContextInfo{}
	}
	info.IsForwarded = proto.Bool(true)
	info.ForwardingScore = proto.Uint32(1)
	return info
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
