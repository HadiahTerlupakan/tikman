package services

import (
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
)

// brasTopUsers is how many of the busiest subscribers the snapshot keeps.
const brasTopUsers = 10

// BRASSnapshot is the latest reading of a BRAS, stored as mikrotik_routers.bras.
type BRASSnapshot struct {
	Sessions int          `json:"sessions"`
	Cores    []BRASCore   `json:"cores"`
	Pools    []BRASPool   `json:"pools"`
	Radius   []BRASRadius `json:"radius"`
	TopUsers []BRASUser   `json:"top_users"`
	// QueuesFound is false when the router has no dynamic queues, which means
	// neither RADIUS nor a PPP profile sets rate limits: an empty top-users
	// list then says nothing about traffic.
	QueuesFound bool `json:"queues_found"`
}

// BRASCore is one CPU core's load in percent.
type BRASCore struct {
	CPU  string `json:"cpu"`
	Load int    `json:"load"`
}

// BRASPool is one address pool. Total and Free are nil when the router gives
// no total and the ranges cannot be counted.
type BRASPool struct {
	Name  string `json:"name"`
	Used  int    `json:"used"`
	Total *int   `json:"total"`
	Free  *int   `json:"free"`
}

// BRASRadius is one RADIUS server's cumulative counters. Answering is judged
// from the change since the previous snapshot; nil until there is one to judge
// against.
type BRASRadius struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Requests  int64  `json:"requests"`
	Accepts   int64  `json:"accepts"`
	Rejects   int64  `json:"rejects"`
	Timeouts  int64  `json:"timeouts"`
	Pending   int64  `json:"pending"`
	RTTMs     *int64 `json:"rtt_ms"`
	Answering *bool  `json:"answering"`
}

// BRASUser is one subscriber's queue rate in bits per second.
type BRASUser struct {
	Name    string `json:"name"`
	DownBps int64  `json:"down_bps"`
	UpBps   int64  `json:"up_bps"`
}

// poolTotalFromRanges counts the addresses in a pool's ranges, such as
// "a.b.c.d-a.b.c.e,a.b.c.f". Only IPv4 is counted; anything else gives nil,
// because a guessed total would make the free count wrong.
func poolTotalFromRanges(ranges string) *int {
	if strings.TrimSpace(ranges) == "" {
		return nil
	}
	total := 0
	for _, part := range strings.Split(ranges, ",") {
		first, last, found := strings.Cut(strings.TrimSpace(part), "-")
		if !found {
			last = first
		}
		from, errFrom := netip.ParseAddr(first)
		to, errTo := netip.ParseAddr(last)
		if errFrom != nil || errTo != nil || !from.Is4() || !to.Is4() || to.Less(from) {
			return nil
		}
		total += int(ipv4Number(to)-ipv4Number(from)) + 1
	}
	return &total
}

func ipv4Number(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// radiusAnswering judges a server from its counters since the previous read.
// Requests without accepts or rejects, while timeouts grow, is a server that
// has stopped answering. With no new requests there is nothing to judge, so
// the earlier verdict stands.
func radiusAnswering(prev *BRASRadius, cur BRASRadius) *bool {
	if prev == nil {
		return nil
	}
	if cur.Requests <= prev.Requests {
		return prev.Answering
	}
	if cur.Accepts > prev.Accepts || cur.Rejects > prev.Rejects {
		answering := true
		return &answering
	}
	if cur.Timeouts > prev.Timeouts {
		answering := false
		return &answering
	}
	return prev.Answering
}

// topUsers keeps the busiest queues by download. RouterOS writes a queue's
// rate as "upload/download", from the subscriber's side.
func topUsers(queues []mikrotik.SimpleQueue) []BRASUser {
	users := make([]BRASUser, 0, len(queues))
	for _, q := range queues {
		up, down, found := strings.Cut(q.Rate, "/")
		upBps, errUp := mikrotik.ParseInt(up)
		downBps, errDown := mikrotik.ParseInt(down)
		if !found || errUp != nil || errDown != nil {
			continue
		}
		users = append(users, BRASUser{Name: q.Name, DownBps: downBps, UpBps: upBps})
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].DownBps > users[j].DownBps })
	if len(users) > brasTopUsers {
		users = users[:brasTopUsers]
	}
	return users
}

// brasSample condenses a snapshot into the row charted per minute.
func brasSample(routerID uuid.UUID, now time.Time, snapshot BRASSnapshot) models.RouterBRASSample {
	sample := models.RouterBRASSample{RouterID: routerID, Time: now, Sessions: snapshot.Sessions}
	sum := 0
	for _, core := range snapshot.Cores {
		sum += core.Load
		if int16(core.Load) > sample.CPUMax {
			sample.CPUMax = int16(core.Load)
		}
	}
	if len(snapshot.Cores) > 0 {
		sample.CPUAvg = int16((sum + len(snapshot.Cores)/2) / len(snapshot.Cores))
	}
	for _, server := range snapshot.Radius {
		sample.RadiusAccepts += server.Accepts
		sample.RadiusRejects += server.Rejects
		sample.RadiusTimeouts += server.Timeouts
	}
	for _, pool := range snapshot.Pools {
		if pool.Free != nil && (sample.PoolMinFree == nil || *pool.Free < *sample.PoolMinFree) {
			free := *pool.Free
			sample.PoolMinFree = &free
		}
	}
	return sample
}
