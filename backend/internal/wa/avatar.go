package wa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// avatarDir is where profile photos live under the media root, kept apart from
// message attachments because nothing sweeps them the same way: attachments go
// with their message, a photo goes when the customer changes it.
const avatarDir = "avatars"

// avatarMimes is the whole list a profile photo may be stored and served as.
//
// Deliberately narrower than the attachment allowlist. This file is handed
// back from the API's own origin to a CS's browser, and the shapes that can
// carry script — svg above all — have no business being a face in a list.
var avatarMimes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// AvatarMime answers the type a stored profile photo is served as, empty for a
// path this inbox did not write. The extension is ours, chosen from
// avatarMimes when the file was stored, so reading it back is not trust in the
// remote server that supplied the bytes.
func AvatarMime(path string) string {
	ext := filepath.Ext(path)
	for mime, known := range avatarMimes {
		if known == ext {
			return mime
		}
	}
	return ""
}

// PictureState is what asking WhatsApp about a customer's photo turned up.
type PictureState int

const (
	// PictureNone is no photo at all, or one hidden from this number. It is by
	// far the commonest answer: most people show their photo to contacts only,
	// and a CS number is nobody's contact.
	PictureNone PictureState = iota
	// PictureUnchanged is the photo already stored, still current.
	PictureUnchanged
	// PictureNew is a photo this inbox does not hold, downloaded into Bytes.
	PictureNew
)

// Picture is a customer's profile photo as WhatsApp answered for it.
type Picture struct {
	State PictureState
	ID    string
	Mime  string
	Bytes []byte
}

// AvatarSource is the part of whatsmeow that fetching a profile photo needs.
// The bytes come back with the answer rather than a link to them: WhatsApp's
// photo URLs expire, so a link kept for later is a link that stops working.
type AvatarSource interface {
	ProfilePicture(ctx context.Context, jid, knownID string) (Picture, error)
}

// AvatarSweeper keeps customers' profile photos current.
//
// It runs on its own schedule rather than on the path that stores an incoming
// message: asking WhatsApp for a photo is a network round trip, and a
// customer's complaint must not wait behind it. It is paced for the same
// reason the outbox is — a burst of queries about a list of strangers is the
// shape of traffic that gets an unofficial number flagged.
type AvatarSweeper struct {
	conversations *services.CSConversationService
	source        AvatarSource
	root          string
	pace          time.Duration
	refreshAfter  time.Duration
}

// NewAvatarSweeper constructs an AvatarSweeper. refreshAfter is how long a
// photo already looked at is left alone before the question is put again.
func NewAvatarSweeper(
	conversations *services.CSConversationService,
	source AvatarSource,
	root string,
	pace, refreshAfter time.Duration,
) *AvatarSweeper {
	return &AvatarSweeper{
		conversations: conversations,
		source:        source,
		root:          root,
		pace:          pace,
		refreshAfter:  refreshAfter,
	}
}

// Sweep looks at up to limit conversations and answers how many photos it
// stored.
//
// One customer's failure costs that customer's face and nothing else: the
// sweep carries on, and the row is left unrecorded so the next run tries
// again. A failure is not recorded as a check because the likeliest cause is
// the connection being down, and marking everyone checked would leave the
// whole inbox faceless for a week over a blip.
func (a *AvatarSweeper) Sweep(ctx context.Context, limit int) (int, error) {
	due, err := a.conversations.StaleAvatars(limit, time.Now().Add(-a.refreshAfter))
	if err != nil {
		return 0, err
	}

	stored := 0
	for i, conv := range due {
		if i > 0 && a.pace > 0 {
			select {
			case <-ctx.Done():
				return stored, ctx.Err()
			case <-time.After(a.pace):
			}
		}

		saved, err := a.refresh(ctx, conv)
		if err != nil {
			continue
		}
		if saved {
			stored++
		}
	}
	return stored, nil
}

func (a *AvatarSweeper) refresh(ctx context.Context, conv models.CSConversation) (bool, error) {
	pic, err := a.source.ProfilePicture(ctx, conv.CustomerJID, conv.AvatarID)
	if err != nil {
		return false, err
	}

	switch pic.State {
	case PictureUnchanged:
		return false, a.conversations.SetAvatarChecked(conv.ID, time.Now())
	case PictureNone:
		return false, a.forget(conv.ID)
	}

	path, err := a.store(pic)
	if err != nil {
		return false, err
	}
	replaced, err := a.conversations.SetAvatar(conv.ID, pic.ID, path)
	if err != nil {
		// The row does not name the file, so nothing would ever collect it.
		a.remove(path)
		return false, err
	}
	a.remove(replaced)
	return true, nil
}

// forget drops a photo the customer has taken down, so the inbox stops showing
// a face they have removed.
func (a *AvatarSweeper) forget(id uuid.UUID) error {
	replaced, err := a.conversations.SetAvatar(id, "", "")
	if err != nil {
		return err
	}
	a.remove(replaced)
	return nil
}

// store writes a downloaded photo to <root>/avatars/<uuid><ext>.
//
// Every segment of that path is ours: the uuid, and an extension mapped from
// the declared type. Nothing the remote server sent becomes a path.
func (a *AvatarSweeper) store(pic Picture) (string, error) {
	ext, ok := avatarMimes[NormalizeMime(pic.Mime)]
	if !ok {
		return "", fmt.Errorf("profile photo is a %q, which is not shown as a face", pic.Mime)
	}

	rel := filepath.Join(avatarDir, uuid.NewString()+ext)
	full := filepath.Join(a.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return "", fmt.Errorf("create avatar directory: %w", err)
	}
	if err := os.WriteFile(full, pic.Bytes, 0o640); err != nil {
		return "", fmt.Errorf("write avatar: %w", err)
	}
	return rel, nil
}

// remove drops a photo that is referenced by nothing. A failure is not worth
// reporting: it costs a file, and the row it belonged to is already correct.
func (a *AvatarSweeper) remove(rel string) {
	if rel == "" {
		return
	}
	_ = os.Remove(filepath.Join(a.root, rel))
}
