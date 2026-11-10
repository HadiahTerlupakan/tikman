package librenms

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "test-token"

// transparentGraph mimics LibreNMS's output: a transparent canvas with one
// black legend pixel.
func transparentGraph(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	img.Set(1, 1, color.NRGBA{A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestDevicesSendsTokenAndDecodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, testToken, r.Header.Get("X-Auth-Token"))
		assert.Equal(t, "/api/v0/devices", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`{"status":"ok","devices":[{"device_id":7,"hostname":"10.0.0.1","sysName":"core","status":1}]}`))
	}))
	defer server.Close()

	devices, err := New(server.URL+"/", testToken).Devices(context.Background())
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, Device{DeviceID: 7, Hostname: "10.0.0.1", SysName: "core"}, devices[0])
}

func TestPortsAsksForTheColumnsItReads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v0/devices/7/ports", r.URL.Path)
		assert.Equal(t, "port_id,ifName,ifAlias,ifOperStatus", r.URL.Query().Get("columns"))
		_, _ = w.Write([]byte(`{"status":"ok","ports":[{"port_id":3,"ifName":"ether1","ifAlias":"Uplink","ifOperStatus":"up"}]}`))
	}))
	defer server.Close()

	ports, err := New(server.URL, testToken).Ports(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, []Port{{PortID: 3, IfName: "ether1", IfAlias: "Uplink", IfOperStatus: "up"}}, ports)
}

func TestPortGraphRequestsAPNGOfTheWindow(t *testing.T) {
	from := time.Unix(1_700_000_000, 0)
	to := from.Add(24 * time.Hour)
	graph := transparentGraph(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, testToken, r.Header.Get("X-Auth-Token"))
		assert.Equal(t, "/api/v0/devices/7/ports/GigabitEthernet%200%2F1/port_bits", r.URL.EscapedPath())
		query := r.URL.Query()
		assert.Equal(t, "png", query.Get("graph_type"))
		assert.Equal(t, "1700000000", query.Get("from"))
		assert.Equal(t, "1700086400", query.Get("to"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(graph)
	}))
	defer server.Close()

	out, err := New(server.URL, testToken).PortGraph(context.Background(), 7, "GigabitEthernet 0/1", from, to)
	require.NoError(t, err)

	img, err := png.Decode(bytes.NewReader(out))
	require.NoError(t, err)
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			assert.Equal(t, uint32(0xffff), a, "pixel %d,%d is not opaque", x, y)
		}
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	assert.Equal(t, [3]uint32{0xffff, 0xffff, 0xffff}, [3]uint32{r, g, b}, "transparent became white")
	r, g, b, _ = img.At(1, 1).RGBA()
	assert.Equal(t, [3]uint32{0, 0, 0}, [3]uint32{r, g, b}, "legend stays black")
}

func TestPortGraphRefusesSVG(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte("<svg/>"))
	}))
	defer server.Close()

	_, err := New(server.URL, testToken).PortGraph(context.Background(), 1, "ether1", time.Now(), time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bukan gambar PNG")
}

func TestUnauthorizedIsTheTokenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthenticated."}`))
	}))
	defer server.Close()

	_, err := New(server.URL, "wrong").Devices(context.Background())
	assert.True(t, errors.Is(err, ErrUnauthorized))
}

func TestOtherStatusNamesTheCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := New(server.URL, testToken).Ports(context.Background(), 1)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "HTTP 500"))
}

// Go forwards custom headers to a redirect's target whatever its host, so a
// followed redirect would hand the token to whoever the Location names.
func TestRedirectIsNotFollowedWithTheToken(t *testing.T) {
	var leaked bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("X-Auth-Token") != ""
		_, _ = w.Write([]byte(`{"status":"ok","devices":[]}`))
	}))
	defer elsewhere.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	defer server.Close()

	_, err := New(server.URL, testToken).Devices(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "302")
	assert.False(t, leaked, "the token reached the redirect's target")
}
