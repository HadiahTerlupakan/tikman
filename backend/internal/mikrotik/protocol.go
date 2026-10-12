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
