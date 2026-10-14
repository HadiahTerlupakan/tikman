package services

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

// ownPrefixRetention matches router_flow_own_prefix's retention policy.
const ownPrefixRetention = 30 * 24 * time.Hour

// ErrOwnPrefixRange refuses an own-prefix read reaching before what the table
// keeps.
var ErrOwnPrefixRange = fmt.Errorf("%w: own-prefix traffic is kept for 30 days; the range starts before that", ErrValidation)

var prependSetting = regexp.MustCompile(`set bgp-path-prepend (\d+)`)

// OwnPrefixPeer is how one peer sees one of the router's own prefixes: the
// output rule that decides it, and the traffic for the prefix over that peer.
type OwnPrefixPeer struct {
	Peer          string   `json:"peer"`
	Transit       bool     `json:"transit"`
	Sessions      []string `json:"sessions"`
	OutputFilter  string   `json:"output_filter"`
	Rule          string   `json:"rule"`
	Announced     bool     `json:"announced"`
	Prepend       int      `json:"prepend"`
	DownloadBytes int64    `json:"download_bytes"`
	UploadBytes   int64    `json:"upload_bytes"`
}

// OwnPrefix is one prefix the router announces, with each peer's view of it.
type OwnPrefix struct {
	Prefix        string          `json:"prefix"`
	DownloadBytes int64           `json:"download_bytes"`
	UploadBytes   int64           `json:"upload_bytes"`
	Peers         []OwnPrefixPeer `json:"peers"`
}

type ownPrefixBytes struct {
	Prefix              string
	UpstreamInterfaceID uuid.UUID `gorm:"column:upstream_interface_id"`
	Direction           string
	Bytes               int64
}

// OwnPrefixes lists the router's announced prefixes with, per peer, the
// output rule shaping the announcement and the bytes over that peer.
func (s *FlowQueryService) OwnPrefixes(routerID uuid.UUID, f FlowFilter, now time.Time) ([]OwnPrefix, error) {
	if now.Sub(f.From) > ownPrefixRetention+topIPRangeSlack {
		return nil, ErrOwnPrefixRange
	}
	var announced []string
	if err := s.db.Model(&models.RouterAnnouncedPrefix{}).Where("router_id = ?", routerID).
		Order("prefix").Pluck("prefix", &announced).Error; err != nil {
		return nil, err
	}
	var sessions []models.RouterBGPSession
	if err := s.db.Where("router_id = ?", routerID).Find(&sessions).Error; err != nil {
		return nil, err
	}
	var rules []models.RouterFilterRule
	if err := s.db.Where("router_id = ?", routerID).Order("position").Find(&rules).Error; err != nil {
		return nil, err
	}
	var traffic []ownPrefixBytes
	query := fmt.Sprintf(`SELECT prefix, upstream_interface_id, direction, %s AS bytes FROM router_flow_own_prefix
		WHERE router_id = ? AND time >= ? AND time < ? GROUP BY 1, 2, 3`, cappedBigint("SUM(bytes)"))
	if err := s.db.Raw(query, routerID, f.From, f.To).Scan(&traffic).Error; err != nil {
		return nil, err
	}
	return buildOwnPrefixes(announced, newRoutePeers(sessions), rules, traffic), nil
}

func buildOwnPrefixes(announced []string, peers routePeers, rules []models.RouterFilterRule, traffic []ownPrefixBytes) []OwnPrefix {
	out := make([]OwnPrefix, 0, len(announced))
	for _, prefix := range announced {
		own := OwnPrefix{Prefix: prefix, Peers: []OwnPrefixPeer{}}
		index := map[string]int{}
		for _, peer := range peers.distinct() {
			view := OwnPrefixPeer{Peer: peer.name, Transit: peer.transit, Sessions: peer.sessions,
				OutputFilter: peer.outputFilter}
			view.Rule, view.Announced, view.Prepend = outputPolicy(rules, peer.outputFilter, prefix)
			index[peer.name] = len(own.Peers)
			own.Peers = append(own.Peers, view)
		}
		for _, t := range traffic {
			peer := peers.byInterface[t.UpstreamInterfaceID]
			if t.Prefix != prefix || peer == nil {
				continue
			}
			view := &own.Peers[index[peer.name]]
			if t.Direction == "upload" {
				view.UploadBytes = addBytes(view.UploadBytes, t.Bytes)
				own.UploadBytes = addBytes(own.UploadBytes, t.Bytes)
			} else {
				view.DownloadBytes = addBytes(view.DownloadBytes, t.Bytes)
				own.DownloadBytes = addBytes(own.DownloadBytes, t.Bytes)
			}
		}
		out = append(out, own)
	}
	return out
}

// distinct is each peer once, by name.
func (p routePeers) distinct() []*routePeer {
	seen := map[*routePeer]bool{}
	var out []*routePeer
	for _, peer := range p.bySession {
		if !seen[peer] {
			seen[peer] = true
			out = append(out, peer)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// outputPolicy finds the first enabled rule in chain naming prefix, as the
// router would reach it, and reads whether it announces the prefix and with
// what prepend. A prefix no rule names returns an empty rule.
func outputPolicy(rules []models.RouterFilterRule, chain, prefix string) (string, bool, int) {
	for _, r := range rules {
		if r.Chain != chain || r.Disabled || !namesPrefix(r.Rule, prefix) {
			continue
		}
		prepend := 0
		if m := prependSetting.FindStringSubmatch(r.Rule); m != nil {
			prepend, _ = strconv.Atoi(m[1])
		}
		return r.Rule, strings.Contains(r.Rule, "accept"), prepend
	}
	return "", false, 0
}

func namesPrefix(rule, prefix string) bool {
	return strings.Contains(rule, "dst in "+prefix) || strings.Contains(rule, "dst == "+prefix)
}
