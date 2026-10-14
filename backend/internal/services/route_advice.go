package services

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// Route advice statuses, most in need of action first.
const (
	RouteTransitOnly       = "transit_only"
	RouteInboundViaTransit = "inbound_via_transit"
	RouteShorterOutbound   = "shorter_outbound"
	RouteOptimal           = "optimal"
)

const (
	// fullTablePrefixes: a peer sending this many prefixes is a transit
	// selling the whole table, the costliest way out.
	fullTablePrefixes = 500_000
	// inboundTransitShare: download counts as arriving via transit past this
	// share of its bytes.
	inboundTransitShare = 0.5
)

// RoutePath is the best candidate one peer offers for a prefix.
type RoutePath struct {
	Peer      string `json:"peer"`
	RemoteAS  int64  `json:"remote_as"`
	Transit   bool   `json:"transit"`
	ASPath    string `json:"as_path"`
	PathLen   int    `json:"path_length"`
	LocalPref int64  `json:"local_pref"`
	// InputFilter is the peer's input filter chain, where a rule preferring
	// this path would go.
	InputFilter string `json:"input_filter"`
}

// RouteInbound is how much of a prefix's AS's download arrived from one peer.
type RouteInbound struct {
	Peer    string `json:"peer"`
	Transit bool   `json:"transit"`
	Bytes   int64  `json:"bytes"`
}

// RouteAdvice is one busy destination prefix: how traffic leaves for it (the
// active route), what else the router was offered, how its download arrives,
// and what to look at first.
type RouteAdvice struct {
	Prefix        string         `json:"prefix"`
	ASN           int64          `json:"asn"`
	Name          string         `json:"name"`
	DownloadBytes int64          `json:"download_bytes"`
	UploadBytes   int64          `json:"upload_bytes"`
	Status        string         `json:"status"`
	Active        *RoutePath     `json:"active"`
	Alternatives  []RoutePath    `json:"alternatives"`
	Inbound       []RouteInbound `json:"inbound"`
	// Suggested is the peer a shorter_outbound prefix could leave by.
	Suggested string    `json:"suggested"`
	CheckedAt time.Time `json:"checked_at"`
}

// routePeer is the router's BGP sessions with one remote AS: an IX's route
// servers are several sessions but one way out.
type routePeer struct {
	name         string
	remoteAS     int64
	transit      bool
	inputFilter  string
	outputFilter string
	sessions     []string
}

type routePeers struct {
	bySession   map[string]*routePeer
	byInterface map[uuid.UUID]*routePeer
}

func newRoutePeers(sessions []models.RouterBGPSession) routePeers {
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })
	byAS := map[int64]*routePeer{}
	peers := routePeers{bySession: map[string]*routePeer{}, byInterface: map[uuid.UUID]*routePeer{}}
	for _, s := range sessions {
		peer, ok := byAS[s.RemoteAS]
		if !ok {
			peer = &routePeer{name: s.Name, remoteAS: s.RemoteAS}
			byAS[s.RemoteAS] = peer
		}
		peer.transit = peer.transit || s.PrefixCount >= fullTablePrefixes
		if peer.inputFilter == "" {
			peer.inputFilter = s.InputFilter
		}
		if peer.outputFilter == "" {
			peer.outputFilter = s.OutputFilter
		}
		peer.sessions = append(peer.sessions, s.Name)
		peers.bySession[s.Name] = peer
		if s.InterfaceID != nil {
			peers.byInterface[*s.InterfaceID] = peer
		}
	}
	return peers
}

// peerFor maps a route's session ("peer-ix-1") to its connection's peer.
func (p routePeers) peerFor(session string) *routePeer {
	if base, ok := connectionName(session); ok {
		if peer := p.bySession[base]; peer != nil {
			return peer
		}
	}
	return p.bySession[session]
}

