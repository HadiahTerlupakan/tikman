package wa

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// A hostile filename is display text, not a path. Nothing outside the media
// root may be created, and the stored name must not steer the extension either.
func TestSaveKeepsHostileFilenameOffTheFilesystem(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "escaped.txt")

	att := attachment{
		kind:     models.MessageKindDocument,
		mime:     "application/pdf",
		filename: "../../escaped.txt",
		download: &waE2E.DocumentMessage{},
	}

	// A nil client refuses the download, which is the failure path: the file it
	// had opened must be gone again.
	if _, err := (mediaStore{root: root}).save(context.Background(), nil, att); err == nil {
		t.Fatal("expected the download to fail")
	}

	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("the sender's filename became a path: %v", err)
	}
	if left := walkFiles(t, root); len(left) != 0 {
		t.Fatalf("a failed download left files behind: %v", left)
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
	if strings.ContainsRune(got, '�') {
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
