package linkpreview

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstURLFindsTheLinkInASentence(t *testing.T) {
	assert.Equal(t, "https://www.facebook.com",
		firstURL("Pak silakan cek https://www.facebook.com ya"))
	assert.Equal(t, "http://example.com/a?b=1", firstURL("http://example.com/a?b=1"))
}

// Trailing punctuation belongs to the sentence, not the address.
func TestFirstURLLeavesSentencePunctuationBehind(t *testing.T) {
	assert.Equal(t, "https://example.com", firstURL("Coba https://example.com."))
	assert.Equal(t, "https://example.com/x", firstURL("Lihat (https://example.com/x)"))
}

func TestFirstURLFindsNothingWhenThereIsNoLink(t *testing.T) {
	assert.Empty(t, firstURL("tidak ada tautan di sini"))
	assert.Empty(t, firstURL(""))
}

func TestOpenGraphIsPreferredOverTheTitleTag(t *testing.T) {
	og := parseOpenGraph([]byte(`<html><head>
		<title>fallback</title>
		<meta property="og:title" content="Judul OG">
		<meta property="og:description" content="Deskripsi OG">
		<meta property="og:image" content="https://cdn.example.com/a.jpg">
	</head></html>`))

	assert.Equal(t, "Judul OG", og.Title)
	assert.Equal(t, "Deskripsi OG", og.Description)
	assert.Equal(t, "https://cdn.example.com/a.jpg", og.Image)
}

// Most of the web has no OpenGraph at all. A bare <title> still makes a card
// worth showing, so falling back is the difference between a preview and none.
func TestTheTitleTagIsUsedWhenThereIsNoOpenGraph(t *testing.T) {
	og := parseOpenGraph([]byte(`<html><head><title>Judul biasa</title></head></html>`))

	assert.Equal(t, "Judul biasa", og.Title)
	assert.Empty(t, og.Image)
}

// A page that is not HTML, or is broken, must produce nothing rather than an
// error the send path has to handle.
func TestRubbishProducesAnEmptyPreview(t *testing.T) {
	assert.Empty(t, parseOpenGraph([]byte("\x00\xff not html at all")).Title)
	assert.Empty(t, parseOpenGraph(nil).Title)
}
