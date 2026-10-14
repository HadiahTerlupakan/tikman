package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm/clause"
)

const (
	// routeReadIPs is how many of the hour's busiest destination addresses
	// the hourly read finds a BGP prefix for.
	routeReadIPs = 50
	// routeReadBudget caps router queries per run. Each scans the full table,
	// about 1.3 s on the border router, so a run stays within the job lease.
	routeReadBudget = 120
	// routeLongestPrefix and routeShortestPrefix bound the search for the
	// prefix holding an address: the router matches dst-address exactly, so
	// it is asked /24 first, then shorter, until one answers.
	routeLongestPrefix  = 24
	routeShortestPrefix = 8
	routeReadWindow     = time.Hour
)

// StoredRoute is one BGP candidate as router_prefix_routes keeps it.
type StoredRoute struct {
	Session   string `json:"session"`
	ASPath    string `json:"as_path"`
	LocalPref int64  `json:"local_pref"`
	Active    bool   `json:"active"`
	Gateway   string `json:"gateway"`
}

// ReadRoutes refreshes the router's announce policy (filter rules and own
// prefixes) and the BGP candidates for the prefixes holding the hour's
// busiest destinations. Either failing leaves the other's result stored.
func (s *RouterPollService) ReadRoutes(router *models.MikrotikRouter, now time.Time) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	defer func() { _ = api.Close() }()
	var policyErr error
	if policy, err := readPolicy(api); err != nil {
		policyErr = fmt.Errorf("announce policy: %w", err)
	} else {
		policyErr = s.savePolicy(router, policy)
	}
	return errors.Join(s.readRoutes(router, api, now), policyErr)
}

// readRoutes reads candidates for the busiest destinations. A prefix already
// known is read once per run; an address no known prefix holds is searched
// for, within the query budget.
func (s *RouterPollService) readRoutes(router *models.MikrotikRouter, api RouterAPI, now time.Time) error {
	ips, err := s.busiestDestinations(router, now)
	if err != nil || len(ips) == 0 {
		return err
	}
	known, err := s.storedPrefixes(router)
	if err != nil {
		return err
	}
	read := &routeReader{api: api, budget: routeReadBudget, found: map[netip.Prefix][]mikrotik.BGPRoute{}}
	for _, ip := range ips {
		if read.budget == 0 {
			break
		}
		if prefix, ok := longestHolding(known, ip); ok {
			read.prefix(prefix)
			continue
		}
		if prefix, ok := read.search(ip); ok {
			known = append(known, prefix)
		}
	}
	// A slow route query is not a broken API: what was read is kept, and the
	// job alone fails, leaving the router's API status to the BGP poll.
	if err := s.saveRoutes(router, read.found, now); err != nil {
		return err
	}
	return read.err
}

// routeReader asks the router for prefixes, counting down its budget and
// keeping the first error.
type routeReader struct {
	api    RouterAPI
	budget int
	found  map[netip.Prefix][]mikrotik.BGPRoute
	err    error
}

func (r *routeReader) prefix(prefix netip.Prefix) []mikrotik.BGPRoute {
	if routes, done := r.found[prefix]; done {
		return routes
	}
	if r.budget == 0 || r.err != nil {
		return nil
	}
	r.budget--
	routes, err := r.api.RoutesTo(prefix.String())
	if err != nil {
		r.err = err
		return nil
	}
	if len(routes) > 0 {
		r.found[prefix] = routes
	}
	return routes
}

// search finds the most specific prefix holding ip that the router has
// routes for.
func (r *routeReader) search(ip netip.Addr) (netip.Prefix, bool) {
	return r.searchBelow(ip, routeLongestPrefix)
}

// searchBelow is search starting at a prefix length of longest.
func (r *routeReader) searchBelow(ip netip.Addr, longest int) (netip.Prefix, bool) {
	for bits := longest; bits >= routeShortestPrefix; bits-- {
		prefix, _ := ip.Prefix(bits)
		if len(r.prefix(prefix)) > 0 {
			return prefix, true
		}
		if r.budget == 0 || r.err != nil {
			break
		}
	}
	return netip.Prefix{}, false
}

func longestHolding(prefixes []netip.Prefix, ip netip.Addr) (netip.Prefix, bool) {
	best, found := netip.Prefix{}, false
	for _, p := range prefixes {
		if p.Contains(ip) && (!found || p.Bits() > best.Bits()) {
			best, found = p, true
		}
	}
	return best, found
}

func (s *RouterPollService) busiestDestinations(router *models.MikrotikRouter, now time.Time) ([]netip.Addr, error) {
	var rows []string
	err := s.db.Model(&models.RouterFlowTopIP{}).
		Select("ip").
		Where("router_id = ? AND time >= ?", router.ID, now.Add(-routeReadWindow)).
		Group("ip").Order("SUM(bytes) DESC").Limit(routeReadIPs).
		Pluck("ip", &rows).Error
	if err != nil {
		return nil, err
	}
	ips := make([]netip.Addr, 0, len(rows))
	for _, row := range rows {
		if prefix, err := netip.ParsePrefix(row); err == nil {
			ips = append(ips, prefix.Addr())
		} else if addr, err := netip.ParseAddr(row); err == nil {
			ips = append(ips, addr)
		}
	}
	return ips, nil
}

func (s *RouterPollService) storedPrefixes(router *models.MikrotikRouter) ([]netip.Prefix, error) {
	var rows []string
	if err := s.db.Model(&models.RouterPrefixRoute{}).Where("router_id = ?", router.ID).
		Pluck("prefix", &rows).Error; err != nil {
		return nil, err
	}
	prefixes := make([]netip.Prefix, 0, len(rows))
	for _, row := range rows {
		if p, err := netip.ParsePrefix(row); err == nil {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes, nil
}

func (s *RouterPollService) saveRoutes(router *models.MikrotikRouter, found map[netip.Prefix][]mikrotik.BGPRoute, now time.Time) error {
	for prefix, routes := range found {
		encoded, err := json.Marshal(toStoredRoutes(routes))
		if err != nil {
			return err
		}
		row := models.RouterPrefixRoute{RouterID: router.ID, Prefix: prefix.String(), Routes: string(encoded), CheckedAt: now}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "router_id"}, {Name: "prefix"}},
			DoUpdates: clause.AssignmentColumns([]string{"routes", "checked_at"}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func toStoredRoutes(routes []mikrotik.BGPRoute) []StoredRoute {
	stored := make([]StoredRoute, 0, len(routes))
	for _, r := range routes {
		pref, _ := mikrotik.ParseInt(r.LocalPref)
		stored = append(stored, StoredRoute{Session: r.Session, ASPath: r.ASPath, LocalPref: pref,
			Active: mikrotik.ParseBool(r.Active), Gateway: r.Gateway})
	}
	return stored
}
