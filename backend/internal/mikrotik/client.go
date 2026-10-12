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
	"time"
)

// requestTimeout bounds one REST call. The router answers these in tens of
// milliseconds; anything near this long means it is unreachable, and the job
// should give up rather than hold a worker.
const requestTimeout = 5 * time.Second

// errorBodyLimit is how much of a failed response is kept for the message.
const errorBodyLimit = 512

var (
	// ErrCertificateChanged means the router presented a certificate other
	// than the one pinned when it was added.
	ErrCertificateChanged = errors.New("router certificate changed since it was trusted")
	// ErrUnauthorized means the router rejected the username or password.
	ErrUnauthorized = errors.New("router rejected the API credentials")
)

// Client reads one router's REST API.
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewClient builds a client that only talks to a router presenting the
// certificate with the given SHA-256 fingerprint.
func NewClient(host string, port int, username, password, fingerprint string) *Client {
	return &Client{
		baseURL:  "https://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/rest",
		username: username,
		password: password,
		http: &http.Client{
			Timeout: requestTimeout,
			// A client lives for one poll. A kept-alive connection would outlive
			// it with nothing left to close it, so each request closes its own.
			Transport: &http.Transport{
				TLSClientConfig:   pinnedTLS(fingerprint),
				DisableKeepAlives: true,
			},
		},
	}
}

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

func (c *Client) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.http.Do(req)
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

// SystemResource reads uptime, version, CPU load, memory and board name.
func (c *Client) SystemResource() (SystemResource, error) {
	var out SystemResource
	return out, c.get("/system/resource", &out)
}

// Clock reads the router's clock settings.
func (c *Client) Clock() (Clock, error) {
	var out Clock
	return out, c.get("/system/clock", &out)
}

// Interfaces lists every interface, dynamic ones included.
func (c *Client) Interfaces() ([]Interface, error) {
	var out []Interface
	return out, c.get("/interface", &out)
}

// VLANs lists VLAN interfaces with their ID and parent.
func (c *Client) VLANs() ([]VLAN, error) {
	var out []VLAN
	return out, c.get("/interface/vlan", &out)
}

// Addresses lists IPv4 addresses and the interface each sits on.
func (c *Client) Addresses() ([]Address, error) {
	var out []Address
	return out, c.get("/ip/address", &out)
}

// BGPConnections lists configured BGP peers.
func (c *Client) BGPConnections() ([]BGPConnection, error) {
	var out []BGPConnection
	return out, c.get("/routing/bgp/connection", &out)
}

// BGPSessions lists running BGP sessions.
func (c *Client) BGPSessions() ([]BGPSession, error) {
	var out []BGPSession
	return out, c.get("/routing/bgp/session", &out)
}
