# MikroTik Protocol Choice (REST / API-SSL / API) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let each MikroTik router be read over REST (`www-ssl`), RouterOS API over TLS (`api-ssl`), or plain RouterOS API (`api`), chosen per router.

**Architecture:** `internal/mikrotik` gets a `Transport` interface (`List(path, out)`, `Close()`) with two implementations — the existing REST client and a new hand-written RouterOS API client (plain TCP or pinned TLS). A `Client` wraps any transport and keeps the typed read methods, so polling, discovery and BGP code do not change. A single `mikrotik.Open(Config)` builds the client; the router row gains `api_protocol`.

**Tech Stack:** Go 1.26 stdlib (`net`, `crypto/tls`, `bufio`, `encoding/json`, `reflect`), GORM, Gin, PostgreSQL/TimescaleDB, React + TypeScript + Ant Design, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-12-mikrotik-api-protocol-design.md`

## Global Constraints

- No new dependencies (Go or npm). The RouterOS API client is stdlib only.
- Protocol values exactly: `rest`, `api-ssl`, `api`. Empty input means `rest`.
- Default ports exactly: `rest` 8443, `api-ssl` 8729, `api` 8728.
- Fingerprint (64 lowercase hex SHA-256) is required and pinned for `rest` and `api-ssl`; for `api` it is ignored and stored as `""`.
- Only read commands (`<path>/print`); never send a write command to a router.
- RouterOS v7 login only: one `/login` with `=name=` and `=password=`.
- One API call (dial, login, or command) is bounded by 5 seconds; a reply word longer than 1 MiB is a malformed reply.
- The repository is PUBLIC: fixtures, tests and docs use only 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24 and AS 64496–64511. No real router IPs, names, AS numbers or communities.
- Files ≤ 350 lines, functions ≤ 50 lines, ≤ 3 nesting levels (CLAUDE.md). Comments explain why, not what. Exported Go identifiers get short doc comments.
- UI copy is Indonesian, matching the existing router pages.
- Never change a production router's configuration from code or tests.

## Review Focus

1. Operator picks **API** but points it at a TLS port (e.g. `www-ssl`): the API client must fail with an error within the call timeout, not hang or allocate a huge buffer from TLS bytes read as a length. → Task 2, `TestAPIFailsFastAgainstATLSPort`.
2. The router drops the API session mid-poll (`!fatal` or EOF): the poll records an API error and the next poll opens a fresh connection. → Task 2 `TestAPIReportsFatal`, Task 4 `TestPollBGPRecordsAFailedOpen`.
3. Password changed on the router after the router was added: the poll stores `api_status=error` with the refusal, nothing panics. → Task 4 `TestPollBGPRecordsAFailedOpen`.
4. Editing a router from `api` to `rest`/`api-ssl` leaves an empty pinned fingerprint: the first poll must report `certificate_changed` (so the existing **Terima** flow takes over), never connect unpinned. → Task 3 `TestOpenRESTWithNoPinnedFingerprintRefusesTheRouter`.
5. A poll's API connection must be closed even when a read fails half-way. → Task 4 `TestPollBGPClosesTheConnection`, `TestDiscoverClosesTheConnectionOnError`.

---

## File Structure

Backend (`backend/`):

| File | Responsibility |
|---|---|
| `internal/mikrotik/protocol.go` (new) | `Protocol`, its constants, `Valid`, `UsesTLS`, `DefaultPort`; `Config` |
| `internal/mikrotik/apiwire.go` (new) | RouterOS API word/sentence encoding and decoding |
| `internal/mikrotik/api.go` (new) | `apiTransport`: dial (TCP or pinned TLS), login, `List`, `Close` |
| `internal/mikrotik/rest.go` (new, moved from client.go) | `restTransport`, `pinnedTLS`, `Fingerprint`, `FetchFingerprint` |
| `internal/mikrotik/client.go` (rewritten) | errors, `Transport`, `Client`, `Open`, typed read methods |
| `internal/mikrotik/apiwire_test.go`, `api_test.go` (new); `client_test.go` (updated) | tests |
| `internal/models/mikrotik.go` | `APIProtocol` field |
| `migrations/57_mikrotik_api_protocol.sql` (new) | column + CHECK |
| `internal/database/migrations_mikrotik_postgres_test.go` (new) | CHECK and default on Postgres |
| `internal/services/router_service.go`, `router_connection.go`, `router_poll.go`, `router_discovery.go` | protocol in create/update/test/poll |
| `internal/services/*_test.go` | tests |
| `internal/api/router_dto.go`, `router_handler.go`, `router_handler_test.go` | `api_protocol` over HTTP |

Frontend (`frontend/src/`):

| File | Responsibility |
|---|---|
| `domain/entities/MikrotikRouter.ts` | `RouterApiProtocol`, fields on entity and DTOs |
| `presentation/components/routers/routerProtocol.ts` (new) | default ports, labels, `usesTls` |
| `presentation/components/routers/routerTestSteps.ts` (+ test) | `savableFingerprint` replaces `confirmedFingerprint` |
| `presentation/components/routers/useRouterTestSteps.ts` | takes the protocol |
| `presentation/components/routers/RouterFormFields.tsx` | protocol radio + notices |
| `presentation/components/routers/RouterFormModal.tsx` (+ new test) | port follows protocol, one-step test for API |
| `RouterInfoTab.tsx`, `RouterStatusTag.tsx`, `BgpSessionTable.tsx`, `pages/RoutersPage.tsx` | show protocol; "REST" wording → "API" |

Docs: `docs/operator_guide.md` (MikroTik section).

---

### Task 1: RouterOS API wire encoding

**Files:**
- Create: `backend/internal/mikrotik/apiwire.go`
- Test: `backend/internal/mikrotik/apiwire_test.go`

**Interfaces:**
- Produces (package-private, used by Task 2): `encodeLength(n int) []byte`, `readLength(r *bufio.Reader) (int, error)`, `writeSentence(w io.Writer, words ...string) error`, `readSentence(r *bufio.Reader) ([]string, error)`, `type reply struct { kind string; attrs map[string]string }`, `parseSentence(words []string) (reply, error)`, `errBadReply`, `maxWordLength`.

- [ ] **Step 1: Write the failing tests**

```go
package mikrotik

import (
	"bufio"
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reader(b []byte) *bufio.Reader { return bufio.NewReader(bytes.NewReader(b)) }

// Each width change of the length prefix is where an off-by-one would corrupt
// every following word, so both sides of every boundary are pinned.
func TestLengthEncodingAtEveryBoundary(t *testing.T) {
	cases := []struct {
		n    int
		want []byte
	}{
		{0, []byte{0x00}},
		{0x7F, []byte{0x7F}},
		{0x80, []byte{0x80, 0x80}},
		{0x3FFF, []byte{0xBF, 0xFF}},
		{0x4000, []byte{0xC0, 0x40, 0x00}},
		{0x1FFFFF, []byte{0xDF, 0xFF, 0xFF}},
		{0x200000, []byte{0xE0, 0x20, 0x00, 0x00}},
		{0xFFFFFFF, []byte{0xEF, 0xFF, 0xFF, 0xFF}},
		{0x10000000, []byte{0xF0, 0x10, 0x00, 0x00, 0x00}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, encodeLength(c.n), "encode %#x", c.n)
		got, err := readLength(reader(c.want))
		require.NoError(t, err, "decode %#x", c.n)
		assert.Equal(t, c.n, got, "decode %#x", c.n)
	}
}

func TestSentenceRoundTrip(t *testing.T) {
	long := string(bytes.Repeat([]byte("x"), 200))
	var buf bytes.Buffer
	require.NoError(t, writeSentence(&buf, "/login", "=name=tikman", "=password="+long))

	words, err := readSentence(reader(buf.Bytes()))

	require.NoError(t, err)
	assert.Equal(t, []string{"/login", "=name=tikman", "=password=" + long}, words)
}

// A control byte (0xF8-0xFF) or an absurd length means the stream is not the
// RouterOS API — for instance a TLS port answering a plain connection.
func TestReadSentenceRejectsWhatIsNotTheAPI(t *testing.T) {
	_, err := readSentence(reader([]byte{0xF8}))
	assert.True(t, errors.Is(err, errBadReply), "control byte: %v", err)

	_, err = readSentence(reader(encodeLength(maxWordLength + 1)))
	assert.True(t, errors.Is(err, errBadReply), "oversized word: %v", err)
}

func TestParseSentenceKeepsEqualsInsideAValue(t *testing.T) {
	r, err := parseSentence([]string{"!re", "=comment=uplink=transit", "=name=vlan-100", ".tag=7"})

	require.NoError(t, err)
	assert.Equal(t, "!re", r.kind)
	assert.Equal(t, "uplink=transit", r.attrs["comment"])
	assert.Equal(t, "vlan-100", r.attrs["name"])
	assert.NotContains(t, r.attrs, ".tag")
}

func TestParseSentenceRejectsAnEmptySentence(t *testing.T) {
	_, err := parseSentence(nil)
	assert.True(t, errors.Is(err, errBadReply))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/mikrotik/ -run 'TestLength|TestSentence|TestReadSentence|TestParseSentence' -v`
Expected: FAIL — `undefined: encodeLength` (and the other helpers).

- [ ] **Step 3: Write the implementation**

```go
package mikrotik

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxWordLength bounds one word of a reply. RouterOS words are property
// values; anything this long means the stream is not the API, and reading it
// would only allocate what a wrong port sent.
const maxWordLength = 1 << 20

// errBadReply means the bytes read cannot be a RouterOS API sentence.
var errBadReply = errors.New("malformed RouterOS API reply")

// encodeLength writes a word length the way the RouterOS API prefixes it: the
// high bits of the first byte say how many bytes follow.
func encodeLength(n int) []byte {
	switch {
	case n < 0x80:
		return []byte{byte(n)}
	case n < 0x4000:
		v := n | 0x8000
		return []byte{byte(v >> 8), byte(v)}
	case n < 0x200000:
		v := n | 0xC00000
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case n < 0x10000000:
		v := uint32(n) | 0xE0000000
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	default:
		return []byte{0xF0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
}

func readLength(r *bufio.Reader) (int, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	var extra, n int
	switch {
	case first&0x80 == 0x00:
		return int(first), nil
	case first&0xC0 == 0x80:
		extra, n = 1, int(first&0x3F)
	case first&0xE0 == 0xC0:
		extra, n = 2, int(first&0x1F)
	case first&0xF0 == 0xE0:
		extra, n = 3, int(first&0x0F)
	case first == 0xF0:
		extra = 4
	default:
		return 0, fmt.Errorf("%w: control byte %#x", errBadReply, first)
	}
	for i := 0; i < extra; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n = n<<8 | int(b)
	}
	return n, nil
}

// writeSentence sends words as one sentence, ended by an empty word, in a
// single write so a sentence is never interleaved with a half-sent one.
func writeSentence(w io.Writer, words ...string) error {
	var buf bytes.Buffer
	for _, word := range words {
		buf.Write(encodeLength(len(word)))
		buf.WriteString(word)
	}
	buf.WriteByte(0)
	_, err := w.Write(buf.Bytes())
	return err
}

func readSentence(r *bufio.Reader) ([]string, error) {
	var words []string
	for {
		n, err := readLength(r)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return words, nil
		}
		if n > maxWordLength {
			return nil, fmt.Errorf("%w: word of %d bytes", errBadReply, n)
		}
		word := make([]byte, n)
		if _, err := io.ReadFull(r, word); err != nil {
			return nil, err
		}
		words = append(words, string(word))
	}
}

// reply is one sentence from the router: its type ("!re", "!done", "!trap",
// "!fatal", "!empty") and its "=key=value" attributes.
type reply struct {
	kind  string
	attrs map[string]string
}

// parseSentence splits attributes on the second "=" only, since values such
// as comments may contain "=" themselves. Words that are not attributes (the
// ".tag" of a tagged command) carry nothing a read needs.
func parseSentence(words []string) (reply, error) {
	if len(words) == 0 {
		return reply{}, fmt.Errorf("%w: empty sentence", errBadReply)
	}
	r := reply{kind: words[0], attrs: map[string]string{}}
	for _, word := range words[1:] {
		if !strings.HasPrefix(word, "=") {
			continue
		}
		key, value, _ := strings.Cut(word[1:], "=")
		r.attrs[key] = value
	}
	return r, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/mikrotik/ -run 'TestLength|TestSentence|TestReadSentence|TestParseSentence' -v -race`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l internal/mikrotik && go vet ./internal/mikrotik/
git add internal/mikrotik/apiwire.go internal/mikrotik/apiwire_test.go
git commit -m "feat(mikrotik): RouterOS API word and sentence encoding"
```

---

### Task 2: Protocol, Config and the RouterOS API transport

**Files:**
- Create: `backend/internal/mikrotik/protocol.go`
- Create: `backend/internal/mikrotik/api.go`
- Test: `backend/internal/mikrotik/api_test.go`

**Interfaces:**
- Consumes: Task 1 helpers; from the existing `client.go`: `requestTimeout`, `ErrUnauthorized`, `ErrCertificateChanged`, `pinnedTLS(fingerprint string) *tls.Config`, `Fingerprint(der []byte) string`.
- Produces:
  - `type Protocol string`; `const ProtocolREST Protocol = "rest"`, `ProtocolAPISSL Protocol = "api-ssl"`, `ProtocolAPI Protocol = "api"`
  - `func (p Protocol) Valid() bool`, `func (p Protocol) UsesTLS() bool`, `func (p Protocol) DefaultPort() int`
  - `type Config struct { Protocol Protocol; Host string; Port int; Username, Password, Fingerprint string }`
  - package-private `dialAPI(cfg Config) (*apiTransport, error)`; `(*apiTransport).List(path string, out interface{}) error`; `(*apiTransport).Close() error`
  - package-private `var apiCallTimeout = requestTimeout` (tests shorten it)
  - test helper (in `api_test.go`, same package, reused by Task 3): `type fakeAPIRouter`, `(*fakeAPIRouter).start(t, tlsConfig *tls.Config) int`, `testTLSCertificate(t) (tls.Certificate, string)`

- [ ] **Step 1: Write the failing tests**

```go
package mikrotik

import (
	"bufio"
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAPIRouter answers the RouterOS API on a local port: one login, then the
// sentences in replies for each command word it receives.
type fakeAPIRouter struct {
	username, password string
	replies            map[string][][]string
	silent             bool
	received           chan []string
}

func (f *fakeAPIRouter) start(t *testing.T, tlsConfig *tls.Config) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	if tlsConfig != nil {
		listener = tls.NewListener(listener, tlsConfig)
	}
	t.Cleanup(func() { _ = listener.Close() })
	f.received = make(chan []string, 16)
	go f.serve(listener)
	return listener.Addr().(*net.TCPAddr).Port
}

func (f *fakeAPIRouter) serve(listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for {
		words, err := readSentence(r)
		if err != nil {
			return
		}
		f.received <- words
		if f.silent {
			continue
		}
		for _, sentence := range f.answer(words) {
			if writeSentence(conn, sentence...) != nil {
				return
			}
		}
	}
}

func (f *fakeAPIRouter) answer(words []string) [][]string {
	if words[0] != "/login" {
		return f.replies[words[0]]
	}
	if slices.Contains(words, "=name="+f.username) && slices.Contains(words, "=password="+f.password) {
		return [][]string{{"!done"}}
	}
	return [][]string{{"!trap", "=message=invalid user name or password (6)"}, {"!done"}}
}

// testTLSCertificate borrows httptest's self-signed certificate, the same kind
// a router serves, and returns it with its fingerprint.
func testTLSCertificate(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	server := httptest.NewTLSServer(nil)
	t.Cleanup(server.Close)
	cert := server.TLS.Certificates[0]
	return cert, Fingerprint(cert.Certificate[0])
}

func apiConfig(port int) Config {
	return Config{Protocol: ProtocolAPI, Host: "127.0.0.1", Port: port, Username: "tikman", Password: "secret"}
}

func shortenAPITimeout(t *testing.T) {
	t.Helper()
	saved := apiCallTimeout
	apiCallTimeout = 300 * time.Millisecond
	t.Cleanup(func() { apiCallTimeout = saved })
}

func TestAPIReadsRowsAfterLogin(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/session/print": {
			{"!re", "=.id=*1", "=name=peer-transit-1", "=remote.address=198.51.100.1", "=established=true"},
			{"!re", "=.id=*2", "=name=peer-ix-1", "=remote.address=203.0.113.166"},
			{"!done"},
		},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	defer func() { _ = transport.Close() }()

	var sessions []BGPSession
	require.NoError(t, transport.List("/routing/bgp/session", &sessions))

	require.Len(t, sessions, 2)
	assert.Equal(t, "peer-transit-1", sessions[0].Name)
	assert.Equal(t, "198.51.100.1", sessions[0].RemoteAddress)
	assert.Equal(t, "true", sessions[0].Established)
	assert.Empty(t, sessions[1].Established)
	assert.Equal(t, []string{"/login", "=name=tikman", "=password=secret"}, <-router.received)
	assert.Equal(t, []string{"/routing/bgp/session/print"}, <-router.received)
}

func TestAPIFillsAStructFromASingleRow(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/system/resource/print": {{"!re", "=board-name=CCR2116-12G-4S+", "=version=7.24.2 (stable)"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var resource SystemResource
	require.NoError(t, transport.List("/system/resource", &resource))

	assert.Equal(t, "CCR2116-12G-4S+", resource.BoardName)
	assert.Equal(t, "7.24.2 (stable)", resource.Version)
}

func TestAPIRefusesWrongCredentials(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "other"}

	_, err := dialAPI(apiConfig(router.start(t, nil)))

	assert.True(t, errors.Is(err, ErrUnauthorized), "got %v", err)
}

func TestAPIReportsATrapWithItsMessage(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/interface/vlan/print": {{"!trap", "=message=no such command prefix"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var vlans []VLAN
	err = transport.List("/interface/vlan", &vlans)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such command prefix")
	assert.False(t, errors.Is(err, ErrUnauthorized), "a refused command is not a refused login")
}

// RouterOS 7.18+ answers an empty listing with !empty before !done.
func TestAPITreatsEmptyAsNoRows(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/connection/print": {{"!empty"}, {"!done"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var connections []BGPConnection
	require.NoError(t, transport.List("/routing/bgp/connection", &connections))
	assert.Empty(t, connections)
}

func TestAPIReportsFatal(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/interface/print": {{"!fatal", "session terminated on request"}},
	}}
	transport, err := dialAPI(apiConfig(router.start(t, nil)))
	require.NoError(t, err)

	var ifaces []Interface
	err = transport.List("/interface", &ifaces)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "session terminated on request")
}

func TestAPIGivesUpOnASilentRouter(t *testing.T) {
	shortenAPITimeout(t)
	router := &fakeAPIRouter{silent: true}
	port := router.start(t, nil)

	started := time.Now()
	_, err := dialAPI(apiConfig(port))

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}

// Review Focus 1: API chosen, but the port is a TLS service. The server reads
// the login sentence as a broken ClientHello and answers with an alert; the
// client must turn that into an error promptly, not hang or trust TLS bytes
// as a word length.
func TestAPIFailsFastAgainstATLSPort(t *testing.T) {
	shortenAPITimeout(t)
	cert, _ := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret"}
	port := router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}})

	started := time.Now()
	_, err := dialAPI(apiConfig(port))

	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second)
}

func TestAPISSLReadsThroughThePinnedCertificate(t *testing.T) {
	cert, fingerprint := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/system/clock/print": {{"!re", "=gmt-offset=+07:00"}, {"!done"}},
	}}
	cfg := apiConfig(router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}}))
	cfg.Protocol, cfg.Fingerprint = ProtocolAPISSL, fingerprint

	transport, err := dialAPI(cfg)
	require.NoError(t, err)

	var clock Clock
	require.NoError(t, transport.List("/system/clock", &clock))
	assert.Equal(t, "+07:00", clock.GMTOffset)
}

func TestAPISSLRefusesAnotherCertificateBeforeLoggingIn(t *testing.T) {
	cert, _ := testTLSCertificate(t)
	router := &fakeAPIRouter{username: "tikman", password: "secret"}
	cfg := apiConfig(router.start(t, &tls.Config{Certificates: []tls.Certificate{cert}}))
	cfg.Protocol, cfg.Fingerprint = ProtocolAPISSL, "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := dialAPI(cfg)

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.Empty(t, router.received, "the password must not reach a router that failed the pin")
}

func TestProtocolDefaults(t *testing.T) {
	assert.Equal(t, 8443, ProtocolREST.DefaultPort())
	assert.Equal(t, 8729, ProtocolAPISSL.DefaultPort())
	assert.Equal(t, 8728, ProtocolAPI.DefaultPort())
	assert.True(t, ProtocolREST.UsesTLS())
	assert.True(t, ProtocolAPISSL.UsesTLS())
	assert.False(t, ProtocolAPI.UsesTLS())
	assert.False(t, Protocol("telnet").Valid())
	assert.False(t, Protocol("").Valid())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/mikrotik/ -run 'TestAPI|TestProtocol' -v`
Expected: FAIL — `undefined: dialAPI`, `undefined: Config`, `undefined: ProtocolAPI`.

- [ ] **Step 3: Write `protocol.go`**

```go
package mikrotik

// Protocol is how TikMan reads a router: REST over www-ssl, or the RouterOS
// API over api-ssl or plain api.
type Protocol string

// The protocols a router can be read with.
const (
	ProtocolREST   Protocol = "rest"
	ProtocolAPISSL Protocol = "api-ssl"
	ProtocolAPI    Protocol = "api"
)

var defaultPorts = map[Protocol]int{ProtocolREST: 8443, ProtocolAPISSL: 8729, ProtocolAPI: 8728}

// Valid reports whether p is one of the three protocols.
func (p Protocol) Valid() bool {
	_, ok := defaultPorts[p]
	return ok
}

// UsesTLS reports whether the protocol runs over TLS and so pins a
// certificate. Plain api has nothing to pin.
func (p Protocol) UsesTLS() bool { return p == ProtocolREST || p == ProtocolAPISSL }

// DefaultPort is the port the add-router form starts from. REST is 8443, not
// 443, because RouterOS 7's reverse-proxy usually holds 443.
func (p Protocol) DefaultPort() int { return defaultPorts[p] }

// Config is everything needed to open one router connection.
type Config struct {
	Protocol    Protocol
	Host        string
	Port        int
	Username    string
	Password    string
	Fingerprint string // pinned SHA-256; ignored for ProtocolAPI
}
```

- [ ] **Step 4: Write `api.go`**

```go
package mikrotik

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// apiCallTimeout bounds dialing, login and each command, as requestTimeout
// does for REST. A variable only so tests need not wait five seconds.
var apiCallTimeout = requestTimeout

// apiTransport is one logged-in RouterOS API connection. It lives for one poll.
type apiTransport struct {
	conn   net.Conn
	reader *bufio.Reader
}

// trapError is a command the router refused; its message is the router's.
type trapError struct{ message string }

func (e *trapError) Error() string { return "router refused: " + e.message }

// dialAPI connects and logs in, so a refused login surfaces here as
// ErrUnauthorized. Over api-ssl the certificate is checked before the
// password is written.
func dialAPI(cfg Config) (*apiTransport, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: apiCallTimeout}
	var conn net.Conn
	var err error
	if cfg.Protocol == ProtocolAPISSL {
		// DialWithDialer applies the dialer's Timeout to the handshake too.
		conn, err = tls.DialWithDialer(dialer, "tcp", address, pinnedTLS(cfg.Fingerprint))
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		if errors.Is(err, ErrCertificateChanged) {
			return nil, ErrCertificateChanged
		}
		return nil, fmt.Errorf("connect %s: %w", address, err)
	}
	t := &apiTransport{conn: conn, reader: bufio.NewReader(conn)}
	if err := t.login(cfg.Username, cfg.Password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return t, nil
}

func (t *apiTransport) login(username, password string) error {
	_, err := t.run("/login", "=name="+username, "=password="+password)
	var trap *trapError
	if errors.As(err, &trap) {
		return ErrUnauthorized
	}
	return err
}

// run sends one command and collects its !re rows until !done. A !trap is
// followed by !done, so it is remembered and returned once the reply ends.
func (t *apiTransport) run(words ...string) ([]map[string]string, error) {
	if err := t.conn.SetDeadline(time.Now().Add(apiCallTimeout)); err != nil {
		return nil, err
	}
	if err := writeSentence(t.conn, words...); err != nil {
		return nil, fmt.Errorf("send %s: %w", words[0], err)
	}
	var rows []map[string]string
	var trap error
	for {
		sentence, err := readSentence(t.reader)
		if err != nil {
			return nil, fmt.Errorf("read %s reply: %w", words[0], err)
		}
		r, err := parseSentence(sentence)
		if err != nil {
			return nil, err
		}
		switch r.kind {
		case "!re":
			rows = append(rows, r.attrs)
		case "!trap":
			trap = &trapError{message: r.attrs["message"]}
		case "!fatal":
			return nil, fmt.Errorf("router closed the API session: %s", strings.Join(sentence[1:], " "))
		case "!done":
			if trap != nil {
				return nil, trap
			}
			return rows, nil
		}
	}
}

// List reads a menu with "<path>/print". Its rows carry the same property
// names REST uses, so they decode into the same structs.
func (t *apiTransport) List(path string, out interface{}) error {
	rows, err := t.run(path + "/print")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return decodeRows(rows, out)
}

// Close ends the connection; the router drops the session with it.
func (t *apiTransport) Close() error { return t.conn.Close() }

// decodeRows fills out from rows through the structs' json tags. A single-
// object menu such as /system/resource arrives as one row and fills a struct.
func decodeRows(rows []map[string]string, out interface{}) error {
	var value interface{} = rows
	if reflect.TypeOf(out).Elem().Kind() != reflect.Slice {
		value = map[string]string{}
		if len(rows) > 0 {
			value = rows[0]
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
```

Note: `case "!empty":` needs no branch — an unknown kind falls through the switch and the loop reads `!done` next.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/mikrotik/ -v -race`
Expected: PASS, including the earlier REST tests in `client_test.go` and Task 1's tests.

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -s -l internal/mikrotik && go vet ./internal/mikrotik/
git add internal/mikrotik/protocol.go internal/mikrotik/api.go internal/mikrotik/api_test.go
git commit -m "feat(mikrotik): RouterOS API transport over plain TCP and pinned TLS"
```

---

### Task 3: Transport interface, REST behind it, and `Open`

**Files:**
- Create: `backend/internal/mikrotik/rest.go` (REST code moved out of `client.go`)
- Rewrite: `backend/internal/mikrotik/client.go`
- Modify: `backend/internal/mikrotik/client_test.go`
- Modify (callers, minimal compile fix only): `backend/internal/services/router_connection.go:78`, `backend/internal/services/router_poll.go:53`

**Interfaces:**
- Consumes: Task 2 `Protocol`, `Config`, `dialAPI`, `fakeAPIRouter`.
- Produces:
  - `type Transport interface { List(path string, out interface{}) error; Close() error }`
  - `type Client struct` with `SystemResource()`, `Clock()`, `Interfaces()`, `VLANs()`, `Addresses()`, `BGPConnections()`, `BGPSessions()` (signatures unchanged) and new `Close() error`
  - `func Open(cfg Config) (*Client, error)` — REST does no I/O here; API/API-SSL dial and log in. Unknown protocol → error.
  - `NewClient` is removed.

- [ ] **Step 1: Update `client_test.go` to the new constructor and add the new tests**

Replace every `NewClient(host, port, user, pass, fp)` call with `openREST(t, host, port, user, pass, fp)` and add this helper plus three tests:

```go
func openREST(t *testing.T, host string, port int, username, password, fingerprint string) *Client {
	t.Helper()
	client, err := Open(Config{Protocol: ProtocolREST, Host: host, Port: port,
		Username: username, Password: password, Fingerprint: fingerprint})
	require.NoError(t, err)
	return client
}

func TestOpenRejectsAnUnknownProtocol(t *testing.T) {
	_, err := Open(Config{Protocol: "telnet", Host: "127.0.0.1", Port: 1})
	assert.Error(t, err)
}

// The typed reads are shared by every transport: what works over REST works
// over the API without a second implementation.
func TestOpenWithAPIReadsThroughTheSameMethods(t *testing.T) {
	router := &fakeAPIRouter{username: "tikman", password: "secret", replies: map[string][][]string{
		"/routing/bgp/session/print": {{"!re", "=name=peer-transit-1", "=prefix-count=1093192"}, {"!done"}},
	}}
	client, err := Open(apiConfig(router.start(t, nil)))
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	sessions, err := client.BGPSessions()

	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "1093192", sessions[0].PrefixCount)
}

// Review Focus 4: a router edited from api to rest keeps an empty pin. It
// must be refused as a changed certificate, which hands it to the existing
// accept-certificate flow, and never be read unpinned.
func TestOpenRESTWithNoPinnedFingerprintRefusesTheRouter(t *testing.T) {
	called := false
	_, host, port := tlsRouter(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	})

	_, err := openREST(t, host, port, "tikman", "secret", "").SystemResource()

	assert.True(t, errors.Is(err, ErrCertificateChanged), "got %v", err)
	assert.False(t, called)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/mikrotik/ -v`
Expected: FAIL — `undefined: Open`.

- [ ] **Step 3: Create `rest.go`**

Move from the current `client.go`, unchanged in behaviour: `pinnedTLS`, `Fingerprint`, `FetchFingerprint`, and the body of `(*Client).get` as `(*restTransport).List`. The REST client becomes:

```go
package mikrotik

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// errorBodyLimit is how much of a failed response is kept for the message.
const errorBodyLimit = 512

// restTransport reads RouterOS's REST API under /rest.
type restTransport struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func newRESTTransport(cfg Config) *restTransport {
	return &restTransport{
		baseURL:  "https://" + net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)) + "/rest",
		username: cfg.Username,
		password: cfg.Password,
		http: &http.Client{
			Timeout: requestTimeout,
			// A client lives for one poll. A kept-alive connection would outlive
			// it with nothing left to close it, so each request closes its own.
			Transport: &http.Transport{
				TLSClientConfig:   pinnedTLS(cfg.Fingerprint),
				DisableKeepAlives: true,
			},
		},
	}
}

// List is GET /rest<path>.
func (t *restTransport) List(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, t.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(t.username, t.password)

	resp, err := t.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrCertificateChanged) {
			return ErrCertificateChanged
		}
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// Close has nothing to release: every request already closed its connection.
func (t *restTransport) Close() error { return nil }
```

followed by `pinnedTLS`, `Fingerprint` and `FetchFingerprint` exactly as they are in today's `client.go` (with their comments).

- [ ] **Step 4: Rewrite `client.go`**

```go
package mikrotik

import (
	"errors"
	"fmt"
	"time"
)

// requestTimeout bounds one call to the router. The router answers these in
// tens of milliseconds; anything near this long means it is unreachable, and
// the job should give up rather than hold a worker.
const requestTimeout = 5 * time.Second

var (
	// ErrCertificateChanged means the router presented a certificate other
	// than the one pinned when it was added.
	ErrCertificateChanged = errors.New("router certificate changed since it was trusted")
	// ErrUnauthorized means the router rejected the username or password.
	ErrUnauthorized = errors.New("router rejected the API credentials")
)

// Transport reads one RouterOS menu. Paths are the menu paths both REST and
// the API use, such as "/routing/bgp/session".
type Transport interface {
	List(path string, out interface{}) error
	Close() error
}

// Client reads a router through whichever transport its protocol uses. A new
// read is one method here, and it works over every protocol.
type Client struct {
	transport Transport
}

// Open builds a client for cfg. REST does no I/O until the first read; the
// API protocols connect and log in here, so refused credentials surface now.
func Open(cfg Config) (*Client, error) {
	switch cfg.Protocol {
	case ProtocolREST:
		return &Client{transport: newRESTTransport(cfg)}, nil
	case ProtocolAPI, ProtocolAPISSL:
		transport, err := dialAPI(cfg)
		if err != nil {
			return nil, err
		}
		return &Client{transport: transport}, nil
	default:
		return nil, fmt.Errorf("unknown router protocol %q", cfg.Protocol)
	}
}

// Close releases the connection, if the transport holds one.
func (c *Client) Close() error { return c.transport.Close() }

// SystemResource reads uptime, version, CPU load, memory and board name.
func (c *Client) SystemResource() (SystemResource, error) {
	var out SystemResource
	err := c.transport.List("/system/resource", &out)
	return out, err
}

// Clock reads the router's clock settings.
func (c *Client) Clock() (Clock, error) {
	var out Clock
	err := c.transport.List("/system/clock", &out)
	return out, err
}

// Interfaces lists every interface, dynamic ones included.
func (c *Client) Interfaces() ([]Interface, error) {
	var out []Interface
	err := c.transport.List("/interface", &out)
	return out, err
}

// VLANs lists VLAN interfaces with their ID and parent.
func (c *Client) VLANs() ([]VLAN, error) {
	var out []VLAN
	err := c.transport.List("/interface/vlan", &out)
	return out, err
}

// Addresses lists IPv4 addresses and the interface each sits on.
func (c *Client) Addresses() ([]Address, error) {
	var out []Address
	err := c.transport.List("/ip/address", &out)
	return out, err
}

// BGPConnections lists configured BGP peers.
func (c *Client) BGPConnections() ([]BGPConnection, error) {
	var out []BGPConnection
	err := c.transport.List("/routing/bgp/connection", &out)
	return out, err
}

// BGPSessions lists running BGP sessions.
func (c *Client) BGPSessions() ([]BGPSession, error) {
	var out []BGPSession
	err := c.transport.List("/routing/bgp/session", &out)
	return out, err
}
```

(`err := ...; return out, err` replaces `return out, c.get(...)`: Go does not specify whether `out` is read before or after the call in a single return statement.)

Also update the `types.go` package comment's first line to: `// RouterOS sends every value as a string over REST and the API alike, ...` (keep the rest).

- [ ] **Step 5: Keep callers compiling (no behaviour change yet)**

In `internal/services/router_connection.go` replace the `mikrotik.NewClient(...).SystemResource()` line with:

```go
	client, err := mikrotik.Open(mikrotik.Config{Protocol: mikrotik.ProtocolREST, Host: in.Host, Port: apiPort,
		Username: in.APIUsername, Password: in.APIPassword, Fingerprint: fingerprint})
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	resource, err := client.SystemResource()
```

In `internal/services/router_poll.go` replace the `newAPI` body with:

```go
		newAPI: func(router *models.MikrotikRouter, password string) RouterAPI {
			client, _ := mikrotik.Open(mikrotik.Config{Protocol: mikrotik.ProtocolREST, Host: router.Host,
				Port: router.APIPort, Username: router.APIUsername, Password: password, Fingerprint: router.TLSFingerprint})
			return client
		},
```

(REST `Open` never fails; Task 4 replaces this with the protocol-aware, error-returning version.)

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/mikrotik/ ./internal/services/ -run 'Router|Client|Open|API|Fetch|Protocol|Length|Sentence|Parse' -race`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -s -l internal && go vet ./internal/mikrotik/ ./internal/services/
git add internal/mikrotik internal/services/router_connection.go internal/services/router_poll.go
git commit -m "refactor(mikrotik): one Client over REST or RouterOS API transports"
```

---

### Task 4: `api_protocol` in the model, migration and services

**Files:**
- Modify: `backend/internal/models/mikrotik.go` (field on `MikrotikRouter`)
- Create: `backend/migrations/57_mikrotik_api_protocol.sql`
- Create: `backend/internal/database/migrations_mikrotik_postgres_test.go`
- Modify: `backend/internal/services/router_service.go`, `router_connection.go`, `router_poll.go`, `router_discovery.go`
- Test: `backend/internal/services/router_service_test.go`, `router_connection_test.go`, `router_poll_test.go`, `router_discovery_test.go`

**Interfaces:**
- Consumes: Task 3 `mikrotik.Open`, `mikrotik.Config`, `mikrotik.Protocol*`.
- Produces:
  - `models.MikrotikRouter.APIProtocol string` (json `api_protocol`)
  - `services.RouterInput.APIProtocol string`, `services.ConnectionTestInput.APIProtocol string`, `services.RouterUpdate.APIProtocol *string`
  - `services.RouterAPI` gains `Close() error`
  - `RouterPollService.newAPI` becomes `func(router *models.MikrotikRouter, password string) (RouterAPI, error)`
  - `services.DefaultRouterAPIPort` is removed (use `mikrotik.Protocol.DefaultPort()`).

- [ ] **Step 1: Write the failing tests**

`internal/database/migrations_mikrotik_postgres_test.go`:

```go
package database

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func insertRouterRow(db *gorm.DB, protocolColumn bool, protocol string) (uuid.UUID, error) {
	id := uuid.New()
	columns := "id, name, host, snmp_community, snmp_port, api_username, api_password, api_port, enabled, tls_fingerprint, status, api_status, created_at, updated_at"
	values := "?, ?, '192.0.2.1', 'x', 161, 'u', 'p', 8728, true, '', 'unknown', 'unknown', now(), now()"
	args := []interface{}{id, "router-" + id.String()}
	if protocolColumn {
		columns += ", api_protocol"
		values += ", ?"
		args = append(args, protocol)
	}
	return id, db.Exec("INSERT INTO mikrotik_routers ("+columns+") VALUES ("+values+")", args...).Error
}

func TestDatabaseRefusesAnUnknownRouterProtocol(t *testing.T) {
	db := freshPostgres(t)
	for _, protocol := range []string{"rest", "api-ssl", "api"} {
		_, err := insertRouterRow(db, true, protocol)
		require.NoError(t, err, protocol)
	}
	_, err := insertRouterRow(db, true, "telnet")
	assert.Error(t, err)
}

// Routers added before the choice existed were all read over REST.
func TestRoutersWithoutAProtocolReadAsREST(t *testing.T) {
	db := freshPostgres(t)
	id, err := insertRouterRow(db, false, "")
	require.NoError(t, err)

	var protocol string
	require.NoError(t, db.Raw("SELECT api_protocol FROM mikrotik_routers WHERE id = ?", id).Scan(&protocol).Error)
	assert.Equal(t, "rest", protocol)
}
```

`internal/services/router_service_test.go` — change the existing `assert.Equal(t, DefaultRouterAPIPort, stored.APIPort)` to `assert.Equal(t, mikrotik.ProtocolREST.DefaultPort(), stored.APIPort)` and add (import `mikrotik` if not already):

```go
func TestCreateRouterDefaultsToRESTAndPinsTheFingerprint(t *testing.T) {
	service, _ := newRouterServiceForTest(t)

	router, err := service.Create(validRouterInput())

	require.NoError(t, err)
	assert.Equal(t, "rest", router.APIProtocol)
	assert.Equal(t, testFingerprint, router.TLSFingerprint)
}

func TestCreateAPIRouterNeedsNoFingerprintAndStoresNone(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	in := validRouterInput()
	in.APIProtocol, in.TLSFingerprint = "api", testFingerprint

	router, err := service.Create(in)

	require.NoError(t, err)
	assert.Equal(t, "api", router.APIProtocol)
	assert.Equal(t, 8728, router.APIPort)
	assert.Empty(t, router.TLSFingerprint, "plain api has no certificate to pin")
}

func TestCreateRouterRefusesWhatItCannotRead(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	cases := map[string]func(*RouterInput){
		"unknown protocol":           func(in *RouterInput) { in.APIProtocol = "telnet" },
		"api-ssl without fingerprint": func(in *RouterInput) { in.APIProtocol, in.TLSFingerprint = "api-ssl", "" },
		"rest without fingerprint":    func(in *RouterInput) { in.TLSFingerprint = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			in := validRouterInput()
			change(&in)
			_, err := service.Create(in)
			assert.ErrorIs(t, err, ErrValidation)
		})
	}
}

func TestUpdateRouterChangesItsProtocol(t *testing.T) {
	service, _ := newRouterServiceForTest(t)
	router, err := service.Create(validRouterInput())
	require.NoError(t, err)

	protocol := "api-ssl"
	updated, err := service.Update(router.ID, RouterUpdate{APIProtocol: &protocol})
	require.NoError(t, err)
	assert.Equal(t, "api-ssl", updated.APIProtocol)

	bad := "telnet"
	_, err = service.Update(router.ID, RouterUpdate{APIProtocol: &bad})
	assert.ErrorIs(t, err, ErrValidation)
}
```

`internal/services/router_connection_test.go` — add (imports `bufio`, `io`, `time` as needed):

```go
// plainAPIListener records the first bytes a client sends, then hangs up.
func plainAPIListener(t *testing.T) (int, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	first := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 64)
		n, _ := io.ReadAtLeast(conn, buf, 6)
		first <- string(buf[:n])
	}()
	return listener.Addr().(*net.TCPAddr).Port, first
}

// Plain api has no certificate to confirm, so the test logs in at once and
// comes back without a fingerprint.
func TestConnectionTestOverPlainAPILogsInWithoutAFingerprint(t *testing.T) {
	port, first := plainAPIListener(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIProtocol: "api", APIPort: port,
		APIUsername: "u", APIPassword: "p",
	})

	assert.Contains(t, <-first, "/login")
	assert.Empty(t, result.Fingerprint)
	assert.NotEmpty(t, result.APIError, "the listener hangs up instead of answering")
}

func TestConnectionTestOverAPISSLStartsWithTheFingerprint(t *testing.T) {
	server, hits, port := fakeRouterTLSServer(t)
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{
		Host: "127.0.0.1", SNMPPort: 1, SNMPCommunity: "x", APIProtocol: "api-ssl", APIPort: port,
		APIUsername: "u", APIPassword: "p",
	})

	assert.Equal(t, mikrotik.Fingerprint(server.Certificate().Raw), result.Fingerprint)
	assert.Empty(t, result.APIError)
	assert.Zero(t, hits.Load())
}

func TestConnectionTestRejectsAnUnknownProtocol(t *testing.T) {
	service, _ := newRouterServiceForTest(t)

	result := service.TestConnection(ConnectionTestInput{Host: "127.0.0.1", SNMPPort: 1, APIProtocol: "telnet"})

	assert.Contains(t, result.APIError, "api_protocol")
}
```

`internal/services/router_poll_test.go` — give the fake a close counter and the fixture an open error:

```go
// in fakeRouterAPI:
	closed int

func (f *fakeRouterAPI) Close() error { f.closed++; return nil }

// in pollFixture: add field
	openErr error

// in newPollFixture: replace the newAPI line with
		newAPI: func(*models.MikrotikRouter, string) (RouterAPI, error) {
			if f.openErr != nil {
				return nil, f.openErr
			}
			return f.api, nil
		},
```

and add:

```go
func TestPollBGPClosesTheConnection(t *testing.T) {
	f := newPollFixture(t)

	require.NoError(t, f.service.PollBGP(f.router, time.Now()))

	assert.Equal(t, 1, f.api.closed)
}

// Review Focus 2 and 3: a refused login or a dropped session is an API error
// on the router row, not a crash, and the next poll opens a fresh connection.
func TestPollBGPRecordsAFailedOpen(t *testing.T) {
	f := newPollFixture(t)
	f.openErr = mikrotik.ErrUnauthorized

	err := f.service.PollBGP(f.router, time.Now())

	require.ErrorIs(t, err, mikrotik.ErrUnauthorized)
	var stored models.MikrotikRouter
	require.NoError(t, f.db.First(&stored, "id = ?", f.router.ID).Error)
	assert.Equal(t, models.RouterAPIStatusError, stored.APIStatus)
	require.NotNil(t, stored.APIError)
	assert.Contains(t, *stored.APIError, "rejected")
}
```

`internal/services/router_discovery_test.go` — add (using that file's existing fixture setup; it uses `newPollFixture`):

```go
func TestDiscoverClosesTheConnectionOnError(t *testing.T) {
	f := newPollFixture(t)
	f.api.err = errors.New("interface/print: read: connection reset")

	require.Error(t, f.service.Discover(f.router))

	assert.Equal(t, 1, f.api.closed)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/services/ -run 'Router|Poll|Discover|Connection' 2>&1 | head -20`
Expected: FAIL to compile — `unknown field APIProtocol`, `fakeRouterAPI does not implement ... Close`, newAPI signature mismatch.

- [ ] **Step 3: Model and migration**

In `internal/models/mikrotik.go`, add to `MikrotikRouter` directly after `APIPort`:

```go
	// APIProtocol is how BGP and inventory are read: "rest", "api-ssl" or
	// "api". Routers added before the choice existed are REST.
	APIProtocol string `gorm:"type:varchar(16);not null;default:'rest'" json:"api_protocol"`
```

`migrations/57_mikrotik_api_protocol.sql`:

```sql
-- The protocol TikMan reads each MikroTik router with. AutoMigrate adds the
-- column from the model tags; the CHECK is what the tags cannot express.
ALTER TABLE mikrotik_routers
    ADD COLUMN IF NOT EXISTS api_protocol varchar(16) NOT NULL DEFAULT 'rest';

ALTER TABLE mikrotik_routers DROP CONSTRAINT IF EXISTS mikrotik_routers_api_protocol_check;
ALTER TABLE mikrotik_routers ADD CONSTRAINT mikrotik_routers_api_protocol_check
    CHECK (api_protocol IN ('rest', 'api-ssl', 'api'));
```

(No `.down.sql`: migrations since the numbered-`000NN` era are single forward files, like 56.)

- [ ] **Step 4: Router service — create, validate, update**

In `router_service.go`:
- Delete `DefaultRouterAPIPort` from the const block.
- Add `APIProtocol string` to `RouterInput` (after `APIPort`) and `APIProtocol *string` to `RouterUpdate`.
- Add:

```go
// routerProtocol reads a submitted or stored protocol. Empty is REST, the only
// protocol before the choice existed.
func routerProtocol(value string) mikrotik.Protocol {
	if value == "" {
		return mikrotik.ProtocolREST
	}
	return mikrotik.Protocol(value)
}

const errProtocolText = "api_protocol must be rest, api-ssl or api"
```

- `validateRouterInput`: compute `protocol := routerProtocol(in.APIProtocol)` before the switch and replace the fingerprint case with:

```go
	case !protocol.Valid():
		return fmt.Errorf("%w: %s", ErrValidation, errProtocolText)
	case protocol.UsesTLS() && !validFingerprint(in.TLSFingerprint):
		return fmt.Errorf("%w: tls_fingerprint must be the SHA-256 the connection test returned", ErrValidation)
```

- `Create`: after validation,

```go
	protocol := routerProtocol(in.APIProtocol)
	fingerprint := ""
	if protocol.UsesTLS() {
		fingerprint = strings.ToLower(in.TLSFingerprint)
	}
```

and in the struct literal use `APIProtocol: string(protocol)`, `APIPort: portOr(in.APIPort, protocol.DefaultPort())`, `TLSFingerprint: fingerprint`.

- `updateColumns`: before `setIfGiven(updates, "api_username", ...)` add

```go
	if upd.APIProtocol != nil {
		if !mikrotik.Protocol(*upd.APIProtocol).Valid() {
			return nil, fmt.Errorf("%w: %s", ErrValidation, errProtocolText)
		}
		updates["api_protocol"] = *upd.APIProtocol
	}
```

(Switching to a TLS protocol keeps the stored fingerprint. If it does not match — or is empty, coming from `api` — the next poll records `certificate_changed` and the presented fingerprint, and an Admin accepts it with the existing **Terima** button.)

- [ ] **Step 5: Connection test per protocol**

In `router_connection.go` add `APIProtocol string` to `ConnectionTestInput` and replace `TestConnection` and `readBoard` with:

```go
// TestConnection checks SNMP and the API. Over TLS it runs in two steps:
// without a fingerprint it only reads the certificate the router presents, so
// the operator can compare it with the router before any password is sent;
// with the confirmed fingerprint it pins to that certificate and reads the
// board. Plain api has no certificate, so it logs in at once.
func (s *RouterService) TestConnection(in ConnectionTestInput) ConnectionTestResult {
	var result ConnectionTestResult
	snmpPort := portOr(in.SNMPPort, DefaultRouterSNMPPort)
	if err := connectivity.SNMPTest(in.Host, snmpPort, in.SNMPCommunity, routerSNMPTestTimeout); err != nil {
		result.SNMPError = err.Error()
	}

	protocol := routerProtocol(in.APIProtocol)
	if !protocol.Valid() {
		result.APIError = errProtocolText
		return result
	}
	cfg := mikrotik.Config{Protocol: protocol, Host: in.Host, Port: portOr(in.APIPort, protocol.DefaultPort()),
		Username: in.APIUsername, Password: in.APIPassword}
	if !protocol.UsesTLS() {
		return readBoard(result, cfg)
	}
	if in.TLSFingerprint == "" {
		fingerprint, err := mikrotik.FetchFingerprint(cfg.Host, cfg.Port)
		if err != nil {
			result.APIError = err.Error()
		}
		result.Fingerprint = fingerprint
		return result
	}
	if !validFingerprint(in.TLSFingerprint) {
		result.APIError = "tls_fingerprint must be the 64-character SHA-256 the connection test returned"
		return result
	}
	cfg.Fingerprint = strings.ToLower(in.TLSFingerprint)
	result.Fingerprint = cfg.Fingerprint
	return readBoard(result, cfg)
}

func readBoard(result ConnectionTestResult, cfg mikrotik.Config) ConnectionTestResult {
	client, err := mikrotik.Open(cfg)
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	defer func() { _ = client.Close() }()
	resource, err := client.SystemResource()
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	result.BoardName, result.Version = resource.BoardName, resource.Version
	return result
}
```

Also update the `ConnectionTestResult` comment's "REST" to "the API".

- [ ] **Step 6: Poller opens per protocol and always closes**

In `router_poll.go`:
- `RouterAPI` comment: "RouterAPI is what polling reads from a router. *mikrotik.Client satisfies it; tests pass a fake." Add `Close() error` to the interface.
- Field: `newAPI func(router *models.MikrotikRouter, password string) (RouterAPI, error)`; in `NewRouterPollService` set `newAPI: openRouterAPI,` and add:

```go
// openRouterAPI returns a nil interface on failure, not a nil *Client inside a
// non-nil interface, so callers can trust err alone.
func openRouterAPI(router *models.MikrotikRouter, password string) (RouterAPI, error) {
	client, err := mikrotik.Open(mikrotik.Config{
		Protocol: routerProtocol(router.APIProtocol), Host: router.Host, Port: router.APIPort,
		Username: router.APIUsername, Password: password, Fingerprint: router.TLSFingerprint,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
```

- `apiFor`:

```go
// apiFor opens the router's API. Failing to open — refused login, wrong pin,
// unreachable — is the router's API error and is recorded as one.
func (s *RouterPollService) apiFor(router *models.MikrotikRouter) (RouterAPI, error) {
	_, password, err := s.routers.Secrets(router)
	if err != nil {
		return nil, err
	}
	api, err := s.newAPI(router, password)
	if err != nil {
		return nil, s.recordAPIError(router, err)
	}
	return api, nil
}
```

- In `PollBGP` (`router_poll.go`) and `Discover` (`router_discovery.go`), directly after the `if err != nil { return err }` that follows `s.apiFor(router)`, add `defer func() { _ = api.Close() }()`.
- `recordAPIError` comment: "stores why the API failed"; behaviour unchanged (only TLS protocols can return `ErrCertificateChanged`).

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd backend && go build ./... && go test ./internal/services/ ./internal/mikrotik/ -race -count=1`
Then with Postgres (start it with `docker start tikman-testpg` if stopped): `cd backend && TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" go test ./internal/database/ -run 'RouterProtocol|WithoutAProtocol' -race -count=1 -v`
Expected: PASS. (If `internal/database` fails on `gin_trgm_ops` or a refused `DROP TABLE ont_metrics`, that is stale test-database state: `docker exec tikman-testpg psql -U tikman -d postgres -c 'DROP DATABASE tikman_test' -c 'CREATE DATABASE tikman_test'` and rerun.)

- [ ] **Step 8: Commit**

```bash
cd backend && gofmt -s -l internal && go vet ./...
git add internal/models/mikrotik.go migrations/57_mikrotik_api_protocol.sql internal/database/migrations_mikrotik_postgres_test.go internal/services
git commit -m "feat(mikrotik): per-router protocol for create, update, connection test and polling"
```

---

### Task 5: `api_protocol` over HTTP

**Files:**
- Modify: `backend/internal/api/router_dto.go`, `backend/internal/api/router_handler.go:62,82-84`
- Test: `backend/internal/api/router_handler_test.go`

**Interfaces:**
- Consumes: Task 4 `RouterInput.APIProtocol`, `RouterUpdate.APIProtocol`, `ConnectionTestInput.APIProtocol`.
- Produces: JSON field `api_protocol` on create, update and test-connection requests; on every router response (from the model).

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateRouterOverPlainAPINeedsNoFingerprint(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	req := createRouterRequest()
	req.APIProtocol, req.TLSFingerprint = "api", ""
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", req)

	handler.Create(c)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"api_protocol":"api"`)
	assert.Contains(t, w.Body.String(), `"api_port":8728`)
}

func TestCreateRouterRefusesAnUnknownProtocol(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	req := createRouterRequest()
	req.APIProtocol = "telnet"
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateRESTRouterStillNeedsAFingerprint(t *testing.T) {
	handler, _ := setupRouterHandlerTest(t)
	req := createRouterRequest()
	req.TLSFingerprint = ""
	w, c := SetupTestContext(http.MethodPost, "/api/v1/routers", req)

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
```

Add a case to `TestTestRouterConnectionBindsItsBody`'s table: `"unknown protocol"` with `full` copied and `APIProtocol: "telnet"` (expect the table's existing 400 assertion). Add an update test following the file's existing update-test pattern (find it with `grep -n "handler.Update(c)" internal/api/router_handler_test.go`), sending `map[string]string{"api_protocol": "api-ssl"}` and asserting 200 and `"api_protocol":"api-ssl"` in the body.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/api/ -run 'Router' -v 2>&1 | tail -20`
Expected: FAIL to compile — `req.APIProtocol undefined`.

- [ ] **Step 3: DTOs and audit**

In `router_dto.go`:
- `CreateRouterRequest`: add `APIProtocol string `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"`` and change `TLSFingerprint`'s binding to `omitempty,len=64,hexadecimal` (the service requires it for TLS protocols). Update its doc comment: "TLSFingerprint is the one the connection test returned; plain api has none."
- `toInput()` passes `APIProtocol: r.APIProtocol`.
- `UpdateRouterRequest`: add `APIProtocol *string `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"``; `toUpdate()` passes it.
- `TestRouterConnectionRequest`: add `APIProtocol string `json:"api_protocol" binding:"omitempty,oneof=rest api-ssl api"``; `toInput()` passes it. Update its comment to say the fingerprint step applies to TLS protocols only.

In `router_handler.go`, add `"api_protocol": router.APIProtocol` to the `create` and `update` audit maps.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/api/ -run 'Router' -race -count=1`
Expected: PASS. (If WireGuard tests in this package fail with 400 while a `ppp0` interface is up on the Mac, that is environmental — run only `-run 'Router'`.)

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -s -l internal/api && go vet ./internal/api/
git add internal/api/router_dto.go internal/api/router_handler.go internal/api/router_handler_test.go
git commit -m "feat(api): api_protocol on router create, update and connection test"
```

---

### Task 6: Frontend — protocol field, one-step API test, wording

**Files:**
- Modify: `frontend/src/domain/entities/MikrotikRouter.ts`
- Create: `frontend/src/presentation/components/routers/routerProtocol.ts`
- Modify: `frontend/src/presentation/components/routers/routerTestSteps.ts`, `routerTestSteps.test.ts`
- Modify: `frontend/src/presentation/components/routers/useRouterTestSteps.ts`
- Modify: `frontend/src/presentation/components/routers/RouterFormFields.tsx`
- Modify: `frontend/src/presentation/components/routers/RouterFormModal.tsx`
- Create: `frontend/src/presentation/components/routers/RouterFormModal.test.tsx`
- Modify: `RouterInfoTab.tsx`, `RouterStatusTag.tsx`, `BgpSessionTable.tsx`, `frontend/src/presentation/pages/RoutersPage.tsx` (wording) and any test asserting the old wording

**Interfaces:**
- Consumes: backend JSON `api_protocol` (humps → `apiProtocol`).
- Produces: `type RouterApiProtocol = "rest" | "api-ssl" | "api"`; `PROTOCOL_DEFAULT_PORT`, `PROTOCOL_LABEL`, `usesTls()`; `savableFingerprint(protocol, first, second): string | null`; `useRouterTestSteps(form, protocol)`.

- [ ] **Step 1: Entities**

In `MikrotikRouter.ts`:

```ts
export type RouterApiProtocol = "rest" | "api-ssl" | "api";
```

Add `apiProtocol: RouterApiProtocol;` to `MikrotikRouter` (after `apiPort`), `apiProtocol?: RouterApiProtocol;` to `TestRouterConnectionDto` and `UpdateRouterDto`, and change `CreateRouterDto` to:

```ts
export interface CreateRouterDto extends TestRouterConnectionDto {
  name: string;
}
```

(its `tlsFingerprint` stays optional from the parent: plain API has none).

- [ ] **Step 2: `routerProtocol.ts`**

```ts
import type { RouterApiProtocol } from "@/domain/entities";

// Must match mikrotik.Protocol.DefaultPort on the backend.
export const PROTOCOL_DEFAULT_PORT: Record<RouterApiProtocol, number> = {
  rest: 8443,
  "api-ssl": 8729,
  api: 8728,
};

export const PROTOCOL_LABEL: Record<RouterApiProtocol, string> = {
  rest: "REST",
  "api-ssl": "API-SSL",
  api: "API",
};

/** Plain API has no certificate, so there is nothing to confirm or pin. */
export function usesTls(protocol: RouterApiProtocol): boolean {
  return protocol !== "api";
}
```

- [ ] **Step 3: Write the failing tests for `savableFingerprint`**

Replace `routerTestSteps.test.ts` with (keeping the existing REST cases, now with a protocol argument):

```ts
import { describe, expect, it } from "vitest";
import { savableFingerprint } from "./routerTestSteps";

