package wa

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// avatarMaxBytes caps what is read from WhatsApp's photo host. The URL is
// theirs, but the response is data from a server we do not run, and a profile
// photo that does not fit in a megabyte is not a profile photo.
const avatarMaxBytes = 1 << 20

// ProfilePicture answers a customer's current profile photo, downloaded, or
// says why there is nothing to store.
//
// knownID is the id of the photo already held. WhatsApp answers "still that
// one" to it without sending the image, which is what keeps a weekly refresh
// from re-downloading every face in the inbox.
//
// A hidden photo and a missing one are ordinary answers, not failures: most
// people show their photo to their contacts only, and a CS number is nobody's
// contact.
func (c *Client) ProfilePicture(ctx context.Context, jid, knownID string) (Picture, error) {
	to, err := types.ParseJID(jid)
	if err != nil {
		return Picture{}, fmt.Errorf("nomor tidak valid %q: %w", jid, err)
	}

	// Preview, not the full image: this is drawn at forty pixels in a list.
	info, err := c.wa.GetProfilePictureInfo(ctx, to, &whatsmeow.GetProfilePictureParams{
		Preview:    true,
		ExistingID: knownID,
	})
	switch {
	case errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized),
		errors.Is(err, whatsmeow.ErrProfilePictureNotSet):
		return Picture{State: PictureNone}, nil
	case err != nil:
		return Picture{}, err
	}
	if info == nil {
		// whatsmeow answers nil only against an id it was given, and it means
		// that id is current. With no id offered there is nothing to confirm.
		if knownID != "" {
			return Picture{State: PictureUnchanged}, nil
		}
		return Picture{State: PictureNone}, nil
	}

	body, mime, err := fetchAvatar(ctx, info.URL)
	if err != nil {
		return Picture{}, err
	}
	return Picture{State: PictureNew, ID: info.ID, Mime: mime, Bytes: body}, nil
}

// fetchAvatar downloads one profile photo, refusing to read more than
// avatarMaxBytes however much the server offers.
func fetchAvatar(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build avatar request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch avatar: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch avatar: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, avatarMaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read avatar: %w", err)
	}
	return body, resp.Header.Get("Content-Type"), nil
}
