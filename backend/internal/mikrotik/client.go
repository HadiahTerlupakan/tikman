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
