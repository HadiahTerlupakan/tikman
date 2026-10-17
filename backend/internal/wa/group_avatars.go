package wa

import (
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// trackAvatars makes sure a group's photo and its members' are on the
// sweep's list. Nothing is fetched here: the sweep paces those queries.
func (c *Client) trackAvatars(groupJID string, participants []types.GroupParticipant) {
	jids := []string{groupJID}
	for _, p := range participants {
		jids = append(jids, p.JID.ToNonAD().String())
	}
	c.track(jids)
}

func (c *Client) trackSnapshot(snap services.GroupSnapshot) {
	jids := []string{snap.JID}
	for _, member := range snap.Participants {
		if !member.IsSelf {
			jids = append(jids, member.JID)
		}
	}
	c.track(jids)
}

func (c *Client) track(jids []string) {
	if err := c.avatars.Track(c.accountID, jids); err != nil {
		c.logger.Warn("Could not list group photos for the sweep", zap.Error(err))
	}
}

// pictureChanged answers WhatsApp saying a photo changed by having the sweep
// look at it again soon, instead of fetching it on the event goroutine.
func (c *Client) pictureChanged(evt *events.Picture) {
	if err := c.avatars.Invalidate(c.accountID, evt.JID.ToNonAD().String()); err != nil {
		c.logger.Warn("Could not mark a photo for refresh", zap.Error(err))
	}
}
