package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm/clause"
)

// portTransition is a port whose recorded status just changed.
type portTransition struct {
	deviceID int
	ifName   string
	status   string
	at       time.Time
	// since is when the port went down, for a recovery's duration.
	since time.Time
}

// record stores a port's status and answers the transition it makes, or nil
// when there is none to notify. Every write is conditional, so of two
// webhooks, or two api processes, seeing the same change only one gets the
// transition back: the other's insert conflicts or its update matches no row.
func (s *PortAlertService) record(deviceID int, ifName, status string, now time.Time) (*portTransition, error) {
	var row models.PortLinkState
	err := s.db.Where("device_id = ? AND if_name = ?", deviceID, ifName).Limit(1).Find(&row).Error
	if err != nil {
		return nil, fmt.Errorf("load port status: %w", err)
	}
	if row.ID == uuid.Nil {
		return s.firstSeen(deviceID, ifName, status, now)
	}
	if row.Status == status {
		return nil, nil
	}
	res := s.db.Model(&models.PortLinkState{}).Where("id = ? AND status = ?", row.ID, row.Status).
		Updates(map[string]any{"status": status, "changed_at": now})
	if res.Error != nil {
		return nil, fmt.Errorf("store port status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &portTransition{deviceID: deviceID, ifName: ifName, status: status, at: now, since: row.ChangedAt}, nil
}

// firstSeen records a port never seen before. Only a port found down is
// announced: the alert that brought it here was most likely that port, while
// one found up has nothing to recover from.
func (s *PortAlertService) firstSeen(deviceID int, ifName, status string, now time.Time) (*portTransition, error) {
	row := models.PortLinkState{DeviceID: deviceID, IfName: ifName, Status: status, ChangedAt: now}
	res := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return nil, fmt.Errorf("store port status: %w", res.Error)
	}
	if res.RowsAffected == 0 || status != models.PortLinkDown {
		return nil, nil
	}
	return &portTransition{deviceID: deviceID, ifName: ifName, status: status, at: now, since: now}, nil
}
