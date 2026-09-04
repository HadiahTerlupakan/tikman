// Posting a WhatsApp Status. Unlike a channel update, a status is encrypted
// per recipient, and whatsmeow resolves those recipients itself from the
// account's status-privacy setting and its contact store — none of that is
// TikMan's to compute or store.
package wa

import (
	"context"
	"fmt"
	"os"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/types"
)

// SendStatusText posts a text status and answers the id WhatsApp gave it.
func (c *Client) SendStatusText(ctx context.Context, body string) (string, error) {
	resp, err := c.wa.SendMessage(ctx, types.StatusBroadcastJID, buildTextMessage(body, nil))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

// SendStatusMedia posts an attachment as a status.
//
// The upload is the ordinary encrypted one the chat path uses, not
// UploadNewsletter: only channel media travels unencrypted. There is no media
// handle to pass on for the same reason.
//
// The file is read whole, which is only safe because the upload boundary caps
// it: the API wraps the request body in a MaxBytesReader before a byte is
// stored.
func (c *Client) SendStatusMedia(
	ctx context.Context, kind models.MessageKind, path, mime, filename, caption string,
) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca lampiran: %w", err)
	}
	uploaded, err := c.wa.Upload(ctx, data, uploadTypeFor(kind))
	if err != nil {
		return "", fmt.Errorf("unggah lampiran: %w", err)
	}

	resp, err := c.wa.SendMessage(ctx, types.StatusBroadcastJID,
		buildMediaMessage(kind, uploaded, mime, filename, caption, nil))
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}
