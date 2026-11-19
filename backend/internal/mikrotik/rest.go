package mikrotik

import (
	"bytes"
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
	"net/url"
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

// List is GET /rest<path>, with each where as a query filter the router
// applies itself.
func (t *restTransport) List(path string, out interface{}, where ...string) error {
	return t.ListProps(path, "", out, where...)
}

// ListProps is List asking the router for only the named, comma-separated
// properties, which keeps a large menu's reply small.
func (t *restTransport) ListProps(path, props string, out interface{}, where ...string) error {
	query, err := restFilter(where, props)
	if err != nil {
		return err
	}
	target := path
	if query != "" {
		target += "?" + query
	}
	return t.do(http.MethodGet, path, target, nil, out)
}

// Count is POST /rest<path>/print with count-only, so the router counts rows
// itself instead of sending them.
func (t *restTransport) Count(path string, where ...string) (int, error) {
	for _, w := range where {
		if _, _, err := splitFilter(w); err != nil {
			return 0, err
		}
	}
	body := map[string]interface{}{"count-only": ""}
	if len(where) > 0 {
		body[".query"] = where
	}
	var raw json.RawMessage
	if err := t.do(http.MethodPost, path+"/print", path+"/print", body, &raw); err != nil {
		return 0, err
	}
	return countFromJSON(raw)
}

// Command is POST /rest<path> with each arg as a body property.
func (t *restTransport) Command(path string, out interface{}, args ...string) error {
	body := map[string]string{}
	for _, a := range args {
		key, value, err := splitFilter(a)
		if err != nil {
			return err
		}
		body[key] = value
	}
	var raw json.RawMessage
	if err := t.do(http.MethodPost, path, path, body, &raw); err != nil {
		return err
	}
	return decodeFirstJSON(raw, out)
}

// do sends one request; label names the call in errors, target is the path
// and query under /rest.
func (t *restTransport) do(method, label, target string, body interface{}, out interface{}) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, t.baseURL+target, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(t.username, t.password)

	resp, err := t.http.Do(req)
	if err != nil {
		if errors.Is(err, ErrCertificateChanged) {
			return ErrCertificateChanged
		}
		return fmt.Errorf("%s %s: %w", method, label, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return fmt.Errorf("%s %s: HTTP %d: %s", method, label, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

// Close has nothing to release: every request already closed its connection.
func (t *restTransport) Close() error { return nil }

// pinnedTLS trusts exactly one certificate. The router's is self-signed, so
// there is no CA to verify against; the pin replaces that check rather than
// dropping it.
func pinnedTLS(fingerprint string) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // verification is VerifyPeerCertificate's pin below
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 || Fingerprint(raw[0]) != strings.ToLower(fingerprint) {
				return ErrCertificateChanged
			}
			return nil
		},
	}
}

// Fingerprint is the lowercase hex SHA-256 of a DER certificate, the form the
// operator compares against Winbox.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

func restFilter(where []string, props string) (string, error) {
	query := url.Values{}
	if props != "" {
		query.Set(".proplist", props)
	}
	for _, w := range where {
		key, value, err := splitFilter(w)
		if err != nil {
			return "", err
		}
		query.Set(key, value)
	}
	return query.Encode(), nil
}

// FetchFingerprint reads the certificate a router presents, for an operator to
// confirm before it is pinned. Nothing is sent over the connection.
func FetchFingerprint(host string, port int) (string, error) {
	dialer := &net.Dialer{Timeout: requestTimeout}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := tls.DialWithDialer(dialer, "tcp", address,
		&tls.Config{InsecureSkipVerify: true}) //nolint:gosec // reading the certificate is the point
	if err != nil {
		return "", fmt.Errorf("TLS handshake with %s: %w", address, err)
	}
	defer func() { _ = conn.Close() }()

	certificates := conn.ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return "", fmt.Errorf("%s presented no certificate", address)
	}
	return Fingerprint(certificates[0].Raw), nil
}
