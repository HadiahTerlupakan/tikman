package librenms

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// flattenPNG paints a graph onto opaque white. LibreNMS draws on a
// transparent background with black legend text, which WhatsApp's dark mode
// shows as black on near-black.
func flattenPNG(raw []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("gambar dari LibreNMS rusak: %w", err)
	}
	bounds := src.Bounds()
	flat := image.NewRGBA(bounds)
	draw.Draw(flat, bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(flat, bounds, src, bounds.Min, draw.Over)

	var out bytes.Buffer
	if err := png.Encode(&out, flat); err != nil {
		return nil, fmt.Errorf("encode graph: %w", err)
	}
	return out.Bytes(), nil
}
