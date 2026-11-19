package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// IfCounterGetter reads IF-MIB counters of the given ifIndexes only;
// connectivity.GetIfCounters in production.
type IfCounterGetter func(ctx context.Context, host string, port int, community string, indexes []int) ([]connectivity.IfCounters, error)

// pollBRASInterfaces asks only for the ports discovery has mapped to an
// ifIndex. A BRAS has an interface per PPP session, so walking IF-MIB every
// minute reads thousands of rows for a handful of ports. An index whose name no
// longer matches (ifIndexes can be reused) is not sampled, and discovery is
// made due to map the ports again.
func (s *RouterPollService) pollBRASInterfaces(router *models.MikrotikRouter, community string, now time.Time) error {
	var ifaces []models.RouterInterface
	if err := s.db.Where("router_id = ? AND disabled = ?", router.ID, false).Find(&ifaces).Error; err != nil {
		return err
	}
	byIndex := make(map[int]models.RouterInterface, len(ifaces))
	indexes := make([]int, 0, len(ifaces))
	stale := false
	for _, iface := range ifaces {
		if iface.IfIndex == nil {
			stale = true
			continue
		}
		byIndex[*iface.IfIndex] = iface
		indexes = append(indexes, *iface.IfIndex)
	}
	sort.Ints(indexes)

	ctx, cancel := context.WithTimeout(context.Background(), snmpWalkBudget)
	defer cancel()
	counters, err := s.getIfCounters(ctx, router.Host, router.SNMPPort, community, indexes)
	if err != nil {
		s.db.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Update("status", models.RouterStatusDown)
		return fmt.Errorf("get interface counters: %w", err)
	}

	routerUpdates := map[string]interface{}{"status": models.RouterStatusUp, "last_seen_at": now}
	s.addHealth(ctx, router, community, now, routerUpdates)

	return s.db.Transaction(func(tx *gorm.DB) error {
		sampled := 0
		for _, c := range counters {
			iface, ok := byIndex[c.IfIndex]
			if !ok || iface.Name != c.Name {
				continue
			}
			sampled++
			if err := recordCounters(tx, iface, c, now); err != nil {
				return err
			}
		}
		if stale || sampled < len(indexes) {
			if err := dueDiscoveryNow(tx, router, now); err != nil {
				return err
			}
		}
		return tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Updates(routerUpdates).Error
	})
}

func dueDiscoveryNow(tx *gorm.DB, router *models.MikrotikRouter, now time.Time) error {
	return tx.Model(&models.RouterPollJob{}).
		Where("router_id = ? AND kind = ? AND due_at > ?", router.ID, models.RouterPollKindDiscovery, now).
		Update("due_at", now).Error
}
