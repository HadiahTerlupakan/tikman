package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/connectivity/shell"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/switchdriver"
	"gorm.io/gorm"
)

// switchCLIBudget bounds one inventory or backup, login included. It stays
// under switchLongLease so a stuck session ends before another worker may
// claim the job.
const switchCLIBudget = 4 * time.Minute

// switchBackupRetention is how long a daily backup is kept.
const switchBackupRetention = 90 * 24 * time.Hour

const (
	backupReasonDaily = "daily"
	operStatusUnknown = "unknown"
)

// IfStateReader reads IF-MIB alias, admin state and error counters;
// connectivity.WalkIfState in production.
type IfStateReader func(ctx context.Context, host string, port int, community string) ([]connectivity.IfState, error)

// SwitchPollService runs the work of each switch poll tier. Every tier only
// reads from the switch.
type SwitchPollService struct {
	db           *gorm.DB
	switches     *SwitchService
	readCounters IfCounterReader
	readIfState  IfStateReader
	openDriver   switchdriver.Opener
}

// NewSwitchPollService creates a poll service that talks to real switches.
func NewSwitchPollService(db *gorm.DB, switches *SwitchService) *SwitchPollService {
	return &SwitchPollService{
		db:           db,
		switches:     switches,
		readCounters: connectivity.WalkIfCounters,
		readIfState:  connectivity.WalkIfState,
		openDriver:   switchdriver.Open,
	}
}

// PollInterfaces reads every port's counters and state over SNMP and records a
// rate for each port that has an earlier sample. Only this tier decides
// whether a switch is up.
func (s *SwitchPollService) PollInterfaces(sw *models.Switch, now time.Time) error {
	community, _, err := s.switches.Secrets(sw)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), snmpWalkBudget)
	defer cancel()
	counters, err := s.readCounters(ctx, sw.Host, sw.SNMPPort, community)
	if err != nil {
		s.db.Model(&models.Switch{}).Where("id = ?", sw.ID).Update("status", models.SwitchStatusDown)
		return fmt.Errorf("read interface counters: %w", err)
	}
	states := s.ifStates(ctx, sw, community)

	var ifaces []models.SwitchInterface
	if err := s.db.Where("switch_id = ?", sw.ID).Find(&ifaces).Error; err != nil {
		return err
	}
	byName := make(map[string]models.SwitchInterface, len(ifaces))
	for _, iface := range ifaces {
		byName[iface.Name] = iface
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, c := range counters {
			iface, ok := byName[c.Name]
			if !ok {
				continue
			}
			if err := recordSwitchCounters(tx, iface, c, states[c.IfIndex], now); err != nil {
				return err
			}
		}
		return tx.Model(&models.Switch{}).Where("id = ?", sw.ID).
			Updates(map[string]interface{}{"status": models.SwitchStatusUp, "last_seen_at": now}).Error
	})
}

// ifStates reads the state columns by ifIndex. They are extra to the traffic:
// a failed read keeps the earlier values rather than failing a poll whose
// counters have already answered.
func (s *SwitchPollService) ifStates(ctx context.Context, sw *models.Switch, community string) map[int]*connectivity.IfState {
	states, err := s.readIfState(ctx, sw.Host, sw.SNMPPort, community)
	if err != nil {
		return nil
	}
	byIndex := make(map[int]*connectivity.IfState, len(states))
	for i := range states {
		byIndex[states[i].IfIndex] = &states[i]
	}
	return byIndex
}