const FP = "AA:BB:CC";

describe("savableFingerprint over TLS", () => {
  it("is null before the fingerprint has been confirmed", () => {
    expect(savableFingerprint("rest", { fingerprint: FP }, null)).toBeNull();
    expect(savableFingerprint("rest", null, null)).toBeNull();
  });

  it("returns the step-1 fingerprint once step 2 reached the API and SNMP", () => {
    expect(
      savableFingerprint(
        "api-ssl",
        { fingerprint: FP },
        { fingerprint: FP, boardName: "CCR2004", version: "7.15" },
      ),
    ).toBe(FP);
  });

  it("stays null when step 2 reports an API error", () => {
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP },
        { fingerprint: FP, apiError: "401 unauthorized" },
      ),
    ).toBeNull();
  });

  it("stays null when SNMP failed in either step", () => {
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP, snmpError: "timeout" },
        { fingerprint: FP, boardName: "CCR2004" },
      ),
    ).toBeNull();
    expect(
      savableFingerprint(
        "rest",
        { fingerprint: FP },
        { fingerprint: FP, snmpError: "timeout", boardName: "CCR2004" },
      ),
    ).toBeNull();
  });
});

describe("savableFingerprint over plain API", () => {
  it("is savable with no fingerprint after one passing step", () => {
    expect(
      savableFingerprint("api", { boardName: "CCR2004", version: "7.24" }, null),
    ).toBe("");
  });

  it("stays null when the single step failed", () => {
    expect(
      savableFingerprint("api", { apiError: "router rejected" }, null),
    ).toBeNull();
    expect(
      savableFingerprint("api", { snmpError: "timeout", boardName: "x" }, null),
    ).toBeNull();
    expect(savableFingerprint("api", null, null)).toBeNull();
  });
});
```

Run: `cd frontend && npm test -- --run src/presentation/components/routers/routerTestSteps.test.ts`
Expected: FAIL — `savableFingerprint` is not exported.

- [ ] **Step 4: Implement `savableFingerprint`**

In `routerTestSteps.ts` replace `confirmedFingerprint` and its comment with:

```ts
/**
 * What a new router may be saved with, or null while saving is not yet safe.
 * Over TLS the backend sends credentials only to a certificate the operator
 * confirmed, so step 2, run against the step-1 fingerprint, must have reached
 * both SNMP and the API. Plain API has no certificate: its single step is the
 * whole test, and the router is saved with no fingerprint ("").
 */
