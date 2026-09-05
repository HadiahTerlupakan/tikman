package linkpreview

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rule the whole package answers to: a message with no link costs nothing
// and reaches the network not at all.
func TestAMessageWithoutALinkResolvesToNothing(t *testing.T) {
	assert.Nil(t, Resolve(context.Background(), "halo pak, sudah dicek ya"))
}

// A link the guard refuses must produce no preview and no error — the caller
// has no error path, because the message must go out regardless.
func TestARefusedAddressResolvesToNothing(t *testing.T) {
	for _, body := range []string{
		"cek http://169.254.169.254/latest/meta-data/",
		"cek http://127.0.0.1:8080/admin",
		"cek http://172.20.0.4:8080/api/v1/cs/conversations",
		"cek file:///etc/passwd",
	} {
		assert.Nil(t, Resolve(context.Background(), body), body)
	}
}

// A context already past its deadline stands in for a slow site: the send
// carries on rather than waiting.
func TestACancelledContextResolvesToNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Nil(t, Resolve(ctx, "cek https://example.com"))
}

// absoluteImageURL is what lets a page that names its image relatively still
// get a thumbnail; most do.
func TestARelativeImageIsResolvedAgainstThePage(t *testing.T) {
	got, err := absoluteImageURL("https://example.com/a/b/page.html", "/img/x.jpg")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/img/x.jpg", got)

	got, err = absoluteImageURL("https://example.com/a/b/page.html", "cover.png")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/a/b/cover.png", got)

	got, err = absoluteImageURL("https://example.com/", "https://cdn.other.com/y.jpg")
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.other.com/y.jpg", got)
}
