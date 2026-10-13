package netflow

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// peeringDBAnswer is a full-sized list: two typed networks, one untyped, and
// filler up to the size ParseASNTypes expects.
func peeringDBAnswer(filler int) string {
	nets := []string{`{"asn":64496,"info_type":"Content"}`, `{"asn":64497,"info_type":"Cable/DSL/ISP"}`,
		`{"asn":64498,"info_type":""}`}
	for i := 0; i < filler; i++ {
		nets = append(nets, fmt.Sprintf(`{"asn":%d,"info_type":"NSP"}`, 100000+i))
	}
	return `{"data":[` + strings.Join(nets, ",") + `],"meta":{}}`
}

func TestFetchASNTypesKeepsTypedNetworks(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(peeringDBAnswer(minASNTypesNetworks)))
	}))
	defer server.Close()

	types, err := FetchASNTypes(server.Client(), server.URL, "prefix.secret")

	require.NoError(t, err)
	assert.Equal(t, "Content", types[64496])
	assert.Equal(t, "Cable/DSL/ISP", types[64497])
	assert.NotContains(t, types, uint32(64498), "a network that states no type")
	assert.Equal(t, "Api-Key prefix.secret", auth)
}

func TestFetchASNTypesWithoutAKeyIsAnonymous(t *testing.T) {
	auth := "unset"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(peeringDBAnswer(minASNTypesNetworks)))
	}))
	defer server.Close()

	_, err := FetchASNTypes(server.Client(), server.URL, "")

	require.NoError(t, err)
	assert.Empty(t, auth)
}

// A rate-limited or truncated answer must not replace a day's types with a
// handful.
func TestParseASNTypesRefusesAShortList(t *testing.T) {
	for name, body := range map[string]string{
		"short":    peeringDBAnswer(10),
		"not json": "<html>throttled</html>",
		"error":    `{"meta":{"error":"Request was throttled"}}`,
	} {
		_, err := ParseASNTypes(strings.NewReader(body))
		assert.Error(t, err, name)
	}
}