export function savableFingerprint(
  protocol: RouterApiProtocol,
  first: RouterConnectionTestResult | null,
  second: RouterConnectionTestResult | null,
): string | null {
  if (!first || first.snmpError) return null;
  if (!usesTls(protocol)) return first.apiError ? null : "";
  if (!first.fingerprint) return null;
  if (!second || second.snmpError || second.apiError) return null;
  return first.fingerprint;
}
```

(import `RouterApiProtocol` from `@/domain/entities` and `usesTls` from `./routerProtocol`). Rerun the test: PASS.

- [ ] **Step 5: `useRouterTestSteps` takes the protocol**

- Add `apiProtocol: RouterApiProtocol;` to `RouterFormValues`.
- Signature `export function useRouterTestSteps(form: FormInstance<RouterFormValues>, protocol: RouterApiProtocol)`.
- Return `fingerprint: savableFingerprint(protocol, first, second),` instead of `confirmedFingerprint(first, second)`.
- Doc comment: "The connection test of a new router: over TLS, step 1 reads the certificate and step 2 sends the password only to the fingerprint step 1 showed; plain API is a single step."

- [ ] **Step 6: Form fields**

`RouterFormFields.tsx` — add imports `Alert, Radio` from antd and `RouterApiProtocol` type; add the protocol field as the first API field (before `apiUsername`) and relabel the API fields:

```tsx
const PROTOCOL_OPTIONS = [
  { label: "REST (www-ssl) — disarankan", value: "rest" },
  { label: "API-SSL", value: "api-ssl" },
  { label: "API (tanpa enkripsi)", value: "api" },
];

