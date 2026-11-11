package librenms

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func solidGraph(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.RGBA{G: 0x80, A: 0xff})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func decodeComposed(t *testing.T, raw []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	return img
}

func TestComposeStacksEverySection(t *testing.T) {
	cases := []struct {
		name    string
		heights []int
	}{
		{"one graph", []int{330}},
		{"three graphs", []int{330, 300, 330}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sections []Section
			want := 2*composePadding + (len(tc.heights)-1)*sectionGap
			for _, h := range tc.heights {
				sections = append(sections, Section{Title: "Uplink", Subtitle: "range", Graph: solidGraph(t, 980, h)})
				want += bandHeight + h
			}
			raw, err := Compose(sections)
			require.NoError(t, err)

			bounds := decodeComposed(t, raw).Bounds()
			assert.Equal(t, want, bounds.Dy())
			assert.Equal(t, 980+2*composePadding, bounds.Dx())
		})
	}
}

func TestComposeDrawsTheTitle(t *testing.T) {
	raw, err := Compose([]Section{{Title: "Uplink IX", Subtitle: "10/11/2026", Graph: solidGraph(t, 400, 100)}})
	require.NoError(t, err)
	img := decodeComposed(t, raw)

	inked := 0
	for y := composePadding; y < composePadding+bandHeight; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if r, g, b, _ := img.At(x, y).RGBA(); r != 0xffff || g != 0xffff || b != 0xffff {
				inked++
			}
		}
	}
	assert.Greater(t, inked, 50, "the title band holds drawn text")
}

func TestComposeRefusesWhatIsNotAGraph(t *testing.T) {
	_, err := Compose([]Section{{Title: "x", Graph: []byte("not a png")}})
	assert.Error(t, err)
	_, err = Compose(nil)
	assert.Error(t, err)
}

func TestFitTextCutsALabelThatOverflows(t *testing.T) {
	fc, err := loadFaces()
	require.NoError(t, err)

	assert.Equal(t, "Uplink", fitText(fc.title, "Uplink", 400))

	long := strings.Repeat("Sangat Panjang ", 20)
	cut := fitText(fc.title, long, 200)
	assert.True(t, strings.HasSuffix(cut, ellipsis))
	assert.Less(t, len([]rune(cut)), len([]rune(long)))
	assert.LessOrEqual(t, font.MeasureString(fc.title, cut), fixed.I(200))
}
