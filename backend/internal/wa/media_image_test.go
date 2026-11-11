package wa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/tikman/olt-provisioning/internal/models"
	"go.mau.fi/whatsmeow"
	"go.uber.org/zap"
)

func sampleImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.RGBA{R: 200, A: 255})
	}
	return img
}

func encodedSample(t *testing.T, format string, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buf, sampleImage(w, h))
	} else {
		err = jpeg.Encode(&buf, sampleImage(w, h), nil)
	}
	if err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return buf.Bytes()
}

func imageClient() *Client {
	return &Client{logger: zap.NewNop()}
}

// A wide graph must reach WhatsApp with its real size, or the client lays it
// out as a square and crops most of it away.
func TestImageMessageCarriesSizeAndThumbnail(t *testing.T) {
	cases := []struct {
		format string
		w, h   int
	}{
		{"png", 900, 330},
		{"jpeg", 240, 600},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			data := encodedSample(t, tc.format, tc.w, tc.h)
			msg := imageClient().mediaMessage(models.MessageKindImage, whatsmeow.UploadResponse{},
				data, "image/"+tc.format, "", "cap", nil).GetImageMessage()

			if msg.GetWidth() != uint32(tc.w) || msg.GetHeight() != uint32(tc.h) {
				t.Fatalf("size = %dx%d, want %dx%d", msg.GetWidth(), msg.GetHeight(), tc.w, tc.h)
			}
			thumb, err := jpeg.Decode(bytes.NewReader(msg.GetJPEGThumbnail()))
			if err != nil {
				t.Fatalf("thumbnail is not a JPEG: %v", err)
			}
			b := thumb.Bounds()
			if max(b.Dx(), b.Dy()) > mediaThumbMaxEdge {
				t.Fatalf("thumbnail is %dx%d, longest side over %d", b.Dx(), b.Dy(), mediaThumbMaxEdge)
			}
		})
	}
}

func TestImageThatCannotBeReadIsStillSent(t *testing.T) {
	msg := imageClient().mediaMessage(models.MessageKindImage, whatsmeow.UploadResponse{URL: "u"},
		[]byte("not an image"), "image/png", "", "cap", nil).GetImageMessage()

	if msg == nil || msg.GetURL() != "u" || msg.GetCaption() != "cap" {
		t.Fatalf("message was not built: %+v", msg)
	}
	if msg.Width != nil || msg.Height != nil || msg.JPEGThumbnail != nil {
		t.Fatalf("garbage produced metadata: %+v", msg)
	}
}

func TestOnlyImagesGetImageMetadata(t *testing.T) {
	data := encodedSample(t, "png", 50, 50)
	msg := imageClient().mediaMessage(models.MessageKindDocument, whatsmeow.UploadResponse{},
		data, "image/png", "graph.png", "", nil)

	if msg.GetDocumentMessage() == nil || msg.GetDocumentMessage().JPEGThumbnail != nil {
		t.Fatalf("document changed: %+v", msg)
	}
}

func TestThumbnailSizeNeverEnlargesOrCollapses(t *testing.T) {
	cases := []struct{ w, h, ww, wh int }{
		{40, 30, 40, 30},
		{900, 330, 100, 36},
		{330, 900, 36, 100},
		{5000, 10, 100, 1},
	}
	for _, tc := range cases {
		w, h := thumbnailSize(tc.w, tc.h)
		if w != tc.ww || h != tc.wh {
			t.Errorf("thumbnailSize(%d,%d) = %d,%d, want %d,%d", tc.w, tc.h, w, h, tc.ww, tc.wh)
		}
	}
}

// pngHeader is a PNG that declares w x h and holds no pixels: enough for
// DecodeConfig, and cheap however large the size it claims.
func pngHeader(w, h uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, uint32(len(ihdr)))
	out = append(out, chunk...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

// Decoding a 40000x40000 image to thumbnail it would allocate gigabytes in
// the process that holds every WhatsApp session.
func TestHugeImageKeepsItsSizeWithoutBeingDecoded(t *testing.T) {
	meta, err := readImageMeta(pngHeader(40000, 40000))

	if !errors.Is(err, errImageMetaSkipped) {
		t.Fatalf("err = %v, want the decode skipped", err)
	}
	if meta.width != 40000 || meta.height != 40000 || meta.thumbnail != nil {
		t.Fatalf("meta = %dx%d thumb %d bytes", meta.width, meta.height, len(meta.thumbnail))
	}
}

// withOrientation puts an EXIF APP1 segment carrying orientation right
// after the SOI of a JPEG, in big-endian TIFF order as phones write it.
func withOrientation(jpg []byte, orientation uint16) []byte {
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01")
	tiff = binary.BigEndian.AppendUint16(tiff, exifOrientationID)
	tiff = append(tiff, 0, 3, 0, 0, 0, 1) // SHORT, count 1
	tiff = binary.BigEndian.AppendUint16(tiff, orientation)
	tiff = append(tiff, 0, 0, 0, 0, 0, 0) // value padding, no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := binary.BigEndian.AppendUint16([]byte{0xFF, jpegMarkerAPP1}, uint16(len(payload)+2))
	seg = append(seg, payload...)
	return append(append(append([]byte{}, jpg[:2]...), seg...), jpg[2:]...)
}

// A portrait phone photo is stored landscape with an Orientation tag; its
// stored size would lay the bubble out sideways.
func TestRotatedJPEGLeavesLayoutToTheClient(t *testing.T) {
	jpg := encodedSample(t, "jpeg", 400, 300)
	cases := []struct {
		orientation uint16
		wantSize    bool
	}{{1, true}, {6, false}, {8, false}}
	for _, tc := range cases {
		msg := imageClient().mediaMessage(models.MessageKindImage, whatsmeow.UploadResponse{},
			withOrientation(jpg, tc.orientation), "image/jpeg", "", "", nil).GetImageMessage()
		if got := msg.Width != nil; got != tc.wantSize {
			t.Errorf("orientation %d: size set = %v, want %v", tc.orientation, got, tc.wantSize)
		}
		if got := msg.JPEGThumbnail != nil; got != tc.wantSize {
			t.Errorf("orientation %d: thumbnail set = %v, want %v", tc.orientation, got, tc.wantSize)
		}
	}
}
