package services

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PollBRAS reads a BRAS's sessions, cores, pools, RADIUS servers and busiest
// subscribers, stores them as the router's snapshot and adds the minute's
// sample. The resource read comes first and is written on its own, so
// api_status behaves as in PollBGP; any later read failing fails the job and
// leaves the previous snapshot in place.
func (s *RouterPollService) PollBRAS(router *models.MikrotikRouter, now time.Time) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	defer func() { _ = api.Close() }()
	resource, err := api.SystemResource()
	if err != nil {
		return s.recordAPIError(router, err)
	}
	if err := saveResource(s.db, router.ID, resource); err != nil {
		return err
	}
	previous, err := s.previousRadius(router.ID)
	if err != nil {
		return err
	}
	snapshot, err := readBRASSnapshot(api, previous)
	if err != nil {
		return s.recordAPIError(router, err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	sample := brasSample(router.ID, now, snapshot)
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&sample).Error; err != nil {
			return err
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).
			Updates(map[string]interface{}{"bras": datatypes.JSON(encoded), "bras_at": now}).Error
	})
}

// previousRadius reads the RADIUS counters of the snapshot about to be
// replaced, by server id. It reads the row rather than the router the job was
// handed, so the comparison is always against what the last poll stored. A
// snapshot that does not decode is treated as none: the verdicts start over.
func (s *RouterPollService) previousRadius(routerID uuid.UUID) (map[string]BRASRadius, error) {
	var stored models.MikrotikRouter
	if err := s.db.Select("bras").First(&stored, "id = ?", routerID).Error; err != nil {
		return nil, err
	}
	var snapshot BRASSnapshot
	if len(stored.BRAS) > 0 {
		_ = json.Unmarshal(stored.BRAS, &snapshot)
	}
	byID := make(map[string]BRASRadius, len(snapshot.Radius))
	for _, server := range snapshot.Radius {
		byID[server.ID] = server
	}
	return byID, nil
}

// readBRASSnapshot makes the reads in the spec's order; the first failure
// stops it.
func readBRASSnapshot(api RouterAPI, previous map[string]BRASRadius) (BRASSnapshot, error) {
	snapshot := BRASSnapshot{}
	var err error
	if snapshot.Cores, err = readCores(api); err != nil {
		return snapshot, err
	}
	if snapshot.Sessions, err = api.PPPActiveCount(); err != nil {
		return snapshot, err
	}
	if snapshot.Pools, err = readPools(api); err != nil {
		return snapshot, err
	}
	if snapshot.Radius, err = readRadius(api, previous); err != nil {
		return snapshot, err
	}
	queues, err := api.DynamicQueues()
	if err != nil {
		return snapshot, err
	}
	snapshot.TopUsers, snapshot.QueuesFound = topUsers(queues), len(queues) > 0
	return snapshot, nil
}

// readCores skips a core whose load is not a number rather than charting it
// as idle.
func readCores(api RouterAPI) ([]BRASCore, error) {
	loads, err := api.CPULoads()
	if err != nil {
		return nil, err
	}
	cores := make([]BRASCore, 0, len(loads))
	for _, l := range loads {
		if load, err := mikrotik.ParseInt(l.Load); err == nil {
			cores = append(cores, BRASCore{CPU: l.CPU, Load: int(load)})
		}
	}
	return cores, nil
}

// readPools uses the totals /ip/pool carries. Older 7.x builds leave them out;
// then the used addresses are counted by the router and the total comes from
// the ranges.
func readPools(api RouterAPI) ([]BRASPool, error) {
	rows, err := api.IPPools()
	if err != nil {
		return nil, err
	}
	pools := make([]BRASPool, 0, len(rows))
	for _, row := range rows {
		pool, err := readPool(api, row)
		if err != nil {
			return nil, err
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

func readPool(api RouterAPI, row mikrotik.IPPool) (BRASPool, error) {
	pool := BRASPool{Name: row.Name}
	total, errTotal := mikrotik.ParseInt(row.Total)
	used, errUsed := mikrotik.ParseInt(row.Used)
	if errTotal == nil && errUsed == nil {
		n := int(total)
		pool.Used, pool.Total = int(used), &n
	} else {
		var err error
		if pool.Used, err = api.PoolUsedCount(row.Name); err != nil {
			return pool, err
		}
		pool.Total = poolTotalFromRanges(row.Ranges)
	}
	if pool.Total != nil {
		free := *pool.Total - pool.Used
		pool.Free = &free
	}
	return pool, nil
}

func readRadius(api RouterAPI, previous map[string]BRASRadius) ([]BRASRadius, error) {
	servers, err := api.RadiusServers()
	if err != nil {
		return nil, err
	}
	out := make([]BRASRadius, 0, len(servers))
	for _, server := range servers {
		stats, err := api.RadiusMonitor(server.ID)
		if err != nil {
			return nil, err
		}
		cur := BRASRadius{
			ID: server.ID, Address: server.Address,
			Requests: counter(stats.Requests), Accepts: counter(stats.Accepts), Rejects: counter(stats.Rejects),
			Timeouts: counter(stats.Timeouts), Pending: counter(stats.Pending),
		}
		if rtt, err := mikrotik.ParseDuration(stats.LastRequestRTT); err == nil {
			ms := rtt.Milliseconds()
			cur.RTTMs = &ms
		}
		var prev *BRASRadius
		if p, ok := previous[server.ID]; ok {
			prev = &p
		}
		cur.Answering = radiusAnswering(prev, cur)
		out = append(out, cur)
	}
	return out, nil
}

// counter reads a RADIUS counter; one the router leaves out is zero.
func counter(s string) int64 {
	n, _ := mikrotik.ParseInt(s)
	return n
}
