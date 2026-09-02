package wa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// mediaExtensions is the whole of what a stored attachment may be called. The
// extension is looked up from the mime type WhatsApp declares, never taken from
// the sender's filename, because that name is written by the customer.
var mediaExtensions = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"video/mp4":       ".mp4",
	"video/3gpp":      ".3gp",
	"audio/ogg":       ".ogg",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"audio/amr":       ".amr",
	"application/pdf": ".pdf",
}

const (
	// unknownExtension is what an unrecognised mime type gets. Refusing the file
	// would lose it; giving it the sender's own suffix would put their text on
	// our filesystem.
	unknownExtension = ".bin"
	// defaultMime is what a document arriving without one is recorded as.
	defaultMime = "application/octet-stream"
	// maxFilenameLength matches the media_filename column. A longer name would
	// fail the insert, and losing the message over a long filename is worse than
	// showing a shortened one.
	maxFilenameLength = 255
)

// attachment is an inbound message reduced to what storing it needs. download
// is nil for a message that carries only text.
type attachment struct {
	kind     models.MessageKind
	caption  string
	mime     string
	filename string
	download whatsmeow.DownloadableMessage
}

// describe reads what a CS would see in a WhatsApp message. It answers false
// for the events that carry nothing to read — reactions, edits, protocol
// messages — so neither a thread nor an empty message is created for them.
func describe(msg *waE2E.Message) (attachment, bool) {
	switch {
	case msg.GetImageMessage() != nil:
		m := msg.GetImageMessage()
		return attachment{kind: models.MessageKindImage, caption: m.GetCaption(), mime: m.GetMimetype(), download: m}, true
	case msg.GetVideoMessage() != nil:
		m := msg.GetVideoMessage()
		return attachment{kind: models.MessageKindVideo, caption: m.GetCaption(), mime: m.GetMimetype(), download: m}, true
	case msg.GetAudioMessage() != nil:
		m := msg.GetAudioMessage()
		return attachment{kind: models.MessageKindAudio, mime: m.GetMimetype(), download: m}, true
	case msg.GetDocumentMessage() != nil:
		m := msg.GetDocumentMessage()
		return attachment{
			kind: models.MessageKindDocument, caption: m.GetCaption(),
			mime: m.GetMimetype(), filename: m.GetFileName(), download: m,
		}, true
	}
	if text := textBody(msg); text != "" {
		return attachment{kind: models.MessageKindText, caption: text}, true
	}
	return attachment{}, false
}

// textBody returns the words of a plain message. WhatsApp uses the extended
// form whenever a message quotes another or carries a link preview.
func textBody(msg *waE2E.Message) string {
	if text := msg.GetConversation(); text != "" {
		return text
	}
	return msg.GetExtendedTextMessage().GetText()
}

// mediaStore writes attachments below one directory.
type mediaStore struct {
	root string
}

// save downloads one attachment to <root>/<year>/<month>/<uuid><ext>.
//
// Every segment of that path is ours: the uuid, and an extension mapped from
// the declared mime type. The customer's own filename is kept for display and
// never becomes a path — a document called "../../etc/passwd" is a label here
// and nothing else.
func (s mediaStore) save(ctx context.Context, cli *whatsmeow.Client, att attachment) (*services.MediaFile, error) {
	mime := att.mime
	if mime == "" {
		mime = defaultMime
	}

	now := time.Now()
	rel := filepath.Join(now.Format("2006"), now.Format("01"), uuid.NewString()+extensionFor(mime))
	full := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return nil, fmt.Errorf("create media directory: %w", err)
	}

	file, err := os.OpenFile(full, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, fmt.Errorf("create media file: %w", err)
	}
	size, err := downloadInto(ctx, cli, att.download, file)
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(full)
		return nil, err
	}

	return &services.MediaFile{
		Path:     rel,
		Mime:     mime,
		Filename: clampFilename(att.filename),
		Size:     size,
	}, nil
}

func downloadInto(ctx context.Context, cli *whatsmeow.Client, msg whatsmeow.DownloadableMessage, file *os.File) (int64, error) {
	if err := cli.DownloadToFile(ctx, msg, file); err != nil {
		return 0, fmt.Errorf("download attachment: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("measure attachment: %w", err)
	}
	return info.Size(), nil
}

func extensionFor(mime string) string {
	if ext, ok := mediaExtensions[mime]; ok {
		return ext
	}
	return unknownExtension
}

// clampFilename keeps the customer's name short enough for the column. It is
// cut on a rune boundary so the stored name stays valid UTF-8.
func clampFilename(name string) string {
	if len(name) <= maxFilenameLength {
		return name
	}
	cut := name[:maxFilenameLength]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func uploadTypeFor(kind models.MessageKind) whatsmeow.MediaType {
	switch kind {
	case models.MessageKindImage:
		return whatsmeow.MediaImage
	case models.MessageKindVideo:
		return whatsmeow.MediaVideo
	case models.MessageKindAudio:
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

// buildMediaMessage wraps an uploaded attachment in the protobuf WhatsApp
// expects for its kind. A kind we have no shape for is sent as a document,
// which is the one form that carries any file.
func buildMediaMessage(kind models.MessageKind, up whatsmeow.UploadResponse, mime, filename, caption string) *waE2E.Message {
	switch kind {
	case models.MessageKindImage:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime), Caption: proto.String(caption),
		}}
	case models.MessageKindVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime), Caption: proto.String(caption),
		}}
	case models.MessageKindAudio:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime),
		}}
	default:
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime), FileName: proto.String(filename), Caption: proto.String(caption),
		}}
	}
}
