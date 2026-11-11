package librenms

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Layout of a composed report, in pixels. A section is a title band followed
// by its graph; sections are divided by a gap with a hairline in its middle.
const (
	composePadding   = 16
	bandHeight       = 56
	titleBaseline    = 24
	subtitleBaseline = 46
	sectionGap       = 17
	titleSize        = 18
	subtitleSize     = 14
	ellipsis         = "…"
)

var (
	titleInk     = color.RGBA{R: 0x1f, G: 0x29, B: 0x37, A: 0xff}
	subtitleInk  = color.RGBA{R: 0x6b, G: 0x72, B: 0x80, A: 0xff}
	separatorInk = color.RGBA{R: 0xe5, G: 0xe7, B: 0xeb, A: 0xff}
)

// Section is one graph of a composed report with the text drawn above it.
// The title is what the admin named the port: LibreNMS's own graph title is
// not used because it carries the router's address.
type Section struct {
	Title    string
	Subtitle string
	Graph    []byte
}

type faces struct {
	title, subtitle font.Face
}

// loadFaces parses the embedded Go fonts once; they are compiled into the
// binary, so failing to parse them is a broken build, not a runtime state.
var loadFaces = sync.OnceValues(func() (faces, error) {
	title, err := newFace(gobold.TTF, titleSize)
	if err != nil {
		return faces{}, err
	}
	subtitle, err := newFace(goregular.TTF, subtitleSize)
	if err != nil {
		return faces{}, err
	}
	return faces{title: title, subtitle: subtitle}, nil
})

// drawMu serialises drawing: faces cache glyphs and are not safe for
// concurrent use, and a report is composed rarely enough that this costs
// nothing.
var drawMu sync.Mutex

func newFace(ttf []byte, size float64) (font.Face, error) {
	parsed, err := opentype.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

// Compose stacks the sections into one PNG on white, so a customer gets
// several ports in one picture instead of one message per port.
func Compose(sections []Section) ([]byte, error) {
	if len(sections) == 0 {
		return nil, fmt.Errorf("tidak ada grafik untuk disusun")
	}
	graphs, width, err := decodeGraphs(sections)
	if err != nil {
		return nil, err
	}
	fc, err := loadFaces()
	if err != nil {
		return nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width+2*composePadding, composedHeight(graphs)))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)

	drawMu.Lock()
	defer drawMu.Unlock()
	y := composePadding
	for i, graph := range graphs {
		if i > 0 {
			drawSeparator(canvas, y+sectionGap/2)
			y += sectionGap
		}
		drawText(canvas, fc.title, titleInk, sections[i].Title, y+titleBaseline, width)
		drawText(canvas, fc.subtitle, subtitleInk, sections[i].Subtitle, y+subtitleBaseline, width)
		y += bandHeight
		at := image.Pt(composePadding, y)
		draw.Draw(canvas, graph.Bounds().Sub(graph.Bounds().Min).Add(at), graph, graph.Bounds().Min, draw.Over)
		y += graph.Bounds().Dy()
	}

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("encode report image: %w", err)
	}
	return out.Bytes(), nil
}

func decodeGraphs(sections []Section) ([]image.Image, int, error) {
	graphs := make([]image.Image, len(sections))
	width := 0
	for i, section := range sections {
		graph, err := png.Decode(bytes.NewReader(section.Graph))
		if err != nil {
			return nil, 0, fmt.Errorf("gambar dari LibreNMS rusak: %w", err)
		}
		graphs[i] = graph
		width = max(width, graph.Bounds().Dx())
	}
	return graphs, width, nil
}

func composedHeight(graphs []image.Image) int {
	height := 2*composePadding + (len(graphs)-1)*sectionGap
	for _, graph := range graphs {
		height += bandHeight + graph.Bounds().Dy()
	}
	return height
}

func drawSeparator(canvas *image.RGBA, y int) {
	line := image.Rect(composePadding, y, canvas.Bounds().Dx()-composePadding, y+1)
	draw.Draw(canvas, line, image.NewUniform(separatorInk), image.Point{}, draw.Src)
}

func drawText(canvas *image.RGBA, face font.Face, ink color.Color, text string, baseline, maxWidth int) {
	drawer := font.Drawer{
		Dst: canvas, Src: image.NewUniform(ink), Face: face,
		Dot: fixed.P(composePadding, baseline),
	}
	drawer.DrawString(fitText(face, text, maxWidth))
}

// fitText cuts text that would run past maxWidth pixels and marks the cut
// with an ellipsis.
func fitText(face font.Face, text string, maxWidth int) string {
	limit := fixed.I(maxWidth)
	if font.MeasureString(face, text) <= limit {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && font.MeasureString(face, string(runes)+ellipsis) > limit {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}
