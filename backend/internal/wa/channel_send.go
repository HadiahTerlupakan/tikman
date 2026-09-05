package wa

import (
	"context"
	"fmt"
	"github.com/tikman/olt-provisioning/internal/wa/linkpreview"
	"os"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// SendChannelText posts a text update and answers the id WhatsApp gave it.
func (c *Client) SendChannelText(ctx context.Context, channelJID, body string) (string, error) {
	to, err := types.ParseJID(channelJID)
	if err != nil {
		return "", fmt.Errorf("saluran tidak valid %q: %w", channelJID, err)
	}

	resp, err := c.wa.SendMessage(ctx, to, buildTextMessage(body, nil, linkpreview.Resolve(ctx, body)))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendChannelMedia posts an attachment as an update.
//
// Channel media is uploaded unencrypted, so UploadNewsletter is the upload and
// the handle it returns has to travel with the send — whatsmeow requires it
// for newsletter media. The message itself is built by the same helper the
// chat path uses: it copies MediaKey and FileEncSHA256 from the response, and
// their being empty here is exactly right for a channel.
//
// The file is read whole, which is only safe because the upload boundary caps
// it: the API wraps the request body in a MaxBytesReader before a byte is
// stored.
func (c *Client) SendChannelMedia(
	ctx context.Context, channelJID string, kind models.MessageKind, path, mime, filename, caption string,
) (string, error) {
	to, err := types.ParseJID(channelJID)
	if err != nil {
		return "", fmt.Errorf("saluran tidak valid %q: %w", channelJID, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca lampiran: %w", err)
	}
	uploaded, err := c.wa.UploadNewsletter(ctx, data, uploadTypeFor(kind))
	if err != nil {
		return "", fmt.Errorf("unggah lampiran: %w", err)
	}

	resp, err := c.wa.SendMessage(ctx, to,
		buildMediaMessage(kind, uploaded, mime, filename, caption, nil),
		whatsmeow.SendRequestExtra{MediaHandle: uploaded.Handle},
	)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
