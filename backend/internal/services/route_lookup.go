package services

import (
	"errors"
	"net/netip"
	"time"

	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Lookup errors the handler maps to a status.
var (
	ErrLookupTarget = errors.New("target must be an IPv4 address or a prefix between /8 and /32")
	ErrLookupBusy   = errors.New("a route lookup is already running on this router")
)

// RouteLookup is what the router holds for one address or prefix: the
// prefix found and every candidate for it, or no prefix when the table has
// none.
type RouteLookup struct {
	Target    string        `json:"target"`
	Prefix    string        `json:"prefix"`
	Routes    []StoredRoute `json:"routes"`
	Queries   int           `json:"queries"`
	CheckedAt time.Time     `json:"checked_at"`
}

// LookupRoutes asks the router, now, for the BGP candidates of an address or
// prefix and stores what it finds for the hourly advice. Only one lookup runs
// per router at a time: each query scans the router's whole table.
func (s *RouterPollService) LookupRoutes(router *models.MikrotikRouter, target string, now time.Time) (RouteLookup, error) {
	prefix, addr, err := parseLookupTarget(target)
	if err != nil {
		return RouteLookup{}, err
	}
	if _, running := s.lookups.LoadOrStore(router.ID, struct{}{}); running {
		return RouteLookup{}, ErrLookupBusy
	}
	defer s.lookups.Delete(router.ID)

	api, err := s.apiFor(router)
	if err != nil {
		return RouteLookup{}, err
	}
	defer func() { _ = api.Close() }()
	read := &routeReader{api: api, budget: routeReadBudget, found: map[netip.Prefix][]mikrotik.BGPRoute{}}
	found, ok, err := s.lookupPrefix(router, read, prefix, addr)
	if err != nil {
		return RouteLookup{}, err
	}
	if err := s.saveRoutes(router, read.found, now); err != nil {
		return RouteLookup{}, err
	}
	out := RouteLookup{Target: target, Routes: []StoredRoute{}, Queries: routeReadBudget - read.budget, CheckedAt: now}
	if ok {
		out.Prefix, out.Routes = found.String(), toStoredRoutes(read.found[found])
	}
	return out, nil
}

// lookupPrefix finds the prefix the router has candidates for: the exact
// prefix given, else the covering one; for an address, a stored prefix
// holding it is tried before searching.
func (s *RouterPollService) lookupPrefix(router *models.MikrotikRouter, read *routeReader, prefix netip.Prefix, addr netip.Addr) (netip.Prefix, bool, error) {
	found, ok := netip.Prefix{}, false
	switch {
	case prefix.IsValid() && len(read.prefix(prefix)) > 0:
		found, ok = prefix, true
	case prefix.IsValid():
		found, ok = read.searchBelow(prefix.Addr(), prefix.Bits()-1)
	default:
		known, err := s.storedPrefixes(router)
		if err != nil {
			return netip.Prefix{}, false, err
		}
		if p, holds := longestHolding(known, addr); holds && len(read.prefix(p)) > 0 {
			found, ok = p, true
		} else {
			found, ok = read.search(addr)
		}
	}
	return found, ok, read.err
}

func parseLookupTarget(target string) (netip.Prefix, netip.Addr, error) {
	if addr, err := netip.ParseAddr(target); err == nil {
		if !addr.Is4() {
			return netip.Prefix{}, netip.Addr{}, ErrLookupTarget
		}
		return netip.Prefix{}, addr, nil
	}
	p, err := netip.ParsePrefix(target)
	if err != nil || !p.Addr().Is4() || p.Bits() < routeShortestPrefix {
		return netip.Prefix{}, netip.Addr{}, ErrLookupTarget
	}
	return p.Masked(), netip.Addr{}, nil
}
