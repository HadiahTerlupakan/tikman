package mikrotik

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// PPPoEServers lists the PPPoE server instances.
func (c *Client) PPPoEServers() ([]PPPoEServer, error) {
	var out []PPPoEServer
	err := c.transport.List("/interface/pppoe-server/server", &out)
	return out, err
}

// PPPActiveCount counts the live PPP sessions on the router. A BRAS holds
// thousands, so the router counts and no row travels.
func (c *Client) PPPActiveCount() (int, error) {
	return c.transport.Count("/ppp/active")
}

// IPPools lists the address pools with their totals.
func (c *Client) IPPools() ([]IPPool, error) {
	var out []IPPool
	err := c.transport.List("/ip/pool", &out)
	return out, err
}

// PoolUsedCount counts the addresses handed out from one pool.
func (c *Client) PoolUsedCount(pool string) (int, error) {
	return c.transport.Count("/ip/pool/used", "pool="+pool)
}

// RadiusServers lists the configured RADIUS servers.
func (c *Client) RadiusServers() ([]RadiusServer, error) {
	var out []RadiusServer
	err := c.transport.List("/radius", &out)
	return out, err
}

// RadiusMonitor reads one RADIUS server's counters once. Without once the
// command streams until cancelled.
func (c *Client) RadiusMonitor(id string) (RadiusStats, error) {
	var out RadiusStats
	err := c.transport.Command("/radius/monitor", &out, "numbers="+id, "once=")
	return out, err
}

// CPULoads lists the load of each CPU core.
func (c *Client) CPULoads() ([]CPULoad, error) {
	var out []CPULoad
	err := c.transport.List("/system/resource/cpu", &out)
	return out, err
}

// DynamicQueues lists the queues PPP created per session. Only three
// properties are requested: a full row is far larger, and there is one per
// subscriber.
func (c *Client) DynamicQueues() ([]SimpleQueue, error) {
	var out []SimpleQueue
	err := c.transport.ListProps("/queue/simple", "name,target,rate", &out, "dynamic=true")
	return out, err
}

// DefaultRoute lists the active default routes, whatever their origin.
func (c *Client) DefaultRoute() ([]Route, error) {
	var out []Route
	err := c.transport.List("/ip/route", &out, "dst-address=0.0.0.0/0", "active=true")
	return out, err
}

var errNoCount = errors.New("router answered a count with no number")

// countFromRows reads the number out of a count-only reply. An empty reply is
// an error: reading it as zero would report a BRAS with no subscribers.
func countFromRows(rows []map[string]string) (int, error) {
	if len(rows) == 0 {
		return 0, errNoCount
	}
	n, err := strconv.Atoi(rows[0]["ret"])
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errNoCount, rows[0]["ret"])
	}
	return n, nil
}

// countFromJSON accepts both shapes REST has been seen to use for a count.
func countFromJSON(raw json.RawMessage) (int, error) {
	rows, err := jsonRows(raw)
	if err != nil {
		return 0, err
	}
	return countFromRows(rows)
}

func jsonRows(raw json.RawMessage) ([]map[string]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '{' {
		raw = append(append([]byte{'['}, raw...), ']')
	}
	var rows []map[string]string
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode reply: %w", err)
	}
	return rows, nil
}

// decodeFirstJSON fills out from the first row of a command's reply, which
// REST wraps in an array.
func decodeFirstJSON(raw json.RawMessage, out interface{}) error {
	rows, err := jsonRows(raw)
	if err != nil {
		return err
	}
	return decodeRows(rows, out)
}
