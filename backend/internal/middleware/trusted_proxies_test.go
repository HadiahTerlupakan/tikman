package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiContainerRoutes is the api container's table: the compose network
// 172.18.0.0/16 behind gateway 172.18.0.1 on eth0, and on wg0 the tunnel
// 10.88.0.0/24 plus a site subnet 172.20.0.0/24 that also falls in 172.16/12.
const apiContainerRoutes = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	010012AC	0003	0	0	0	00000000	0	0	0
eth0	000012AC	00000000	0001	0	0	0	0000FFFF	0	0	0
wg0	0000580A	00000000	0001	0	0	0	00FFFFFF	0	0	0
wg0	000014AC	00000000	0001	0	0	0	00FFFFFF	0	0	0
`

const forgedClient = "203.0.113.9"

func clientIPFrom(t *testing.T, trusted []string, peer string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	require.NoError(t, engine.SetTrustedProxies(trusted))
	var clientIP string
	engine.GET("/", func(c *gin.Context) { clientIP = c.ClientIP() })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = net.JoinHostPort(peer, "40000")
	req.Header.Set("X-Forwarded-For", forgedClient)
	engine.ServeHTTP(httptest.NewRecorder(), req)
	return clientIP
}

func TestTrustedProxies_OnlyComposeNetworkMayForward(t *testing.T) {
	trusted, err := trustedProxiesFromRoutes(strings.NewReader(apiContainerRoutes))
	require.NoError(t, err)

	tests := []struct {
		name string
		peer string
		want string
	}{
		{"nginx on the compose network", "172.18.0.5", forgedClient},
		{"far end of the compose network", "172.18.255.254", forgedClient},
		{"docker-proxy relaying from the gateway", "172.18.0.1", "172.18.0.1"},
		{"host on a WireGuard site subnet", "172.20.0.7", "172.20.0.7"},
		{"WireGuard tunnel peer", "10.88.0.2", "10.88.0.2"},
		{"another docker network", "172.17.0.3", "172.17.0.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, clientIPFrom(t, trusted, tt.peer))
		})
	}
}

func TestTrustedProxies_CoverSubnetExceptGateway(t *testing.T) {
	trusted, err := trustedProxiesFromRoutes(strings.NewReader(apiContainerRoutes))
	require.NoError(t, err)

	blocks := make([]netip.Prefix, 0, len(trusted))
	for _, entry := range trusted {
		blocks = append(blocks, netip.MustParsePrefix(entry))
	}
	subnet := netip.MustParsePrefix("172.18.0.0/16")
	gateway := netip.MustParseAddr("172.18.0.1")
	for addr := subnet.Addr(); subnet.Contains(addr); addr = addr.Next() {
		covered := 0
		for _, block := range blocks {
			if block.Contains(addr) {
				covered++
			}
		}
		if addr == gateway {
			require.Zero(t, covered, "gateway %s must not be trusted", addr)
			continue
		}
		require.Equal(t, 1, covered, "%s must be in exactly one trusted block", addr)
	}
}

func TestTrustedProxies_RefusesToTrust(t *testing.T) {
	tests := []struct {
		name   string
		routes string
	}{
		{
			name: "default route through a LAN outside 172.16.0.0/12",
			routes: `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0101A8C0	0003	0	0	0	00000000	0	0	0
eth0	0001A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`,
		},
		{
			name: "no default route",
			routes: `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	000012AC	00000000	0001	0	0	0	0000FFFF	0	0	0
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trusted, err := trustedProxiesFromRoutes(strings.NewReader(tt.routes))
			assert.Error(t, err)
			assert.Empty(t, trusted)
		})
	}
}
