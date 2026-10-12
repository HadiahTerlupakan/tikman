package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// snmpWalkBudget bounds one IF-MIB read as a whole. Five columns of about fifty
// rows each take a few GETBULK round trips; a router that needs longer is not
// answering.
const snmpWalkBudget = 10 * time.Second

// RouterAPI is what polling reads from a router. *mikrotik.Client satisfies
// it; tests pass a fake.
type RouterAPI interface {
	Close() error
	SystemResource() (mikrotik.SystemResource, error)
	Clock() (mikrotik.Clock, error)
	Interfaces() ([]mikrotik.Interface, error)
	VLANs() ([]mikrotik.VLAN, error)
	Addresses() ([]mikrotik.Address, error)
	BGPConnections() ([]mikrotik.BGPConnection, error)
	BGPSessions() ([]mikrotik.BGPSession, error)
	StaticRoutes() ([]mikrotik.Route, error)
}

// IfCounterReader reads IF-MIB counters; connectivity.WalkIfCounters in
// production.
type IfCounterReader func(ctx context.Context, host string, port int, community string) ([]connectivity.IfCounters, error)

// RouterPollService runs the work of each router poll tier.
type RouterPollService struct {
	db               *gorm.DB
	routers          *RouterService
	newAPI           func(router *models.MikrotikRouter, password string) (RouterAPI, error)
	readCounters     IfCounterReader
	fetchFingerprint func(host string, port int) (string, error)
}

// NewRouterPollService creates a poll service that talks to real routers.
func NewRouterPollService(db *gorm.DB, routers *RouterService) *RouterPollService {
	return &RouterPollService{
		db:               db,
		routers:          routers,
		newAPI:           openRouterAPI,
		readCounters:     connectivity.WalkIfCounters,
		fetchFingerprint: mikrotik.FetchFingerprint,
	}
}

// openRouterAPI returns a nil interface on failure, not a nil *Client inside a
// non-nil interface, so callers can trust err alone.
func openRouterAPI(router *models.MikrotikRouter, password string) (RouterAPI, error) {
	client, err := mikrotik.Open(mikrotik.Config{
		Protocol: routerProtocol(router.APIProtocol), Host: router.Host, Port: router.APIPort,
		Username: router.APIUsername, Password: password, Fingerprint: router.TLSFingerprint,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// PollInterfaces reads every interface's counters and records a rate for each
// enabled interface that has an earlier sample to compare against.
func (s *RouterPollService) PollInterfaces(router *models.MikrotikRouter, now time.Time) error {
	community, _, err := s.routers.Secrets(router)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), snmpWalkBudget)
	defer cancel()
	counters, err := s.readCounters(ctx, router.Host, router.SNMPPort, community)
	if err != nil {
		s.db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Update("status", models.RouterStatusDown)
		return fmt.Errorf("read interface counters: %w", err)
	}

	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ? AND disabled = ?", router.ID, false).Find(&ifaces).Error; err != nil {
		return err
	}
	byName := make(map[string]models.RouterInterface, len(ifaces))
	for _, iface := range ifaces {
		byName[iface.Name] = iface
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, c := range counters {
			iface, ok := byName[c.Name]
			if !ok {
				continue
			}
			if err := recordCounters(tx, iface, c, now); err != nil {
				return err
			}
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).
			Updates(map[string]interface{}{"status": models.RouterStatusUp, "last_seen_at": now}).Error
	})
}

func recordCounters(tx *gorm.DB, iface models.RouterInterface, c connectivity.IfCounters, now time.Time) error {
	updates := map[string]interface{}{
		"if_index": c.IfIndex, "speed_mbps": int64(c.SpeedMbps), "oper_status": operStatus(c.OperUp),
		"prev_in_octets": int64(c.InOctets), "prev_out_octets": int64(c.OutOctets),
		"prev_sample_at": now, "last_polled_at": now,
	}
	if iface.PrevSampleAt != nil && iface.PrevInOctets != nil && iface.PrevOutOctets != nil {
		inBps, inOK := InterfaceRate(uint64(*iface.PrevInOctets), c.InOctets, *iface.PrevSampleAt, now)
		outBps, outOK := InterfaceRate(uint64(*iface.PrevOutOctets), c.OutOctets, *iface.PrevSampleAt, now)
		if inOK && outOK {
			sample := models.RouterInterfaceMetric{Time: now, InterfaceID: iface.ID, InBps: inBps, OutBps: outBps}
			if err := tx.Create(&sample).Error; err != nil {
				return err
			}
			updates["last_in_bps"], updates["last_out_bps"] = inBps, outBps
			trackSilence(iface, inBps, now, updates)
		}
	}
	return tx.Model(&models.RouterInterface{}).Where("id = ?", iface.ID).Updates(updates).Error
}

func operStatus(up bool) string {
	if up {
		return "up"
	}
	return "down"
}

