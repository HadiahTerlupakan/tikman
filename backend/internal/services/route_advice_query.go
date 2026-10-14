package services

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

const (
	// routeAdviceIPs is how many of the range's busiest addresses are summed
	// into their prefixes.
	routeAdviceIPs = 2000
	// routeAdviceLimit is how many prefixes the advice lists.
	routeAdviceLimit = 50
)

type ipDirectionBytes struct {
	IP        string `gorm:"column:ip"`
	Direction string
	Bytes     int64
}

// RouteAdvice lists the range's busiest destination prefixes that the hourly
// route read has candidates for, busiest first, each classified. Prefixes
// the read has not reached yet are left out rather than guessed.
func (s *FlowQueryService) RouteAdvice(routerID uuid.UUID, f FlowFilter, now time.Time) ([]RouteAdvice, error) {
	if err := checkTopIPsRange(f.From, now); err != nil {
		return nil, err
	}
	advices, routes, err := s.prefixTraffic(routerID, f)
	if err != nil || len(advices) == 0 {
		return []RouteAdvice{}, err
	}
	var sessions []models.RouterBGPSession
	if err := s.db.Where("router_id = ?", routerID).Find(&sessions).Error; err != nil {
		return nil, err
	}
	asns := make([]int64, 0, len(advices))
	for i := range advices {
		advices[i].ASN = prefixOrigin(routes[i])
		asns = append(asns, advices[i].ASN)
	}
	inbound, err := s.downloadShares(routerID, f, now, asns)
	if err != nil {
		return nil, err
	}
	names, err := s.asnNames(asns)
	if err != nil {
		return nil, err
	}
	peers := newRoutePeers(sessions)
	for i := range advices {
		advices[i].Name = asnLabel(advices[i].ASN, names)
		adviseRoute(&advices[i], routes[i], peers, inbound[advices[i].ASN])
	}
	return advices, nil
}

// prefixTraffic sums the busiest addresses into the most specific stored
// prefix holding each, keeping the routeAdviceLimit busiest prefixes.
func (s *FlowQueryService) prefixTraffic(routerID uuid.UUID, f FlowFilter) ([]RouteAdvice, [][]StoredRoute, error) {
	stored, err := s.storedPrefixRoutes(routerID)
	if err != nil || len(stored) == 0 {
		return nil, nil, err
	}
	f.Upstream, f.Direction = nil, ""
	where, args := f.where(routerID, "time")
	query := fmt.Sprintf(`SELECT host(ip) AS ip, direction, %s AS bytes FROM router_flow_top_ips
		WHERE %s GROUP BY ip, direction ORDER BY bytes DESC LIMIT ?`, cappedBigint("SUM(bytes)"), where)
	var rows []ipDirectionBytes
	if err := s.db.Raw(query, withArgs(args, routeAdviceIPs)...).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	prefixes := make([]netip.Prefix, 0, len(stored))
	for p := range stored {
		prefixes = append(prefixes, p)
	}
	byPrefix := map[netip.Prefix]*RouteAdvice{}
	for _, row := range rows {
		ip, err := netip.ParseAddr(row.IP)
		if err != nil {
			continue
		}
		prefix, ok := longestHolding(prefixes, ip)
		if !ok {
			continue
		}
		advice := byPrefix[prefix]
		if advice == nil {
			advice = &RouteAdvice{Prefix: prefix.String(), CheckedAt: stored[prefix].checkedAt}
			byPrefix[prefix] = advice
		}
		if row.Direction == "upload" {
			advice.UploadBytes = addBytes(advice.UploadBytes, row.Bytes)
		} else {
			advice.DownloadBytes = addBytes(advice.DownloadBytes, row.Bytes)
		}
	}
	return busiestPrefixes(byPrefix, stored)
}

func busiestPrefixes(byPrefix map[netip.Prefix]*RouteAdvice, stored map[netip.Prefix]prefixRoutes) ([]RouteAdvice, [][]StoredRoute, error) {
	order := make([]netip.Prefix, 0, len(byPrefix))
	for p := range byPrefix {
		order = append(order, p)
	}
	total := func(p netip.Prefix) int64 { return addBytes(byPrefix[p].DownloadBytes, byPrefix[p].UploadBytes) }
	sort.Slice(order, func(i, j int) bool { return total(order[i]) > total(order[j]) })
	if len(order) > routeAdviceLimit {
		order = order[:routeAdviceLimit]
	}
	advices := make([]RouteAdvice, 0, len(order))
	routes := make([][]StoredRoute, 0, len(order))
	for _, p := range order {
		advices = append(advices, *byPrefix[p])
		routes = append(routes, stored[p].routes)
	}
	return advices, routes, nil
}

type prefixRoutes struct {
	routes    []StoredRoute
	checkedAt time.Time
}

func (s *FlowQueryService) storedPrefixRoutes(routerID uuid.UUID) (map[netip.Prefix]prefixRoutes, error) {
	var rows []models.RouterPrefixRoute
	if err := s.db.Where("router_id = ?", routerID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[netip.Prefix]prefixRoutes, len(rows))
	for _, row := range rows {
		prefix, err := netip.ParsePrefix(row.Prefix)
		if err != nil {
			continue
		}
		var routes []StoredRoute
		if err := json.Unmarshal([]byte(row.Routes), &routes); err != nil {
			continue
		}
		out[prefix] = prefixRoutes{routes: routes, checkedAt: row.CheckedAt}
	}
	return out, nil
}

// prefixOrigin is the AS announcing the prefix, from its active candidate or
// else its first.
func prefixOrigin(routes []StoredRoute) int64 {
	for _, r := range routes {
		if r.Active {
			return originASN(r.ASPath)
		}
	}
	if len(routes) == 0 {
		return otherASN
	}
	return originASN(routes[0].ASPath)
}

// downloadShares is each AS's download split over upstream interfaces.
func (s *FlowQueryService) downloadShares(routerID uuid.UUID, f FlowFilter, now time.Time, asns []int64) (map[int64][]FlowUpstreamShare, error) {
	src := resolveFlowRange(f.From, f.To, now).source
	f.Upstream, f.Direction = nil, "download"
	where, args := f.where(routerID, src.timeColumn)
	return s.upstreamShares(src.table, where, args, asns)
}