// The notice follows the chosen protocol, so it reads the form it sits in.
function ProtocolNotice() {
  const protocol = Form.useWatch<RouterApiProtocol | undefined>("apiProtocol");
  if (protocol === "api") {
    return (
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Password dikirim tanpa enkripsi"
        description="Username dan password user read-only melewati internet apa adanya. Batasi service api di router hanya untuk IP VPS: /ip service set api address=IP_VPS/32"
      />
    );
  }
  if (protocol === "api-ssl") {
    return (
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="API-SSL butuh sertifikat di router"
        description="Sertifikat service api-ssl tidak boleh none: tanpa sertifikat RouterOS hanya menawarkan cipher anonim yang ditolak TikMan."
      />
    );
  }
  return null;
}
```

JSX additions, before the `apiUsername` item:

```tsx
      <Form.Item name="apiProtocol" label="Protokol">
        <Radio.Group optionType="button" options={PROTOCOL_OPTIONS} />
      </Form.Item>
      <ProtocolNotice />
```

Relabel: `"User REST (read-only)"` → `"User API (read-only)"`, `"Password REST"` → `"Password API"`, `"Port REST (www-ssl)"` → `"Port API"`.

- [ ] **Step 7: Write the failing modal test**

`RouterFormModal.test.tsx`:

```tsx
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RouterFormModal } from "./RouterFormModal";

