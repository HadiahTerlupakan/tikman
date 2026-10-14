package services

import (
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// BGPPeerState is one configured BGP connection merged with its running
// session.
type BGPPeerState struct {
	Name          string
	RemoteAddress string
	RemoteAS      int64
	LocalAddress  string
	State         models.BGPState
	UptimeSeconds int64
	PrefixCount   int64
	LastStartedAt *time.Time
	LastStoppedAt *time.Time
	InputFilter   string
	OutputFilter  string
}

// MergeBGP reports every enabled connection once.
//
// Connections, not sessions, are the list: a peer that never comes up has a
// connection and only a useless session (remote.as 0), and reporting from
// sessions would leave it out instead of showing it down.
func MergeBGP(conns []mikrotik.BGPConnection, sessions []mikrotik.BGPSession, loc *time.Location) []BGPPeerState {
	peers := make([]BGPPeerState, 0, len(conns))
	for _, conn := range conns {
		if mikrotik.ParseBool(conn.Disabled) {
			continue
		}
		peer := BGPPeerState{Name: conn.Name, RemoteAddress: hostAddress(conn.RemoteAddress), State: models.BGPStateDown,
			InputFilter: conn.InputFilter, OutputFilter: conn.OutputFilter}
		peer.RemoteAS, _ = mikrotik.ParseInt(conn.RemoteAS)
		if session := sessionFor(conn, sessions); session != nil {
			applySession(&peer, *session, loc)
		}
		peers = append(peers, peer)
	}
	return peers
}

// sessionFor finds a connection's running session. A connection can have more
// than one (listening and connecting); the established one is what counts.
func sessionFor(conn mikrotik.BGPConnection, sessions []mikrotik.BGPSession) *mikrotik.BGPSession {
	var found *mikrotik.BGPSession
	for i := range sessions {
		if !belongsTo(sessions[i], conn) {
			continue
		}
		if mikrotik.ParseBool(sessions[i].Established) {
			return &sessions[i]
		}
		if found == nil {
			found = &sessions[i]
		}
	}
	return found
}

// belongsTo matches a session to its connection by name, RouterOS v7 naming a
// session "<connection>-<n>". The remote address is the fallback for a name in
// any other shape.
func belongsTo(session mikrotik.BGPSession, conn mikrotik.BGPConnection) bool {
	if base, ok := connectionName(session.Name); ok {
		return base == conn.Name
	}
	return hostAddress(session.RemoteAddress) == hostAddress(conn.RemoteAddress)
}

// connectionName strips the "-<n>" suffix, so "peer-rs1-NEW-1" gives
// "peer-rs1-NEW" and never matches "peer-rs1".
func connectionName(session string) (string, bool) {
	i := strings.LastIndex(session, "-")
	if i <= 0 || i == len(session)-1 {
		return "", false
	}
	if _, err := strconv.Atoi(session[i+1:]); err != nil {
		return "", false
	}
	return session[:i], true
}

func hostAddress(address string) string {
	host, _, _ := strings.Cut(address, "/")
	return host
}

func applySession(peer *BGPPeerState, session mikrotik.BGPSession, loc *time.Location) {
	peer.LocalAddress = hostAddress(session.LocalAddress)
	peer.LastStartedAt = routerTime(session.LastStarted, loc)
	peer.LastStoppedAt = routerTime(session.LastStopped, loc)
	if !mikrotik.ParseBool(session.Established) {
		return
	}
	peer.State = models.BGPStateEstablished
	if uptime, err := mikrotik.ParseDuration(session.Uptime); err == nil {
		peer.UptimeSeconds = int64(uptime.Seconds())
	}
	peer.PrefixCount, _ = mikrotik.ParseInt(session.PrefixCount)
}

func routerTime(s string, loc *time.Location) *time.Time {
	t, err := mikrotik.ParseTime(s, loc)
	if err != nil {
		return nil
	}
	return &t
}

// MatchInterface returns the interface whose subnet holds the peer's address,
// the most specific one when several do. Nil when none does: a multihop peer's
// interface is not guessed.
func MatchInterface(remote string, ifaces []models.RouterInterface) *uuid.UUID {
	addr, err := netip.ParseAddr(remote)
	if err != nil {
		return nil
	}
	var best *uuid.UUID
	bestBits := -1
	for i := range ifaces {
		bits := longestContaining(ifaces[i].Addresses, addr)
		if bits > bestBits {
			id := ifaces[i].ID
			best, bestBits = &id, bits
		}
	}
	return best
}

// longestContaining returns the prefix length of the most specific CIDR in a
// comma-separated list that holds addr, or -1 when none does.
func longestContaining(cidrs string, addr netip.Addr) int {
	best := -1
	for _, cidr := range strings.Split(cidrs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err == nil && prefix.Masked().Contains(addr) && prefix.Bits() > best {
			best = prefix.Bits()
		}
	}
	return best
}

// DiffBGP turns the difference between what was stored and what the router
// reports now into events. A session seen for the first time produces none:
// there is no earlier state for it to have changed from.
func DiffBGP(routerID uuid.UUID, stored []models.RouterBGPSession, current []BGPPeerState, now time.Time) []models.RouterBGPEvent {
	previous := make(map[string]models.RouterBGPSession, len(stored))
	for _, s := range stored {
		previous[s.Name] = s
	}

	var events []models.RouterBGPEvent
	for _, peer := range current {
		old, seen := previous[peer.Name]
		switch {
		case !seen:
		case old.State != peer.State:
			events = append(events, bgpEvent(routerID, peer.Name, old.State, peer.State, transitionTime(peer, now)))
		case peer.State == models.BGPStateEstablished && restarted(old, peer):
			events = append(events, restartEvents(routerID, old, peer)...)
		}
	}
	return events
}

func restarted(old models.RouterBGPSession, peer BGPPeerState) bool {
	return old.LastStartedAt != nil && peer.LastStartedAt != nil && !old.LastStartedAt.Equal(*peer.LastStartedAt)
}

// restartEvents records a session that went down and came back between two
// polls. Only last-started moving shows it, and an outage of seconds is still
// an outage. A last-stopped no later than the start already stored is from an
// earlier outage, so the drop is then dated at the new start instead.
func restartEvents(routerID uuid.UUID, old models.RouterBGPSession, peer BGPPeerState) []models.RouterBGPEvent {
	downAt := *peer.LastStartedAt
	if peer.LastStoppedAt != nil && peer.LastStoppedAt.After(*old.LastStartedAt) {
		downAt = *peer.LastStoppedAt
	}
	return []models.RouterBGPEvent{
		bgpEvent(routerID, peer.Name, models.BGPStateEstablished, models.BGPStateDown, downAt),
		bgpEvent(routerID, peer.Name, models.BGPStateDown, models.BGPStateEstablished, *peer.LastStartedAt),
	}
}

// transitionTime is when the router says the change happened, so an event
// carries the moment of the outage rather than the moment it was polled.
func transitionTime(peer BGPPeerState, now time.Time) time.Time {
	when := peer.LastStoppedAt
	if peer.State == models.BGPStateEstablished {
		when = peer.LastStartedAt
	}
	if when == nil {
		return now
	}
	return *when
}

func bgpEvent(routerID uuid.UUID, name string, from, to models.BGPState, at time.Time) models.RouterBGPEvent {
	return models.RouterBGPEvent{RouterID: routerID, SessionName: name, FromState: from, ToState: to, OccurredAt: at}
}
