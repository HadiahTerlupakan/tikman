package wa

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"

	// Registered for their side effects: image.DecodeConfig only knows the
	// formats that are imported. WebP is left out because no decoder for it is
	// linked anywhere in the binary; a WebP goes out without dimensions, as
	// every image did before.
	_ "image/gif"
	_ "image/png"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.uber.org/zap"
	"golang.org/x/image/draw"
	"google.golang.org/protobuf/proto"
)

const (
	// mediaThumbMaxEdge is the longest side of the preview WhatsApp shows while
	// the full image downloads. It rides inside the message itself, so it is
	// kept tiny; WhatsApp's own clients send one about this size.
	mediaThumbMaxEdge = 100
	// mediaThumbQuality is low on purpose: the preview is blurred by the
	// client anyway, and every byte of it is in every copy of the message.
	mediaThumbQuality = 60
	// mediaThumbMaxPixels bounds the full decode a thumbnail needs. Bytes are
	// capped at upload, pixels are not: a flat 40000x40000 PNG is a few MB
	// on disk and 6 GB decoded, which would kill the process holding every
	// WhatsApp session. 50 MP still covers any phone camera.
	mediaThumbMaxPixels = 50_000_000
)

// errImageMetaSkipped marks an image left without part of its metadata on
// purpose, which is routine and not worth a warning.
var errImageMetaSkipped = errors.New("image metadata skipped")

// imageMeta is what a WhatsApp client needs to lay an image out before it
// has downloaded it. Without Width and Height the bubble is a square and the
// picture is cropped into it, which cut most of a wide traffic graph away.
type imageMeta struct {
	width, height uint32
	thumbnail     []byte
}

// readImageMeta measures an image and draws its preview.
func readImageMeta(data []byte) (imageMeta, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return imageMeta{}, fmt.Errorf("read image size: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return imageMeta{}, fmt.Errorf("image has no area")
	}
	if format == "jpeg" && jpegOrientation(data) > 1 {
		// A client that honours EXIF would draw the bubble at the wrong
		// aspect ratio from the stored size; without one it measures itself.
		return imageMeta{}, fmt.Errorf("%w: JPEG rotated by EXIF", errImageMetaSkipped)
	}
	meta := imageMeta{width: uint32(cfg.Width), height: uint32(cfg.Height)}
	if int64(cfg.Width)*int64(cfg.Height) > mediaThumbMaxPixels {
		return meta, fmt.Errorf("%w: %dx%d is too large to thumbnail", errImageMetaSkipped, cfg.Width, cfg.Height)
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return meta, fmt.Errorf("decode image: %w", err)
	}
	thumb, err := encodeThumbnail(src)
	if err != nil {
		return meta, err
	}
	meta.thumbnail = thumb
	return meta, nil
}

func encodeThumbnail(src image.Image) ([]byte, error) {
	w, h := thumbnailSize(src.Bounds().Dx(), src.Bounds().Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// Transparent PNG pixels would encode as black in a JPEG.
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: mediaThumbQuality}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return out.Bytes(), nil
}

// thumbnailSize scales w×h so its longest side is mediaThumbMaxEdge, never
// enlarging and never collapsing a side to zero.
func thumbnailSize(w, h int) (int, int) {
	if w <= mediaThumbMaxEdge && h <= mediaThumbMaxEdge {
		return w, h
	}
	if w >= h {
		return mediaThumbMaxEdge, max(1, h*mediaThumbMaxEdge/w)
	}
	return max(1, w*mediaThumbMaxEdge/h), mediaThumbMaxEdge
}

// mediaMessage builds the message for an uploaded attachment and, for an
// image, adds its size and preview. Failing to read the image is logged and
// the image is sent without them: it still arrives, only laid out as before.
func (c *Client) mediaMessage(
	kind models.MessageKind, up whatsmeow.UploadResponse, data []byte, mime, filename, caption string,
	info *waE2E.ContextInfo,
) *waE2E.Message {
	msg := buildMediaMessage(kind, up, mime, filename, caption, info)
	if kind != models.MessageKindImage {
		return msg
	}
	meta, err := readImageMeta(data)
	switch {
	case errors.Is(err, errImageMetaSkipped):
		c.logger.Debug("Sending image without part of its metadata", zap.String("mime", mime), zap.Error(err))
	case err != nil:
		c.logger.Warn("Sending image without size or preview", zap.String("mime", mime), zap.Error(err))
	}
	applyImageMeta(msg.GetImageMessage(), meta)
	return msg
}

func applyImageMeta(img *waE2E.ImageMessage, meta imageMeta) {
	if meta.width > 0 && meta.height > 0 {
		img.Width = proto.Uint32(meta.width)
		img.Height = proto.Uint32(meta.height)
	}
	if len(meta.thumbnail) > 0 {
		img.JPEGThumbnail = meta.thumbnail
	}
}
