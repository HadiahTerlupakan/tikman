package linkpreview

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	// maxRedirects is low on purpose. A preview is worth one or two hops, and
	// each hop is another address the guard has to clear.
	maxRedirects = 3
	// dialTimeout and requestTimeout keep a slow or hostile host from holding
	// a send. The whole preview is abandoned rather than the message delayed.
	dialTimeout    = 3 * time.Second
	requestTimeout = 5 * time.Second
)

// checkRedirect refuses a hop the guard would not have allowed as the first
// request. A public hostname redirecting to an internal one is exactly how a
// guard that only inspects the typed URL is walked past.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("too many redirects")
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("refused redirect to scheme %q", req.URL.Scheme)
	}
	if ip := net.ParseIP(req.URL.Hostname()); ip != nil && blockedIP(ip) {
		return fmt.Errorf("refused redirect to %s", req.URL.Hostname())
	}
	return nil
}

// guardedDial resolves the host itself and refuses every address it gets back
// before connecting. Checking the name instead would leave the resolution to
// the dialler, where a hostname pointing at 169.254.169.254 slips through.
func guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no address for %s", host)
	}
	// Every answer must clear it, not merely the first: a host that resolves
	// to one public and one internal address would otherwise be a coin toss.
	for _, ip := range ips {
		if blockedIP(ip.IP) {
			return nil, fmt.Errorf("refused address %s for %s", ip.IP, host)
		}
	}
	d := net.Dialer{Timeout: dialTimeout}
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func client() *http.Client {
	return &http.Client{
		Timeout:       requestTimeout,
		CheckRedirect: checkRedirect,
		Transport:     &http.Transport{DialContext: guardedDial},
	}
}

// get fetches a URL the CS typed, refusing anything that is not plain web
// traffic before a connection is opened.
func get(ctx context.Context, raw string, limit int64) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("refused scheme %q", u.Scheme)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	// Some sites serve a preview only to something that looks like a browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TikManBot/1.0)")

	return readCappedWith(client(), req, limit)
}

// readCapped is the seam the size test drives, with a caller-supplied client.
func readCapped(c *http.Client, raw string, limit int64) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	return readCappedWith(c, req, limit)
}

func readCappedWith(c *http.Client, req *http.Request, limit int64) ([]byte, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	// LimitReader rather than trusting Content-Length: a hostile server can
	// declare one size and send another.
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}
