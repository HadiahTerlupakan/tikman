package wa

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

func newsletter(jid, name string, role types.NewsletterRole) *types.NewsletterMetadata {
	meta := &types.NewsletterMetadata{
		ID:         types.JID{User: jid, Server: types.NewsletterServer},
		ViewerMeta: &types.NewsletterViewerMetadata{Role: role},
	}
	meta.ThreadMeta.Name.Text = name
	meta.ThreadMeta.SubscriberCount = 240
	return meta
}

// GetSubscribedNewsletters answers everything the number follows, and most of
// it is followed as a plain subscriber. Keeping those would fill the picker
// with channels every post to would be refused.
func TestOnlyChannelsTheNumberCanPostToAreKept(t *testing.T) {
	account := uuid.New()

	kept := channelsFrom([]*types.NewsletterMetadata{
		newsletter("120363000000000001", "Info Gangguan", types.NewsletterRoleOwner),
		newsletter("120363000000000002", "Promo", types.NewsletterRoleAdmin),
		newsletter("120363000000000003", "Berita Tetangga", types.NewsletterRoleSubscriber),
		newsletter("120363000000000004", "Kanal Tamu", types.NewsletterRoleGuest),
	}, account)

	require.Len(t, kept, 2)
	names := []string{kept[0].Name, kept[1].Name}
	assert.ElementsMatch(t, []string{"Info Gangguan", "Promo"}, names)
	assert.Equal(t, account, kept[0].WAAccountID)
}

// ViewerMeta is a pointer in whatsmeow and is absent for a newsletter the
// server told us nothing about. Reading Role off it unguarded would panic and
// take the whole sync — and with it the session's goroutine — down.
func TestANewsletterWithoutViewerMetadataIsSkipped(t *testing.T) {
	kept := channelsFrom([]*types.NewsletterMetadata{
		{ID: types.JID{User: "120363000000000005", Server: types.NewsletterServer}},
		nil,
	}, uuid.New())

	assert.Empty(t, kept)
}

// The JID is what a post is addressed to, so it must be stored in the form
// whatsmeow parses back, not just the numeric part.
func TestTheStoredJIDIsAddressable(t *testing.T) {
	kept := channelsFrom([]*types.NewsletterMetadata{
		newsletter("120363000000000001", "Info Gangguan", types.NewsletterRoleOwner),
	}, uuid.New())

	require.Len(t, kept, 1)
	assert.Equal(t, "120363000000000001@newsletter", kept[0].JID)
	assert.Equal(t, models.ChannelRoleOwner, kept[0].Role)
	assert.Equal(t, 240, kept[0].SubscriberCount)
}