// bgpSnapshot is everything one BGP poll reads from the router.
type bgpSnapshot struct {
	resource    mikrotik.SystemResource
	location    *time.Location
	connections []mikrotik.BGPConnection
	sessions    []mikrotik.BGPSession
}

// PollBGP stores each configured peer's state, an event for each change, and
// the router's resource summary.
func (s *RouterPollService) PollBGP(router *models.MikrotikRouter, now time.Time) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	defer func() { _ = api.Close() }()
	snapshot, err := readBGPSnapshot(api)
	if err != nil {
		return s.recordAPIError(router, err)
	}

	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ?", router.ID).Find(&ifaces).Error; err != nil {
		return err
	}
	var stored []models.RouterBGPSession
	if err := s.db.Where("router_id = ?", router.ID).Find(&stored).Error; err != nil {
		return err
	}
	peers := MergeBGP(snapshot.connections, snapshot.sessions, snapshot.location)
	events := DiffBGP(router.ID, stored, peers, now)

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := replaceSessions(tx, router.ID, peers, ifaces); err != nil {
			return err
		}
		if len(events) > 0 {
			if err := tx.Create(&events).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).
			Updates(resourceUpdates(snapshot.resource)).Error
	})
}

func readBGPSnapshot(api RouterAPI) (bgpSnapshot, error) {
	var snapshot bgpSnapshot
	var err error
	if snapshot.resource, err = api.SystemResource(); err != nil {
		return snapshot, err
	}
	clock, err := api.Clock()
	if err != nil {
		return snapshot, err
	}
	if snapshot.location, err = mikrotik.ParseOffset(clock.GMTOffset); err != nil {
		return snapshot, err
	}
	if snapshot.connections, err = api.BGPConnections(); err != nil {
		return snapshot, err
	}
	snapshot.sessions, err = api.BGPSessions()
	return snapshot, err
}

func replaceSessions(tx *gorm.DB, routerID uuid.UUID, peers []BGPPeerState, ifaces []models.RouterInterface) error {
	names := make([]string, 0, len(peers))
	for _, p := range peers {
		names = append(names, p.Name)
		row := models.RouterBGPSession{
			RouterID: routerID, Name: p.Name, RemoteAddress: p.RemoteAddress, RemoteAS: p.RemoteAS,
			LocalAddress: p.LocalAddress, State: p.State, UptimeSeconds: p.UptimeSeconds,
			PrefixCount: p.PrefixCount, LastStartedAt: p.LastStartedAt, LastStoppedAt: p.LastStoppedAt,
			InterfaceID: MatchInterface(p.RemoteAddress, ifaces),
		}
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "router_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"remote_address", "remote_as", "local_address", "state",
				"uptime_seconds", "prefix_count", "last_started_at", "last_stopped_at", "interface_id", "updated_at"}),
		}).Create(&row).Error
		if err != nil {
			return err
		}
	}
	gone := tx.Where("router_id = ?", routerID)
	if len(names) > 0 {
		gone = gone.Where("name NOT IN ?", names)
	}
	return gone.Delete(&models.RouterBGPSession{}).Error
}

func resourceUpdates(r mikrotik.SystemResource) map[string]interface{} {
	updates := map[string]interface{}{
		"api_status": models.RouterAPIStatusOK, "api_error": nil, "presented_fingerprint": nil,
		"ros_version": r.Version, "board_name": r.BoardName,
	}
	for column, value := range map[string]string{"cpu_load": r.CPULoad, "memory_total": r.TotalMemory, "memory_free": r.FreeMemory} {
		if n, err := mikrotik.ParseInt(value); err == nil {
			updates[column] = n
		}
	}
	if uptime, err := mikrotik.ParseDuration(r.Uptime); err == nil {
		updates["uptime_seconds"] = int64(uptime.Seconds())
	}
	return updates
}

// apiFor opens the router's API. Failing to open (refused login, wrong pin,
// unreachable) is the router's API error and is recorded as one.
func (s *RouterPollService) apiFor(router *models.MikrotikRouter) (RouterAPI, error) {
	_, password, err := s.routers.Secrets(router)
	if err != nil {
		return nil, err
	}
	api, err := s.newAPI(router, password)
	if err != nil {
		return nil, s.recordAPIError(router, err)
	}
	return api, nil
}

// recordAPIError stores why the API failed and returns the cause for the job to
// fail on. A changed certificate also stores what the router presents now, so
// an admin can see and accept it.
func (s *RouterPollService) recordAPIError(router *models.MikrotikRouter, cause error) error {
	updates := map[string]interface{}{"api_status": models.RouterAPIStatusError, "api_error": cause.Error()}
	if errors.Is(cause, mikrotik.ErrCertificateChanged) {
		updates["api_status"] = models.RouterAPIStatusCertificateChanged
		if fingerprint, err := s.fetchFingerprint(router.Host, router.APIPort); err == nil {
			updates["presented_fingerprint"] = fingerprint
		}
	}
	if err := s.db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("%w (recording it also failed: %v)", cause, err)
	}
	return cause
}
