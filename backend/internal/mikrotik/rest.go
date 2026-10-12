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
	target := t.baseURL + path
	if len(where) > 0 {
		query, err := restFilter(where)
		if err != nil {
			return err
		}
		target += "?" + query
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
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

func restFilter(where []string) (string, error) {
	query := url.Values{}
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