const testMutate = vi.fn();
const createMutate = vi.fn();

vi.mock("@/application/hooks/useMikrotikRouters", () => ({
  useTestRouterConnection: () => ({ mutateAsync: testMutate, isPending: false }),
  useCreateMikrotikRouter: () => ({ mutateAsync: createMutate, isPending: false }),
  useUpdateMikrotikRouter: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

function portInput(): HTMLInputElement {
  return screen.getByLabelText("Port API") as HTMLInputElement;
}

function fillRequired() {
  fireEvent.change(screen.getByLabelText("Nama"), { target: { value: "Border" } });
  fireEvent.change(screen.getByLabelText("IP atau hostname"), { target: { value: "192.0.2.1" } });
  fireEvent.change(screen.getByLabelText("SNMP community"), { target: { value: "c" } });
  fireEvent.change(screen.getByLabelText("User API (read-only)"), { target: { value: "tikman" } });
  fireEvent.change(screen.getByLabelText("Password API"), { target: { value: "p" } });
}

describe("RouterFormModal protocol", () => {
  beforeEach(() => {
    testMutate.mockReset();
    createMutate.mockReset();
  });

  it("moves the port to the protocol's default", async () => {
    render(<RouterFormModal onClose={vi.fn()} />);
    expect(portInput().value).toBe("8443");

    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));

    await waitFor(() => expect(portInput().value).toBe("8728"));
    expect(screen.getByText("Password dikirim tanpa enkripsi")).toBeTruthy();
  });

  it("keeps a port the operator typed", async () => {
    render(<RouterFormModal onClose={vi.fn()} />);
    fireEvent.change(portInput(), { target: { value: "65489" } });

    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));

    await waitFor(() => expect(portInput().value).toBe("65489"));
  });

  it("saves a plain-API router after one passing test, without a fingerprint", async () => {
    testMutate.mockResolvedValue({ boardName: "CCR2116", version: "7.24.2" });
    createMutate.mockResolvedValue({});
    render(<RouterFormModal onClose={vi.fn()} />);
    fireEvent.click(screen.getByText("API (tanpa enkripsi)"));
    fillRequired();

    fireEvent.click(screen.getByText("Uji koneksi"));
    await screen.findByText(/CCR2116/);
    fireEvent.click(screen.getByText("Simpan"));

    await waitFor(() => expect(createMutate).toHaveBeenCalled());
    expect(createMutate.mock.calls[0][0]).toMatchObject({ apiProtocol: "api" });
    expect(createMutate.mock.calls[0][0].tlsFingerprint).toBeUndefined();
  });
});
```

Run: `cd frontend && npm test -- --run src/presentation/components/routers/RouterFormModal.test.tsx`
Expected: FAIL (no protocol field / labels). If the repo's Vitest setup needs a wrapper for antd (e.g. `window.matchMedia`), it is already in `src/test/setup` — check `vitest.config` `setupFiles` before adding anything.

- [ ] **Step 8: Modal**

In `RouterFormModal.tsx`:
- Remove `DEFAULT_API_PORT`; import `PROTOCOL_DEFAULT_PORT`, `PROTOCOL_LABEL`, `usesTls` from `./routerProtocol` and `RouterApiProtocol` from `@/domain/entities`.
- `initialValues`:

```ts
function initialValues(router?: MikrotikRouter): Partial<RouterFormValues> {
  const apiProtocol = router?.apiProtocol ?? "rest";
  return {
    name: router?.name,
    host: router?.host,
    snmpPort: router?.snmpPort ?? DEFAULT_SNMP_PORT,
    apiUsername: router?.apiUsername,
    apiProtocol,
    apiPort: router?.apiPort ?? PROTOCOL_DEFAULT_PORT[apiProtocol],
  };
}
```

- In `RouterFormModal`:

```tsx
  const protocol: RouterApiProtocol =
    Form.useWatch("apiProtocol", form) ?? router?.apiProtocol ?? "rest";
  const steps = useRouterTestSteps(form, protocol);
  ...
  // A protocol change moves the port to that protocol's default, unless the
  // operator already typed one.
  const onValuesChange = (changed: Partial<RouterFormValues>) => {
    if (changed.apiProtocol && !form.isFieldTouched("apiPort")) {
      form.setFieldsValue({ apiPort: PROTOCOL_DEFAULT_PORT[changed.apiProtocol] });
    }
    if (!editing) steps.reset();
  };