func recordSwitchCounters(tx *gorm.DB, iface models.SwitchInterface, c connectivity.IfCounters,
	state *connectivity.IfState, now time.Time) error {
	updates := map[string]interface{}{
		"if_index": c.IfIndex, "speed_mbps": int64(c.SpeedMbps), "oper_status": operStatus(c.OperUp),
		"prev_in_octets": int64(c.InOctets), "prev_out_octets": int64(c.OutOctets),
		"prev_sample_at": now, "last_polled_at": now,
	}
	if state != nil {
		updates["alias"], updates["admin_up"] = state.Alias, state.AdminUp
		updates["in_errors"], updates["out_errors"] = int64(state.InErrors), int64(state.OutErrors)
		updates["in_discards"] = int64(state.InDiscards)
	}
	if iface.PrevSampleAt != nil && iface.PrevInOctets != nil && iface.PrevOutOctets != nil {
		inBps, inOK := InterfaceRate(uint64(*iface.PrevInOctets), c.InOctets, *iface.PrevSampleAt, now)
		outBps, outOK := InterfaceRate(uint64(*iface.PrevOutOctets), c.OutOctets, *iface.PrevSampleAt, now)
		if inOK && outOK {
			sample := models.SwitchInterfaceMetric{Time: now, InterfaceID: iface.ID, InBps: inBps, OutBps: outBps}
			if err := tx.Create(&sample).Error; err != nil {
				return err
			}
			updates["last_in_bps"], updates["last_out_bps"] = inBps, outBps
		}
	}
	return tx.Model(&models.SwitchInterface{}).Where("id = ?", iface.ID).Updates(updates).Error
}

// openReader opens the switch's driver. A refused SSH host key is recorded
// so an admin can see and trust it; the job still fails.
func (s *SwitchPollService) openReader(ctx context.Context, sw *models.Switch) (switchdriver.Reader, error) {
	community, password, err := s.switches.Secrets(sw)
	if err != nil {
		return nil, err
	}
	reader, err := s.openDriver(ctx, sw, community, password)
	if err != nil {
		return nil, errors.Join(err, s.recordHostKey(sw, err))
	}
	return reader, nil
}

func (s *SwitchPollService) recordHostKey(sw *models.Switch, cause error) error {
	var unpinned shell.ErrHostKeyUnpinned
	var changed shell.ErrHostKeyChanged
	updates := map[string]interface{}{}
	switch {
	case errors.As(cause, &changed):
		updates["presented_fingerprint"], updates["host_key_changed"] = changed.Presented, true
	case errors.As(cause, &unpinned):
		updates["presented_fingerprint"] = unpinned.Presented
	default:
		return nil
	}
	return s.db.Model(&models.Switch{}).Where("id = ?", sw.ID).Updates(updates).Error
}

// switchInventory is one complete read of a switch's ports, VLANs and neighbours.
type switchInventory struct {
	ports     []switchdriver.Port
	vlans     []switchdriver.VLAN
	neighbors []switchdriver.Neighbor
}

// Inventory reads the switch's identity, ports, VLANs and neighbours over its
// CLI. Nothing is replaced until every read has answered, so a switch that
// stops answering midway keeps its earlier rows.
func (s *SwitchPollService) Inventory(sw *models.Switch, _ time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), switchCLIBudget)
	defer cancel()
	reader, err := s.openReader(ctx, sw)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	identity, err := reader.Identify(ctx)
	if err != nil {
		return fmt.Errorf("identify: %w", err)
	}
	if err := s.db.Model(&models.Switch{}).Where("id = ?", sw.ID).
		Updates(map[string]interface{}{"model": identity.Model, "version": identity.Version}).Error; err != nil {
		return err
	}
	inv, err := readSwitchInventory(ctx, reader)
	if err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertSwitchPorts(tx, sw, inv.ports); err != nil {
			return err
		}
		return replaceVLANsAndNeighbors(tx, sw, inv)
	})
}

func readSwitchInventory(ctx context.Context, reader switchdriver.Reader) (switchInventory, error) {
	var inv switchInventory
	var err error
	if inv.ports, err = reader.ReadPorts(ctx); err != nil {
		return inv, fmt.Errorf("read ports: %w", err)
	}
	if inv.vlans, err = reader.ReadVLANs(ctx); err != nil {
		return inv, fmt.Errorf("read VLANs: %w", err)
	}
	if inv.neighbors, err = reader.ReadNeighbors(ctx); err != nil {
		return inv, fmt.Errorf("read neighbours: %w", err)
	}
	return inv, nil
}

