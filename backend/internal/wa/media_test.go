package wa

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// The path an attachment is stored at is built from the declared mime type and
// nothing else. This asserts on the builder's output rather than on the
// filesystem, because a `save` that used the sender's filename would still pass
// a filesystem check: it deletes the file again when the download fails.
func TestRelPathIgnoresTheSenderFilename(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	att := attachment{
		kind:     models.MessageKindDocument,
		mime:     "audio/ogg; codecs=opus",
		filename: "../../../etc/passwd",
	}

	got := relPath(now, att)

	segments := strings.Split(got, string(filepath.Separator))
	if len(segments) != 3 {
		t.Fatalf("relPath = %q, want <year>/<month>/<name>", got)
	}
	if segments[0] != "2026" || segments[1] != "03" {
		t.Fatalf("relPath = %q, want it filed under 2026/03", got)
	}
	for _, segment := range segments {
		if segment == ".." || segment == "." || segment == "" {
			t.Fatalf("relPath = %q contains a traversal segment", got)
		}
	}
	for _, leaked := range []string{"passwd", "etc", ".."} {
		if strings.Contains(got, leaked) {
			t.Fatalf("relPath = %q leaked %q from the sender's filename", got, leaked)
		}
	}
	if ext := filepath.Ext(got); ext != ".ogg" {
		t.Fatalf("extension = %q, want .ogg from the mime type", ext)
	}
}

// A download that fails must leave nothing behind: the file is opened before the
// bytes arrive, and an orphan under the media root is never collected.
func TestSaveRemovesTheFileWhenTheDownloadFails(t *testing.T) {
	root := t.TempDir()
	att := attachment{
		kind:     models.MessageKindDocument,
		mime:     "application/pdf",
		download: &waE2E.DocumentMessage{},
	}

	// A nil client refuses the download, which is the failure path.
	if _, err := (mediaStore{root: root}).save(context.Background(), nil, att); err == nil {
		t.Fatal("expected the download to fail")
	}

	if left := walkFiles(t, root); len(left) != 0 {
		t.Fatalf("a failed download left files behind: %v", left)
	}
}

func TestRemoveDropsAnUnreferencedAttachment(t *testing.T) {
	root := t.TempDir()
	store := mediaStore{root: root}
	rel := filepath.Join("2026", "03", "orphan.pdf")

	if err := os.MkdirAll(filepath.Join(root, "2026", "03"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := store.remove(rel); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if left := walkFiles(t, root); len(left) != 0 {
		t.Fatalf("remove left files behind: %v", left)
	}
}

func TestNormalizeMimeStripsParametersAndOverLongValues(t *testing.T) {
	if got := normalizeMime("audio/ogg; codecs=opus"); got != "audio/ogg" {
		t.Fatalf("voice note mime = %q", got)
	}
	if got := extensionFor(normalizeMime("audio/ogg; codecs=opus")); got != ".ogg" {
		t.Fatalf("voice note extension = %q, want .ogg", got)
	}
	if got := normalizeMime(""); got != defaultMime {
		t.Fatalf("empty mime = %q", got)
	}

	hostile := "image/" + strings.Repeat("a", maxMimeLength)
	got := normalizeMime(hostile)
	if len(got) > maxMimeLength {
		t.Fatalf("mime = %d bytes, media_mime holds %d", len(got), maxMimeLength)
	}
	if got != defaultMime {
		t.Fatalf("over-long mime = %q, want the default", got)
	}
}

func TestExtensionComesFromMimeNotFilename(t *testing.T) {
	if got := extensionFor("image/jpeg"); got != ".jpg" {
		t.Fatalf("image/jpeg = %q", got)
	}
	if got := extensionFor("application/x-sh"); got != unknownExtension {
		t.Fatalf("unknown mime = %q, want %q", got, unknownExtension)
	}
}

func TestClampFilenameStaysValidUTF8(t *testing.T) {
	name := strings.Repeat("é", 200) // 400 bytes
	got := clampFilename(name)

	if len(got) > maxFilenameLength {
		t.Fatalf("clamped to %d bytes", len(got))
	}
	if !strings.HasPrefix(name, got) {
		t.Fatal("clamped name is not a prefix of the original")
	}
	if !utf8.ValidString(got) {
		t.Fatal("clamped name was cut mid-rune")
	}
}

func TestDescribeSkipsWhatNobodyReads(t *testing.T) {
	if _, ok := describe(&waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{}}); ok {
		t.Fatal("a reaction should not become a message")
	}

	att, ok := describe(&waE2E.Message{Conversation: proto.String("halo")})
	if !ok || att.kind != models.MessageKindText || att.caption != "halo" {
		t.Fatalf("plain text = %+v, ok=%v", att, ok)
	}

	att, ok = describe(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		Caption:  proto.String("ini rusak"),
		Mimetype: proto.String("image/jpeg"),
	}})
	if !ok || att.kind != models.MessageKindImage || att.download == nil {
		t.Fatalf("image = %+v, ok=%v", att, ok)
	}
}

// A message we cannot store must be nameable in the log, or a CS never learns
// the customer sent anything.
func TestMessageShapeNamesWhatArrived(t *testing.T) {
	got := messageShape(&waE2E.Message{StickerMessage: &waE2E.StickerMessage{}})
	if !strings.Contains(got, "stickerMessage") {
		t.Fatalf("shape = %q, want it to name the sticker", got)
	}
	if got := messageShape(&waE2E.Message{}); got != "empty" {
		t.Fatalf("empty message shape = %q", got)
	}
}

func walkFiles(t *testing.T, root string) []string {
	t.Helper()

	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}