```

and pass `onValuesChange={onValuesChange}` to `<Form>`; `disabled={!editing && steps.fingerprint === null}` on Simpan; `<TestStepResults steps={steps} protocol={protocol} />`.
- `useSaveRouter`: `} else if (fingerprint !== null) { await create.mutateAsync({ ...values, tlsFingerprint: fingerprint || undefined }); }`.
- Rename `RestStep` → `ApiStep({ result, label })`, messages `${label}: ${result.apiError}` / `${label}: ${result.boardName} · RouterOS ${result.version}`.
- `FingerprintStep` gets a `label` prop: description "Cocokkan dengan hasil `/certificate print detail` di router. Password {label} baru dikirim setelah Anda mengonfirmasi."; button "Fingerprint cocok, uji {label}"; error message `${label}: ${result.apiError ?? "sertifikat tidak terbaca"}`.
- `TestStepResults({ steps, protocol })`:

```tsx
  const label = PROTOCOL_LABEL[protocol];
  const { first, second } = steps;
  if (!first) return null;
  if (!usesTls(protocol)) {
    return (
      <Space direction="vertical" style={{ width: "100%" }}>
        {snmpAlert(first)}
        <ApiStep result={first} label={label} />
      </Space>
    );
  }
```

followed by the existing two-step branches with `label` passed through. The comment above `FingerprintStep` becomes: "Step 1 over TLS only reads the certificate: it must not claim the API worked."

Run the modal test and `routerTestSteps.test.ts`: PASS.

- [ ] **Step 9: Show the protocol and neutral wording**

- `RouterInfoTab.tsx`: add `<Descriptions.Item label="Protokol">{PROTOCOL_LABEL[router.apiProtocol]} · port {router.apiPort}</Descriptions.Item>` next to the existing API items; "Sertifikat REST router berubah" → "Sertifikat API router berubah"; "Error REST terakhir" → "Error API terakhir".
- `RouterStatusTag.tsx`: `"REST error"` → `"API error"`.
- `BgpSessionTable.tsx`: `— REST sedang gagal` → `— API router sedang gagal`.
- `pages/RoutersPage.tsx`: `"REST sedang gagal; jumlah sesi terakhir sudah usang"` → `"API router sedang gagal; jumlah sesi terakhir sudah usang"`.
- Find tests that assert the old strings: `grep -rn "REST" frontend/src --include='*.test.ts*'` and update their expectations to the new wording (behaviour unchanged).
- Any test fixture building a `MikrotikRouter` object now needs `apiProtocol: "rest"` (TypeScript will list them in `npm run build`).

- [ ] **Step 10: Run the full frontend gates**

Run: `cd frontend && npm test -- --run && npm run lint && npm run format:check && npm run build`
Expected: all tests pass, 0 lint errors (the 4 pre-existing warnings in `components/cs` may remain), format clean, build succeeds.

- [ ] **Step 11: Commit**

```bash
cd frontend && npx prettier --write src/presentation/components/routers src/domain/entities/MikrotikRouter.ts src/presentation/pages/RoutersPage.tsx
git add src
git commit -m "feat(frontend): choose REST, API-SSL or API per MikroTik router"
```

---

### Task 7: Operator guide and whole-branch verification

**Files:**
- Modify: `docs/operator_guide.md` (section "Memantau router MikroTik border", ~line 327)

- [ ] **Step 1: Document the three protocols**

Change the section's opening sentence to say BGP/interface/CPU data is read "lewat REST API, API-SSL, atau API RouterOS v7 (dipilih per router)". After the existing command block and its paragraph, add:

````markdown
### Memilih protokol

Di form router, **Protokol** menentukan cara TikMan membaca BGP, daftar
interface, dan CPU/RAM. SNMP tetap dipakai untuk trafik di ketiganya.

| Protokol | Service router | Port default | Sertifikat |
|---|---|---|---|
| REST (disarankan) | `www-ssl` | 8443 | wajib, fingerprint dicocokkan |
| API-SSL | `api-ssl` | 8729 | wajib, fingerprint dicocokkan |
| API | `api` | 8728 | tidak ada — password lewat tanpa enkripsi |

Untuk **API-SSL**, pakai sertifikat yang sama seperti REST:

```
/ip service set api-ssl certificate=tikman-rest address=IP_VPS/32 disabled=no
```

`api-ssl` dengan `certificate=none` tidak bisa dipakai: RouterOS lalu hanya
menawarkan cipher anonim, yang ditolak TikMan karena tidak melindungi dari
penyadapan.

Untuk **API**, cukup batasi service-nya ke VPS:

```
/ip service set api address=IP_VPS/32 disabled=no
```

API mengirim username dan password tanpa enkripsi, jadi siapa pun di jalur
antara VPS dan router bisa membacanya. Pakai hanya bila REST atau API-SSL
tidak memungkinkan, dan tetap dengan user grup `tikman-ro` yang read-only.
Uji koneksi untuk API hanya satu langkah karena tidak ada sertifikat yang
dicocokkan.

Mengganti protokol router yang sudah ada dari API ke REST atau API-SSL membuat
TikMan menampilkan fingerprint sertifikat di tab Router pada poll berikutnya;
Admin menerimanya dengan **Terima** setelah mencocokkannya di router.
````

In "Menambah router", change step 1's first sentence to "Klik **Uji koneksi**. Untuk REST dan API-SSL, langkah ini hanya menguji SNMP dan menampilkan fingerprint sertifikat router." and add after the numbered list: "Untuk API, uji koneksi langsung login dan membaca board dalam satu langkah."

- [ ] **Step 2: Public-repo check**

Run: `cd .. && git diff main -- . | grep -oE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' | grep -vE '^(192\.0\.2|198\.51\.100|203\.0\.113|127\.0\.0)\.' | sort -u`
Expected: no output — every address in the change is a documentation or loopback address. Also read the diff for router names, AS numbers or community strings that are not obviously fictional; the controller knows the real ones and checks them outside the repository.

- [ ] **Step 3: Full verification**

```bash
cd backend && gofmt -s -l . && go vet ./... && go mod verify && \
  TEST_POSTGRES_DSN="postgres://tikman:testpw@localhost:5439/tikman_test?sslmode=disable" go test ./... -race -count=1
cd ../frontend && npm test -- --run && npm run lint && npm run format:check && npm run build
```

Expected: gofmt prints nothing; every Go package `ok`; frontend all green.

- [ ] **Step 4: Commit**

```bash
git add docs/operator_guide.md
git commit -m "docs: choosing REST, API-SSL or API for a MikroTik router"
```

- [ ] **Step 5: Post-deploy check (operator, not automated)**

After deploy: add the border router with **API** on its `api` port; the single test step shows the board and RouterOS version; within two minutes the Upstream & BGP tab shows the BGP sessions, proving `/routing/bgp/session/print` returns the same property names REST does on the real router.
