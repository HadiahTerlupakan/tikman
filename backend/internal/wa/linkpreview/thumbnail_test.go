package linkpreview

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func drawn(t *testing.T, w, h int, encode func(*bytes.Buffer, image.Image) error) []byte {
	t.Helper()
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, encode(&buf, src))
	return buf.Bytes()
}

// The thumbnail travels inside every copy of the message, so a large source
// image has to come down or each send carries megabytes.
func TestALargeImageIsScaledDown(t *testing.T) {
	raw := drawn(t, 1600, 1200, func(b *bytes.Buffer, m image.Image) error {
		return jpeg.Encode(b, m, nil)
	})

	out, err := thumbnail(raw)

	require.NoError(t, err)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	assert.LessOrEqual(t, cfg.Width, thumbMaxEdge)
	assert.LessOrEqual(t, cfg.Height, thumbMaxEdge)
	assert.Less(t, len(out), len(raw))
}

// Aspect ratio is kept; a squashed preview looks broken.
func TestTheShapeIsKept(t *testing.T) {
	raw := drawn(t, 800, 400, func(b *bytes.Buffer, m image.Image) error {
		return jpeg.Encode(b, m, nil)
	})

	out, err := thumbnail(raw)

	require.NoError(t, err)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, 2.0, float64(cfg.Width)/float64(cfg.Height))
}

// og:image is frequently a PNG, and WhatsApp wants JPEG in the message.
func TestAPNGBecomesJPEG(t *testing.T) {
	raw := drawn(t, 600, 600, func(b *bytes.Buffer, m image.Image) error {
		return png.Encode(b, m)
	})

	out, err := thumbnail(raw)

	require.NoError(t, err)
	_, err = jpeg.Decode(bytes.NewReader(out))
	assert.NoError(t, err, "must be decodable as JPEG")
}

// A tiny image is left at its own size rather than blown up into a blurry one.
func TestASmallImageIsNotEnlarged(t *testing.T) {
	raw := drawn(t, 80, 60, func(b *bytes.Buffer, m image.Image) error {
		return jpeg.Encode(b, m, nil)
	})

	out, err := thumbnail(raw)

	require.NoError(t, err)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	require.NoError(t, err)
	assert.Equal(t, 80, cfg.Width)
	assert.Equal(t, 60, cfg.Height)
}

// Whatever og:image points at is chosen by the CS, so it may not be an image
// at all. That must be an error the caller drops, never a panic.
func TestSomethingThatIsNotAnImageIsAnError(t *testing.T) {
	_, err := thumbnail([]byte("<html>bukan gambar</html>"))
	assert.Error(t, err)

	_, err = thumbnail(nil)
	assert.Error(t, err)
}
