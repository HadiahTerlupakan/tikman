package mikrotik

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
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
// the API use, such as "/routing/bgp/session". Each where is "key=value", a
// filter the router applies before replying.
type Transport interface {
	List(path string, out interface{}, where ...string) error
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

// StaticRoutes lists active static routes, filtered on the router.
func (c *Client) StaticRoutes() ([]Route, error) {
	var out []Route
	err := c.transport.List("/ip/route", &out, "static=true", "active=true")
	return out, err
}

// filterKey is what a RouterOS property name looks like. Checking it keeps a
// caller's filter from smuggling a second query word or parameter in.
var filterKey = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// RoutesTo lists every BGP candidate for exactly prefix. The router scans its
// whole table for this, about a second on a full table, so callers ask for a
// few prefixes at a time and never list the table.
func (c *Client) RoutesTo(prefix string) ([]BGPRoute, error) {
	var out []BGPRoute
	err := c.transport.List("/routing/route", &out, "dst-address="+prefix, "bgp=true")
	return out, err
}

// FilterRules lists every routing filter rule, in evaluation order. A
// border router has a few dozen.
func (c *Client) FilterRules() ([]FilterRule, error) {
	var out []FilterRule
	err := c.transport.List("/routing/filter/rule", &out)
	return out, err
}

// AddressList lists the entries of one firewall address list.
func (c *Client) AddressList(list string) ([]AddressListEntry, error) {
	var out []AddressListEntry
	err := c.transport.List("/ip/firewall/address-list", &out, "list="+list)
	return out, err
}

// splitFilter splits a "key=value" filter, rejecting any other shape.
func splitFilter(where string) (key, value string, err error) {
	key, value, ok := strings.Cut(where, "=")
	if !ok || !filterKey.MatchString(key) || strings.ContainsAny(value, "\r\n\x00") {
		return "", "", fmt.Errorf("mikrotik: filter %q is not key=value", where)
	}
	return key, value, nil
}