// upsertSwitchPorts merges the driver's view onto the rows SNMP fills, by
// name. A port the driver no longer lists is kept: its history stays readable.
func upsertSwitchPorts(tx *gorm.DB, sw *models.Switch, ports []switchdriver.Port) error {
	var existing []models.SwitchInterface
	if err := tx.Where("switch_id = ?", sw.ID).Find(&existing).Error; err != nil {
		return err
	}
	byName := make(map[string]models.SwitchInterface, len(existing))
	for _, iface := range existing {
		byName[iface.Name] = iface
	}
	for _, port := range ports {
		row, ok := byName[port.Name]
		if !ok {
			row = models.SwitchInterface{SwitchID: sw.ID, Name: port.Name, OperStatus: operStatusUnknown, Kind: port.Kind}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.SwitchInterface{}).Where("id = ?", row.ID).Updates(portColumns(port)).Error; err != nil {
			return err
		}
	}
	return nil
}

func portColumns(port switchdriver.Port) map[string]interface{} {
	var pvid *int
	if port.PVID > 0 {
		pvid = &port.PVID
	}
	return map[string]interface{}{
		"alias": port.Alias, "kind": port.Kind, "admin_up": port.AdminUp, "mode": port.Mode,
		"pvid": pvid, "allowed_vlans": port.AllowedVLANs, "stp_edge": port.STPEdge,
	}
}

func replaceVLANsAndNeighbors(tx *gorm.DB, sw *models.Switch, inv switchInventory) error {
	if err := tx.Where("switch_id = ?", sw.ID).Delete(&models.SwitchVLAN{}).Error; err != nil {
		return err
	}
	if err := tx.Where("switch_id = ?", sw.ID).Delete(&models.SwitchNeighbor{}).Error; err != nil {
		return err
	}
	for _, vlan := range inv.vlans {
		if err := tx.Create(&models.SwitchVLAN{SwitchID: sw.ID, VID: vlan.VID, Name: vlan.Name}).Error; err != nil {
			return err
		}
	}
	for _, n := range inv.neighbors {
		row := models.SwitchNeighbor{SwitchID: sw.ID, Port: n.Port, Kind: n.Kind, MAC: n.MAC,
			RemoteName: n.RemoteName, RemotePort: n.RemotePort, RemotePlatform: n.RemotePlatform}
		if n.VID > 0 {
			vid := n.VID
			row.VID = &vid
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// Backup reads the running config and stores it masked, with the SHA-256 of
// the raw text. An unchanged config is not stored again. The raw text never
// leaves this function.
func (s *SwitchPollService) Backup(sw *models.Switch, now time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), switchCLIBudget)
	defer cancel()
	reader, err := s.openReader(ctx, sw)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	raw, err := reader.Backup(ctx)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])

	var latest models.SwitchConfigBackup
	err = s.db.Where("switch_id = ?", sw.ID).Order("taken_at DESC").First(&latest).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err != nil || latest.RawSHA256 != hash {
		masked := MaskConfig(sw.Vendor, raw)
		latest = models.SwitchConfigBackup{SwitchID: sw.ID, TakenAt: now, Reason: backupReasonDaily,
			RawSHA256: hash, Masked: masked, Size: len(masked)}
		if err := s.db.Create(&latest).Error; err != nil {
			return err
		}
	}
	return s.pruneBackups(sw, latest, now)
}

// pruneBackups drops daily backups past retention, except the latest: an
// unchanged config is stored once, and that one copy must outlive 90 days.
func (s *SwitchPollService) pruneBackups(sw *models.Switch, latest models.SwitchConfigBackup, now time.Time) error {
	return s.db.Where("switch_id = ? AND reason = ? AND taken_at < ? AND id <> ?",
		sw.ID, backupReasonDaily, now.Add(-switchBackupRetention), latest.ID).
		Delete(&models.SwitchConfigBackup{}).Error
}
