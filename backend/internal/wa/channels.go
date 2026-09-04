// Reading which WhatsApp Channels a number may post to. The list is mirrored
// into the database rather than asked for on demand, because the API process
// holds no WhatsApp connection.
package wa

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

// AdminChannels answers the channels this number administers.
func (c *Client) AdminChannels(ctx context.Context) ([]models.WAChannel, error) {
	metas, err := c.wa.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, fmt.Errorf("baca daftar saluran: %w", err)
	}
	return channelsFrom(metas, c.accountID), nil
}

// channelsFrom keeps only what this number may actually post to.
//
// ViewerMeta is a pointer and is absent for a newsletter the server said
// nothing about; reading Role off it unguarded would panic inside the
// session's own goroutine.
func channelsFrom(metas []*types.NewsletterMetadata, accountID uuid.UUID) []models.WAChannel {
	out := make([]models.WAChannel, 0, len(metas))
	for _, meta := range metas {
		if meta == nil || meta.ViewerMeta == nil {
			continue
		}
		role, ok := postingRole(meta.ViewerMeta.Role)
		if !ok {
			continue
		}
		out = append(out, models.WAChannel{
			WAAccountID:     accountID,
			JID:             meta.ID.String(),
			Name:            meta.ThreadMeta.Name.Text,
			Role:            role,
			SubscriberCount: meta.ThreadMeta.SubscriberCount,
		})
	}
	return out
}

// postingRole maps the roles whatsmeow reports to the two that may post,
// answering false for a channel this number only follows.
func postingRole(role types.NewsletterRole) (models.ChannelRole, bool) {
	switch role {
	case types.NewsletterRoleOwner:
		return models.ChannelRoleOwner, true
	case types.NewsletterRoleAdmin:
		return models.ChannelRoleAdmin, true
	default:
		return "", false
	}
}
