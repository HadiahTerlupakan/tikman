package wa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/services"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	// defaultMime is what an attachment gets when the declared type is missing
	// or too long to keep.
	defaultMime = "application/octet-stream"
	// maxFilenameLength matches the media_filename column. A longer name would
	// fail the insert, and losing the message over a long filename is worse than
	// showing a shortened one.
	maxFilenameLength = 255
	// maxMimeLength matches the media_mime column, for the same reason.
	maxMimeLength = 100
	// shapeFieldLimit is how many populated fields are named when logging a
	// message we cannot store. One is the answer almost always; the cap is there
	// so a crafted message cannot write a long log line.
	shapeFieldLimit = 3
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

// messageShape names the fields a WhatsApp message actually carries, so one we
// cannot store — a sticker, a location, a contact card, a poll — is logged as
// itself instead of vanishing.
func messageShape(msg *waE2E.Message) string {
	var names []string
	msg.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		names = append(names, string(fd.Name()))
		return len(names) < shapeFieldLimit
	})
	if len(names) == 0 {
		return "empty"
	}
	return strings.Join(names, ",")
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
	rel := relPath(time.Now(), att)
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
		Mime:     NormalizeMime(att.mime),
		Filename: ClampFilename(att.filename),
		Size:     size,
	}, nil
}

// remove drops a stored attachment that ended up belonging to no message row.
func (s mediaStore) remove(rel string) error {
	return os.Remove(filepath.Join(s.root, rel))
}

// relPath builds the path one attachment is stored at, relative to the media
// root: <year>/<month>/<uuid><ext>. It reads only the declared mime type. The
// customer's filename is deliberately not an input — it is display text, and a
// document called "../../etc/passwd" must not be able to become a path.
func relPath(now time.Time, att attachment) string {
	name := uuid.NewString() + ExtensionFor(NormalizeMime(att.mime))
	return filepath.Join(now.Format("2006"), now.Format("01"), name)
}

// NormalizeMime reduces what a sender declares to something the column holds
// and the extension table can match. Voice notes arrive as
// "audio/ogg; codecs=opus", so the parameters have to go before the lookup; and
// the value is written by the sender, so an over-long one would fail the insert
// and cost the whole message rather than just the picture.
func NormalizeMime(declared string) string {
	mime := declared
	if cut := strings.IndexByte(mime, ';'); cut >= 0 {
		mime = mime[:cut]
	}
	mime = strings.TrimSpace(mime)

	if mime == "" || len(mime) > maxMimeLength {
		return defaultMime
	}
	return mime
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

// ExtensionFor answers the file extension a mime type is stored under. An
// unrecognised type gets .bin: an inbound WhatsApp message the customer
// already sent must still be stored somewhere, so this never refuses one.
// Something that must refuse an unrecognised type instead — an outbound
// upload, which arrived by nobody's obligation — wants AllowedExtension.
func ExtensionFor(mime string) string {
	if ext, ok := mediaExtensions[mime]; ok {
		return ext
	}
	return unknownExtension
}

// AllowedExtension reports the extension for a mime type only when it is on
// the allowlist, and answers false otherwise — unlike ExtensionFor, which
// always answers something so an inbound message is never lost over its type.
// A caller that must refuse an unrecognised type (an outbound upload) needs
// that false, not a fallback it would otherwise trust as legitimate.
func AllowedExtension(mime string) (string, bool) {
	ext, ok := mediaExtensions[mime]
	return ext, ok
}

// ClampFilename keeps a display filename short enough for the media_filename
// column (varchar(255)) on both the inbound and outbound paths — a Postgres
// insert over that length fails outright, and a filename is attacker-supplied
// on either side (a customer's client, or a CS's own upload) so neither side
// gets to skip this. It cuts on a rune boundary so the stored name stays
// valid UTF-8.
func ClampFilename(name string) string {
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
func buildMediaMessage(
	kind models.MessageKind, up whatsmeow.UploadResponse, mime, filename, caption string,
	quoted *waE2E.ContextInfo,
) *waE2E.Message {
	switch kind {
	case models.MessageKindImage:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime), Caption: proto.String(caption),
			ContextInfo: quoted,
		}}
	case models.MessageKindVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength: proto.Uint64(up.FileLength),
			Mimetype:   proto.String(mime), Caption: proto.String(caption),
			ContextInfo: quoted,
		}}
	case models.MessageKindAudio:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: proto.String(up.URL), DirectPath: proto.String(up.DirectPath),
			MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256,
			FileLength:  proto.Uint64(up.FileLength),
			Mimetype:    proto.String(mime),
			ContextInfo: quoted,
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
