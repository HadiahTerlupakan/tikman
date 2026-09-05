package linkpreview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// httptest binds on 127.0.0.1, so every server here is an address the guard
// must refuse. That makes the guard untestable against a real listener — the
// tests below therefore reach past the dialler and exercise the pieces the
// guard is built from, and the redirect test asserts the policy directly.
func TestOnlyHTTPSchemesAreFetched(t *testing.T) {
	for _, raw := range []string{
		"file:///etc/passwd",
		"ftp://example.com/x",
		"gopher://example.com",
		"data:text/html,<title>x</title>",
	} {
		_, err := get(context.Background(), raw, 1024)
		require.Error(t, err, raw)
		assert.Contains(t, err.Error(), "scheme")
	}
}

// A public hostname that redirects to an internal one is the way past a guard
// that only checks the address the user typed.
func TestARedirectIsCheckedAgain(t *testing.T) {
	err := checkRedirect(&http.Request{URL: mustURL(t, "http://169.254.169.254/latest/meta-data/")}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestRedirectsAreCapped(t *testing.T) {
	via := make([]*http.Request, maxRedirects+1)
	err := checkRedirect(&http.Request{URL: mustURL(t, "https://example.com/")}, via)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many")
}

// A page that never ends must not be read until the process runs out of
// memory; the cap is what bounds it.
func TestTheBodyIsCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for i := 0; i < 100; i++ {
			_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
		}
	}))
	defer srv.Close()

	body, err := readCapped(srv.Client(), srv.URL, 2048)

	require.NoError(t, err)
	assert.Len(t, body, 2048)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}
