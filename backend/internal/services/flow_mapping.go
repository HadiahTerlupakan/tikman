package services

import (
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/netflow"
	"gorm.io/gorm"
)

// FlowMappingService builds flowd's view of each router from what discovery
// and the interfaces poll stored.
type FlowMappingService struct {
	db *gorm.DB
}

// NewFlowMappingService creates a mapping loader.
func NewFlowMappingService(db *gorm.DB) *FlowMappingService {
	return &FlowMappingService{db: db}
}

// Load returns a map for every enabled router collecting flows, keyed by the
// address its IPFIX arrives from, and the names of those whose host is not an
// address, or share one with another router: IPFIX carries only the sender's
// address, so they cannot be matched.
func (s *FlowMappingService) Load() (map[netip.Addr]*netflow.RouterMap, []string, error) {
	var routers []models.MikrotikRouter
	if err := s.db.Where("enabled = ? AND flow_enabled = ?", true, true).Order("name").Find(&routers).Error; err != nil {
		return nil, nil, err
	}
	maps := make(map[netip.Addr]*netflow.RouterMap, len(routers))
	owner := make(map[netip.Addr]string, len(routers))
	var skipped []string
	for i := range routers {
		addr, err := netip.ParseAddr(routers[i].Host)
		if err != nil {
			skipped = append(skipped, routers[i].Name)
			continue
		}
		addr = addr.Unmap()
		if first, shared := owner[addr]; shared {
			// A datagram carries only its source address, so neither router's
			// flows can be credited to the right one.
			if maps[addr] != nil {
				skipped = append(skipped, first)
				delete(maps, addr)
			}
			skipped = append(skipped, routers[i].Name)
			continue
		}
		m, err := s.routerMap(&routers[i])
		if err != nil {
			return nil, nil, err
		}
		owner[addr], maps[addr] = routers[i].Name, m
	}
	return maps, skipped, nil
}

func (s *FlowMappingService) routerMap(router *models.MikrotikRouter) (*netflow.RouterMap, error) {
	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ?", router.ID).Order("name").Find(&ifaces).Error; err != nil {
		return nil, err
	}
	upstream, err := s.upstreamIDs(router.ID)
	if err != nil {
		return nil, err
	}
	var routed []models.RouterRoutedPrefix
	if err := s.db.Where("router_id = ?", router.ID).Order("prefix").Find(&routed).Error; err != nil {
		return nil, err
	}
	own, err := s.announcedPrefixes(router.ID)
	if err != nil {
		return nil, err
	}
	return &netflow.RouterMap{
		RouterID: router.ID,
		// Classify multiplies by Sampling, so 0 would erase every count; the DB
		// CHECK forbids it but a bad row must not silence a router.
		Sampling:  uint64(max(router.FlowSampling, 1)),
		Upstreams: upstreamIndexes(ifaces, upstream),
		POPs:      netflow.NewPrefixTable(popPrefixes(ifaces, routed, upstream)),
		Own:       own,
	}, nil
}

// announcedPrefixes are the router's own prefixes, as the route read last
// stored them from its announce policy.
func (s *FlowMappingService) announcedPrefixes(routerID uuid.UUID) ([]netip.Prefix, error) {
	var rows []string
	if err := s.db.Model(&models.RouterAnnouncedPrefix{}).Where("router_id = ?", routerID).
		Order("prefix").Pluck("prefix", &rows).Error; err != nil {
		return nil, err
	}
	out := make([]netip.Prefix, 0, len(rows))
	for _, row := range rows {
		if prefix, err := netip.ParsePrefix(row); err == nil {
			out = append(out, prefix)
		}
	}
	return out, nil
}

// upstreamIDs are the interfaces discovery matched to a BGP session: where
// the router meets its upstreams.
func (s *FlowMappingService) upstreamIDs(routerID uuid.UUID) (map[uuid.UUID]bool, error) {
	var ids []uuid.UUID
	err := s.db.Model(&models.RouterBGPSession{}).
		Where("router_id = ? AND interface_id IS NOT NULL", routerID).
		Distinct("interface_id").Pluck("interface_id", &ids).Error
	set := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, err
}

// upstreamIndexes maps each enabled upstream's ifIndex to it. A disabled
// interface is not polled, so its stored index may since have been given to
// another interface. Claims are counted over every enabled interface, so a
// stale upstream whose number a POP interface now holds is dropped rather
// than guessed.
func upstreamIndexes(ifaces []models.RouterInterface, upstream map[uuid.UUID]bool) map[uint32]uuid.UUID {
	out := map[uint32]uuid.UUID{}
	claims := map[uint32]int{}
	for _, i := range ifaces {
		if i.Disabled || i.IfIndex == nil || *i.IfIndex <= 0 {
			continue
		}
		index := uint32(*i.IfIndex)
		claims[index]++
		if upstream[i.ID] {
			out[index] = i.ID
		}
	}
	for index, n := range claims {
		if n > 1 {
			delete(out, index)
		}
	}
	return out
}

// popPrefixes lists the blocks of every interface that is not an upstream:
// its addresses, then the blocks routed to devices behind it.
func popPrefixes(ifaces []models.RouterInterface, routed []models.RouterRoutedPrefix,
	upstream map[uuid.UUID]bool) []netflow.PrefixEntry {
	var entries []netflow.PrefixEntry
	for _, i := range ifaces {
		if upstream[i.ID] {
			continue
		}
		for _, cidr := range strings.Split(i.Addresses, ",") {
			if p, err := netip.ParsePrefix(strings.TrimSpace(cidr)); err == nil {
				entries = append(entries, netflow.PrefixEntry{Prefix: p, InterfaceID: i.ID})
			}
		}
	}
	for _, r := range routed {
		if p, err := netip.ParsePrefix(r.Prefix); err == nil && !upstream[r.InterfaceID] {
			entries = append(entries, netflow.PrefixEntry{Prefix: p, InterfaceID: r.InterfaceID})
		}
	}
	return entries
}
