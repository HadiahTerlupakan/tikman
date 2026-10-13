package netflow

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustIPSet(t *testing.T, list string) *IPSet {
	t.Helper()
	set, err := ParseIPSet(strings.NewReader(list))
	require.NoError(t, err)
	return set
}

// A Tor exit usually sits in a hosting network too; it must read as Tor.
func TestClassifyTakesTheMostSpecificList(t *testing.T) {
	lists := AnonymityLists{
		Tor:     mustIPSet(t, "192.0.2.10\n"),
		VPN:     mustIPSet(t, "192.0.2.0/27\n"),
		Hosting: mustIPSet(t, "192.0.2.0/24\n"),
	}

	assert.Equal(t, AnonymityTor, lists.Classify(addr("192.0.2.10")))
	assert.Equal(t, AnonymityVPN, lists.Classify(addr("192.0.2.11")))
	assert.Equal(t, AnonymityHosting, lists.Classify(addr("192.0.2.200")))
	assert.Empty(t, lists.Classify(addr("198.51.100.1")))
}

func TestClassifyWithListsMissingIsEmpty(t *testing.T) {
	assert.Empty(t, AnonymityLists{Hosting: mustIPSet(t, "192.0.2.0/24\n")}.Classify(addr("198.51.100.1")))
	assert.Equal(t, AnonymityHosting, AnonymityLists{Hosting: mustIPSet(t, "192.0.2.0/24\n")}.Classify(addr("192.0.2.1")),
		"a list still loading does not stop the others")
}

func TestListDownloadRefusesAnErrorPage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tor.txt")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>429 Too Many Requests</html>"))
	}))
	defer server.Close()

	_, err := (&ListFetcher{URL: server.URL, Path: path, Client: server.Client()}).Download()

	assert.Error(t, err)
	assert.NoFileExists(t, path)
}
