package wa

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
)

// fakePictures answers for each customer JID in turn, so a sweep over several
// conversations can be given a different outcome for each.
type fakePictures struct {
	answers map[string]Picture
	err     map[string]error
	asked   []string
	knownID map[string]string
}

func (f *fakePictures) ProfilePicture(_ context.Context, jid, knownID string) (Picture, error) {
	f.asked = append(f.asked, jid)
	if f.knownID == nil {
		f.knownID = map[string]string{}
	}
	f.knownID[jid] = knownID
	if err, ok := f.err[jid]; ok {
		return Picture{}, err
	}
	return f.answers[jid], nil
}

func avatarSweepSetup(t *testing.T) (*services.CSConversationService, *models.CSConversation, string) {
	t.Helper()
	_, _, conversations, conv := drainSetup(t)
	return conversations, conv, t.TempDir()
}

func TestSweepStoresADownloadedPhotoAndPointsTheThreadAtIt(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {State: PictureNew, ID: "PIC1", Mime: "image/jpeg", Bytes: []byte("jpegbytes")},
	}}

	stored, err := NewAvatarSweeper(conversations, source, root, 0, time.Hour).
		Sweep(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, stored)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	require.NotEmpty(t, after.AvatarPath)
	assert.Equal(t, "PIC1", after.AvatarID)
	assert.True(t, after.HasAvatar)

	written, err := os.ReadFile(filepath.Join(root, after.AvatarPath))
	require.NoError(t, err)
	assert.Equal(t, []byte("jpegbytes"), written)
	assert.Equal(t, avatarDir, filepath.Dir(after.AvatarPath))
}

// Most customers hide their photo from a number that is not in their contacts.
// The point of recording the attempt is that the next sweep moves on to
// somebody else instead of asking about them again.
func TestSweepRecordsTheAttemptWhenThereIsNoPhoto(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {State: PictureNone},
	}}
	sweeper := NewAvatarSweeper(conversations, source, root, 0, time.Hour)

	stored, err := sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, stored)

	_, err = sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, source.asked, 1, "a customer just asked about must not be asked about again")
}

// The stored id goes back to WhatsApp so it can answer "still that one" without
// sending the image. Without it every refresh re-downloads every face.
func TestSweepOffersTheStoredIDAndKeepsThePhotoWhenNothingChanged(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)
	_, err := conversations.SetAvatar(conv.ID, "PIC1", "avatars/kept.jpg")
	require.NoError(t, err)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {State: PictureUnchanged},
	}}
	_, err = NewAvatarSweeper(conversations, source, root, 0, 0).Sweep(context.Background(), 10)
	require.NoError(t, err)

	assert.Equal(t, "PIC1", source.knownID[conv.CustomerJID])

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Equal(t, "avatars/kept.jpg", after.AvatarPath)
}

// A customer taking their photo down has to reach the inbox, or it keeps
// showing a face they have removed. The file goes with it: nothing else
// collects avatars, since retention sweeps from message rows.
func TestSweepForgetsAPhotoTheCustomerHasTakenDown(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {State: PictureNew, ID: "PIC1", Mime: "image/jpeg", Bytes: []byte("face")},
	}}
	sweeper := NewAvatarSweeper(conversations, source, root, 0, 0)
	_, err := sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)

	before, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	gone := filepath.Join(root, before.AvatarPath)
	require.FileExists(t, gone)

	source.answers[conv.CustomerJID] = Picture{State: PictureNone}
	_, err = sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Empty(t, after.AvatarPath)
	assert.False(t, after.HasAvatar)
	assert.NoFileExists(t, gone, "the file is referenced by nothing and nothing else collects it")
}

// A new photo replaces an old one, and the old file is left named by nothing.
func TestSweepRemovesThePhotoItReplaced(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {State: PictureNew, ID: "PIC1", Mime: "image/jpeg", Bytes: []byte("old")},
	}}
	sweeper := NewAvatarSweeper(conversations, source, root, 0, 0)
	_, err := sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)

	first, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	oldFile := filepath.Join(root, first.AvatarPath)

	source.answers[conv.CustomerJID] = Picture{State: PictureNew, ID: "PIC2", Mime: "image/jpeg", Bytes: []byte("new")}
	_, err = sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)

	second, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.NotEqual(t, first.AvatarPath, second.AvatarPath)
	assert.NoFileExists(t, oldFile)
}

// An SVG is an image the way a script is a document. This file is handed back
// to a CS's browser from the API's own origin, so the narrow list is the whole
// defence — and it must not be possible to get round it by declaring a type.
func TestSweepRefusesAPhotoThatIsNotAPlainImage(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{answers: map[string]Picture{
		conv.CustomerJID: {
			State: PictureNew, ID: "PIC1",
			Mime:  "image/svg+xml",
			Bytes: []byte("<svg onload='alert(1)'/>"),
		},
	}}
	stored, err := NewAvatarSweeper(conversations, source, root, 0, 0).Sweep(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 0, stored)

	after, err := conversations.Get(conv.ID)
	require.NoError(t, err)
	assert.Empty(t, after.AvatarPath)
}

// One customer's failure costs that customer's face and nothing else.
func TestSweepCarriesOnPastACustomerItCouldNotAskAbout(t *testing.T) {
	conversations, first, root := avatarSweepSetup(t)
	second, err := conversations.FindOrCreate(services.IncomingPeer{
		WAAccountID: first.WAAccountID, JID: "628999@s.whatsapp.net",
		Phone: "628999888777", Name: "Siti",
	})
	require.NoError(t, err)

	source := &fakePictures{
		err: map[string]error{first.CustomerJID: errors.New("koneksi putus")},
		answers: map[string]Picture{
			second.CustomerJID: {State: PictureNew, ID: "PIC2", Mime: "image/jpeg", Bytes: []byte("face")},
		},
	}
	stored, err := NewAvatarSweeper(conversations, source, root, 0, 0).Sweep(context.Background(), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, stored)

	after, err := conversations.Get(second.ID)
	require.NoError(t, err)
	assert.True(t, after.HasAvatar)
}

// A failure is likeliest to be the connection, not the customer. Recording it
// as a check would leave the whole inbox faceless for a week over a blip.
func TestSweepLeavesAFailedCustomerDueAgain(t *testing.T) {
	conversations, conv, root := avatarSweepSetup(t)

	source := &fakePictures{err: map[string]error{conv.CustomerJID: errors.New("koneksi putus")}}
	sweeper := NewAvatarSweeper(conversations, source, root, 0, time.Hour)

	_, err := sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)
	_, err = sweeper.Sweep(context.Background(), 10)
	require.NoError(t, err)

	assert.Len(t, source.asked, 2, "the next sweep must try again")
}
