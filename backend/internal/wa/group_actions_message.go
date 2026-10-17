package wa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// MessageAdmin is the part of the connection that acts on one message in a
// group: reacting, editing, deleting for everyone, voting, and asking the
// phone for what came before.
type MessageAdmin interface {
	SelfJID() string
	React(ctx context.Context, groupJID, senderJID, waID, emoji string) error
	Revoke(ctx context.Context, groupJID, senderJID, waID string) error
	EditText(ctx context.Context, groupJID, waID, body string, mentions []string) error
	Vote(ctx context.Context, groupJID string, poll PollTarget, options []string) error
	RequestHistory(ctx context.Context, groupJID string, anchor HistoryAnchor) error
}

// PollTarget is the poll a vote answers, as WhatsApp addresses it.
type PollTarget struct {
	WAMessageID string
	SenderJID   string
	FromMe      bool
	At          time.Time
}

var errMessageNotSent = errors.New("pesan ini belum sampai di WhatsApp")

// applyToMessage carries out an action on one message and records what it
// changed, so the page shows it without waiting for WhatsApp to echo it back
// — which it does not do for what this device itself sent.
func (r *GroupActionRunner) applyToMessage(
	ctx context.Context, kind models.GroupActionKind, group *models.WAGroup, args services.GroupActionArgs,
) error {
	if kind == models.GroupActionHistory {
		return r.requestHistory(ctx, group)
	}
	id, err := uuid.Parse(args.MessageID)
	if err != nil {
		return services.ErrGroupActionUnknown
	}
	msg, err := r.messages.Get(id)
	if err != nil || msg.GroupID != group.ID {
		return fmt.Errorf("pesan tidak ditemukan di grup ini")
	}
	if msg.WAMessageID == nil {
		return errMessageNotSent
	}
	switch kind {
	case models.GroupActionReact:
		return r.react(ctx, group, msg, args.Emoji)
	case models.GroupActionRevoke:
		return r.revoke(ctx, group, msg)
	case models.GroupActionEdit:
		return r.edit(ctx, group, msg, args.Body)
	case models.GroupActionVote:
		return r.vote(ctx, group, msg, args.Options)
	default:
		return services.ErrGroupActionUnknown
	}
}

func (r *GroupActionRunner) self() models.GroupActor {
	return models.GroupActor{JID: r.admin.SelfJID(), IsSelf: true}
}

func (r *GroupActionRunner) react(ctx context.Context, group *models.WAGroup, msg *models.WAGroupMessage, emoji string) error {
	if err := r.admin.React(ctx, group.JID, msg.SenderJID, *msg.WAMessageID, emoji); err != nil {
		return err
	}
	return r.messages.SetReaction(msg.ID, r.self(), emoji)
}

func (r *GroupActionRunner) revoke(ctx context.Context, group *models.WAGroup, msg *models.WAGroupMessage) error {
	sender := msg.SenderJID
	if msg.Direction == models.MessageOut {
		sender = ""
	}
	if err := r.admin.Revoke(ctx, group.JID, sender, *msg.WAMessageID); err != nil {
		return err
	}
	path, err := r.messages.MarkDeleted(msg.ID)
	if err != nil || path == "" {
		return err
	}
	if err := os.Remove(filepath.Join(r.mediaRoot, path)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("hapus lampiran: %w", err)
	}
	return nil
}

func (r *GroupActionRunner) edit(ctx context.Context, group *models.WAGroup, msg *models.WAGroupMessage, body string) error {
	mentions := services.Mentions(msg)
	if err := r.admin.EditText(ctx, group.JID, *msg.WAMessageID, body, mentions); err != nil {
		return err
	}
	return r.messages.MarkEdited(msg.ID, body, mentions)
}

func (r *GroupActionRunner) vote(ctx context.Context, group *models.WAGroup, msg *models.WAGroupMessage, options []string) error {
	poll := PollTarget{
		WAMessageID: *msg.WAMessageID, SenderJID: msg.SenderJID,
		FromMe: msg.Direction == models.MessageOut, At: msg.WATimestamp,
	}
	if err := r.admin.Vote(ctx, group.JID, poll, options); err != nil {
		return err
	}
	return r.messages.SetVote(msg.ID, r.self(), options)
}

func (r *GroupActionRunner) requestHistory(ctx context.Context, group *models.WAGroup) error {
	oldest, err := r.messages.OldestAnchor(group.ID)
	if err != nil {
		return err
	}
	return r.admin.RequestHistory(ctx, group.JID, HistoryAnchor{
		WAMessageID: *oldest.WAMessageID, SenderJID: oldest.SenderJID,
		FromMe: oldest.Direction == models.MessageOut, At: oldest.WATimestamp,
	})
}

// SelfJID is the paired number, which is how our own reactions and votes are
// recorded.
func (c *Client) SelfJID() string {
	return c.selfJID().String()
}

// React puts an emoji on a group message, or takes ours off with "".
func (c *Client) React(ctx context.Context, groupJID, senderJID, waID, emoji string) error {
	chat, sender, err := chatAndSender(groupJID, senderJID)
	if err != nil {
		return err
	}
	_, err = c.wa.SendMessage(ctx, chat, c.wa.BuildReaction(chat, sender, types.MessageID(waID), emoji))
	return err
}

// Revoke deletes a message for everyone: ours with an empty sender, someone
// else's, as a group admin, with theirs.
func (c *Client) Revoke(ctx context.Context, groupJID, senderJID, waID string) error {
	chat, sender, err := chatAndSender(groupJID, senderJID)
	if err != nil {
		return err
	}
	_, err = c.wa.SendMessage(ctx, chat, c.wa.BuildRevoke(chat, sender, types.MessageID(waID)))
	return err
}

// EditText replaces the words of one of our messages.
func (c *Client) EditText(ctx context.Context, groupJID, waID, body string, mentions []string) error {
	chat, _, err := chatAndSender(groupJID, "")
	if err != nil {
		return err
	}
	content := &waE2E.Message{Conversation: proto.String(body)}
	if len(mentions) > 0 {
		content = &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(body), ContextInfo: withMentions(nil, mentions),
		}}
	}
	_, err = c.wa.SendMessage(ctx, chat, c.wa.BuildEdit(chat, types.MessageID(waID), content))
	return err
}

// Vote answers a poll with the options named; none withdraws the vote.
func (c *Client) Vote(ctx context.Context, groupJID string, poll PollTarget, options []string) error {
	chat, sender, err := chatAndSender(groupJID, poll.SenderJID)
	if err != nil {
		return err
	}
	info := &types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: poll.FromMe, IsGroup: true},
		ID:            types.MessageID(poll.WAMessageID),
		Timestamp:     poll.At,
	}
	vote, err := c.wa.BuildPollVote(ctx, info, options)
	if err != nil {
		return err
	}
	_, err = c.wa.SendMessage(ctx, chat, vote)
	return err
}

func chatAndSender(groupJID, senderJID string) (types.JID, types.JID, error) {
	chat, err := types.ParseJID(groupJID)
	if err != nil {
		return types.JID{}, types.JID{}, fmt.Errorf("grup tidak valid %q: %w", groupJID, err)
	}
	if senderJID == "" {
		return chat, types.EmptyJID, nil
	}
	sender, err := types.ParseJID(senderJID)
	if err != nil {
		return types.JID{}, types.JID{}, fmt.Errorf("pengirim tidak valid %q: %w", senderJID, err)
	}
	return chat, sender, nil
}