// adviseRoute classifies one prefix from its candidates and its AS's
// download split over upstreams.
func adviseRoute(advice *RouteAdvice, routes []StoredRoute, peers routePeers, inbound []FlowUpstreamShare) {
	active, best := bestPaths(routes, peers)
	advice.Active = active
	for _, path := range best {
		if active == nil || path.Peer != active.Peer {
			advice.Alternatives = append(advice.Alternatives, path)
		}
	}
	advice.Inbound = inboundView(inbound, peers)
	advice.Status = RouteOptimal
	switch {
	case allTransit(best):
		advice.Status = RouteTransitOnly
	case transitShare(advice.Inbound) > inboundTransitShare && hasNonTransit(best):
		advice.Status = RouteInboundViaTransit
	default:
		if alt := shorterNonTransit(active, advice.Alternatives); alt != nil {
			advice.Status = RouteShorterOutbound
			advice.Suggested = alt.Peer
		}
	}
}

// bestPaths keeps, per peer, its active candidate or else its shortest one,
// shortest first.
func bestPaths(routes []StoredRoute, peers routePeers) (*RoutePath, []RoutePath) {
	byPeer := map[string]RoutePath{}
	var active *RoutePath
	for _, r := range routes {
		peer := peers.peerFor(r.Session)
		if peer == nil {
			continue
		}
		path := RoutePath{Peer: peer.name, RemoteAS: peer.remoteAS, Transit: peer.transit,
			ASPath: r.ASPath, PathLen: asPathLength(r.ASPath), LocalPref: r.LocalPref, InputFilter: peer.inputFilter}
		if r.Active {
			active = &path
			byPeer[peer.name] = path
			continue
		}
		if kept, ok := byPeer[peer.name]; !ok || (kept.PathLen > path.PathLen && (active == nil || active.Peer != peer.name)) {
			byPeer[peer.name] = path
		}
	}
	best := make([]RoutePath, 0, len(byPeer))
	for _, path := range byPeer {
		best = append(best, path)
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].PathLen != best[j].PathLen {
			return best[i].PathLen < best[j].PathLen
		}
		return best[i].Peer < best[j].Peer
	})
	return active, best
}

func asPathLength(path string) int {
	if path == "" {
		return 0
	}
	return len(strings.Split(path, ","))
}

// originASN is the AS at the end of a path, the network that announced it.
func originASN(path string) int64 {
	parts := strings.Split(path, ",")
	var asn int64
	_, _ = fmt.Sscan(strings.TrimSpace(parts[len(parts)-1]), &asn)
	return asn
}

func allTransit(paths []RoutePath) bool {
	for _, p := range paths {
		if !p.Transit {
			return false
		}
	}
	return len(paths) > 0
}

func hasNonTransit(paths []RoutePath) bool {
	return len(paths) > 0 && !allTransit(paths)
}

func inboundView(shares []FlowUpstreamShare, peers routePeers) []RouteInbound {
	index := map[string]int{}
	var out []RouteInbound
	for _, share := range shares {
		peer := peers.byInterface[share.UpstreamInterfaceID]
		if peer == nil {
			continue
		}
		if i, ok := index[peer.name]; ok {
			out[i].Bytes = addBytes(out[i].Bytes, share.Bytes)
			continue
		}
		index[peer.name] = len(out)
		out = append(out, RouteInbound{Peer: peer.name, Transit: peer.transit, Bytes: share.Bytes})
	}
	return out
}

func transitShare(inbound []RouteInbound) float64 {
	var total, transit int64
	for _, in := range inbound {
		total += in.Bytes
		if in.Transit {
			transit += in.Bytes
		}
	}
	if total == 0 {
		return 0
	}
	return float64(transit) / float64(total)
}

// shorterNonTransit is the shortest alternative that is not a transit and
// beats the active path's length. Moving traffic onto a transit is never
// suggested: it is the costliest way out.
func shorterNonTransit(active *RoutePath, alternatives []RoutePath) *RoutePath {
	if active == nil {
		return nil
	}
	for i := range alternatives {
		alt := alternatives[i]
		if !alt.Transit && alt.PathLen < active.PathLen {
			return &alt
		}
	}
	return nil
}
