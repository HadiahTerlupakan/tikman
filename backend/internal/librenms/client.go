// Package librenms reads devices, ports and port graphs from a LibreNMS
// server's API.
package librenms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// requestTimeout covers a graph render, which LibreNMS does on demand and
// which takes seconds on a busy server.
const requestTimeout = 30 * time.Second

// maxBodyBytes bounds what one answer may weigh; a 900x300 graph is tens of
// kilobytes, so anything near this is not a graph.
const maxBodyBytes = 16 << 20

// Graph dimensions: wide enough for WhatsApp to show the legend legibly
// without the viewer zooming.
const (
	graphWidth  = "900"
	graphHeight = "300"
)

const statusOK = "ok"

// ErrUnauthorized is LibreNMS refusing the token.
var ErrUnauthorized = errors.New("token LibreNMS ditolak")

// Device is one device LibreNMS monitors.
type Device struct {
	DeviceID int    `json:"device_id"`
	Hostname string `json:"hostname"`
	SysName  string `json:"sysName"`
}

// Port is one interface of a device.
type Port struct {
	PortID       int    `json:"port_id"`
	IfName       string `json:"ifName"`
	IfAlias      string `json:"ifAlias"`
	IfOperStatus string `json:"ifOperStatus"`
}

// Client talks to one LibreNMS server.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New constructs a client for the server at baseURL.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		// Go copies custom headers such as X-Auth-Token to a redirect's target
		// whatever its host, so following one could hand the token to another
		// server. A redirect lands in the non-200 branch for the admin to fix
		// the URL instead.
		http: &http.Client{
			Timeout:       requestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Devices lists every device.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var answer struct {
		Status  string   `json:"status"`
		Message string   `json:"message"`
		Devices []Device `json:"devices"`
	}
	if err := c.getJSON(ctx, "/devices?type=all", &answer); err != nil {
		return nil, err
	}
	if answer.Status != statusOK {
		return nil, fmt.Errorf("LibreNMS menolak permintaan perangkat: %s", answer.Message)
	}
	return answer.Devices, nil
}

// Ports lists one device's interfaces.
func (c *Client) Ports(ctx context.Context, deviceID int) ([]Port, error) {
	var answer struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Ports   []Port `json:"ports"`
	}
	path := "/devices/" + strconv.Itoa(deviceID) + "/ports?columns=port_id,ifName,ifAlias,ifOperStatus"
	if err := c.getJSON(ctx, path, &answer); err != nil {
		return nil, err
	}
	if answer.Status != statusOK {
		return nil, fmt.Errorf("LibreNMS menolak permintaan port: %s", answer.Message)
	}
	return answer.Ports, nil
}

// PortGraph renders a port's traffic between from and to as a PNG on an
// opaque white background.
func (c *Client) PortGraph(ctx context.Context, deviceID int, ifName string, from, to time.Time) ([]byte, error) {
	query := url.Values{
		"from":       {strconv.FormatInt(from.Unix(), 10)},
		"to":         {strconv.FormatInt(to.Unix(), 10)},
		"width":      {graphWidth},
		"height":     {graphHeight},
		"graph_type": {"png"},
	}
	// ifNames carry spaces and slashes ("GigabitEthernet 0/1"); PathEscape
	// keeps the slash from splitting the path segment.
	path := "/devices/" + strconv.Itoa(deviceID) + "/ports/" + url.PathEscape(ifName) +
		"/port_bits?" + query.Encode()
	body, contentType, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(contentType, "image/png") {
		return nil, fmt.Errorf("LibreNMS mengirim %q, bukan gambar PNG", contentType)
	}
	return flattenPNG(body)
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	body, _, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("jawaban LibreNMS tidak dapat dibaca: %w", err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v0"+path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("URL LibreNMS tidak valid: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("LibreNMS tidak dapat dihubungi: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, "", fmt.Errorf("jawaban LibreNMS terputus: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, "", ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		return nil, "", fmt.Errorf("LibreNMS menjawab HTTP %d", resp.StatusCode)
	}
	return body, resp.Header.Get("Content-Type"), nil
}
