package linkpreview

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	// Registered for their side effects: og:image is as often a PNG or GIF as
	// a JPEG, and image.Decode only knows the formats that are imported.
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
)

const (
	// thumbMaxEdge is the longest side the thumbnail may have. It rides inside
	// every copy of the message, so this is sized for a preview card rather
	// than for looking at.
	thumbMaxEdge = 320
	// thumbQuality trades a little sharpness for a much smaller message.
	thumbQuality = 70
)

// thumbnail scales an image down and re-encodes it as the JPEG WhatsApp
// carries in the message.
//
// The bytes come from an address the CS chose, so "not an image" is an
// ordinary outcome rather than an exceptional one: it returns an error the
// caller drops, and the message goes out without a card.
func thumbnail(raw []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("image has no area")
	}

	// Only ever shrink. Enlarging a small favicon produces a blurry card that
	// looks worse than the crisp small one.
	if w > thumbMaxEdge || h > thumbMaxEdge {
		if w >= h {
			h = h * thumbMaxEdge / w
			w = thumbMaxEdge
		} else {
			w = w * thumbMaxEdge / h
			h = thumbMaxEdge
		}
		if h < 1 {
			h = 1
		}
		if w < 1 {
			w = 1
		}
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: thumbQuality}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return out.Bytes(), nil
}
