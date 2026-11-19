package services

import (
	"context"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ifNameWalkBudget bounds a BRAS's ifName walk: thousands of rows at 50 a
// request take a few seconds, well inside discovery's five-minute lease.
const ifNameWalkBudget = 30 * time.Second

// IfNameReader maps a router's interface names to their ifIndex;
// connectivity.WalkIfNames in production.
type IfNameReader func(ctx context.Context, host string, port int, community string) (map[string]int, error)

// roleReads is what discovery read toward the router's role. guess is nil
// when a read failed, since a missing answer is no evidence for any role.
// role is the role this run acts on.
type roleReads struct {
	guess       *models.RouterRole
	role        models.RouterRole
	defaults    []mikrotik.Route
	defaultsErr error
	ifNames     map[string]int
	ifNamesErr  error
}

// guessRole names what a router appears to do: only a BRAS terminates PPPoE,
// and a router with a live BGP session and no PPPoE is taken for a border.
func guessRole(pppoe []mikrotik.PPPoEServer, bgp []mikrotik.BGPConnection) models.RouterRole {
	if len(pppoe) > 0 {
		return models.RouterRoleBRAS
	}
	for _, c := range bgp {
		if !mikrotik.ParseBool(c.Disabled) {
			return models.RouterRoleBorder
		}
	}
	return models.RouterRoleOther
}

// uplinkFor returns the interface the default route leaves by. A gateway that
// names its interface ("addr%iface" or a bare name) is taken at its word;
// otherwise the interface whose subnet holds the gateway address.
func uplinkFor(routes []mikrotik.Route, ifaces []models.RouterInterface) *uuid.UUID {
	for _, route := range routes {
		gateway := route.Gateway
		if at := strings.LastIndex(gateway, "%"); at >= 0 {
			gateway = gateway[at+1:]
		}
		if _, err := netip.ParseAddr(gateway); err == nil {
			if id := MatchInterface(gateway, ifaces); id != nil {
				return id
			}
			continue
		}
		for i := range ifaces {
			if ifaces[i].Name == gateway {
				return &ifaces[i].ID
			}
		}
	}
	return nil
}

// readRole reads the role's evidence and, for a BRAS, its default route and
// ifName table. The walk runs here rather than in discovery's transaction so
// no database transaction waits on SNMP.
func (s *RouterPollService) readRole(api RouterAPI, router *models.MikrotikRouter) roleReads {
	reads := roleReads{role: router.Role}
	if pppoe, err := api.PPPoEServers(); err == nil {
		if bgp, err := api.BGPConnections(); err == nil {
			guess := guessRole(pppoe, bgp)
			reads.guess = &guess
		}
	}
	if reads.guess != nil && !router.RoleSetByAdmin {
		reads.role = *reads.guess
	}
	if reads.role != models.RouterRoleBRAS {
		return reads
	}
	reads.defaults, reads.defaultsErr = api.DefaultRoute()
	reads.ifNames, reads.ifNamesErr = s.walkIfNames(router)
	return reads
}

func (s *RouterPollService) walkIfNames(router *models.MikrotikRouter) (map[string]int, error) {
	community, _, err := s.routers.Secrets(router)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), ifNameWalkBudget)
	defer cancel()
	return s.readIfNames(ctx, router.Host, router.SNMPPort, community)
}

// applyRole stores the guess, and moves the role to it unless an admin chose
// the role.
func applyRole(tx *gorm.DB, router *models.MikrotikRouter, reads roleReads) error {
	if reads.guess == nil {
		return nil
	}
	err := tx.Model(&models.MikrotikRouter{}).Where("id = ?", router.ID).Update("role_guess", *reads.guess).Error
	if err != nil || router.RoleSetByAdmin {
		return err
	}
	return SetRouterRole(tx, router.ID, *reads.guess, false)
}

// applyBRAS stores a BRAS's uplink and ifIndexes against the interfaces just
// upserted. A read that failed leaves what was stored before.
func applyBRAS(tx *gorm.DB, routerID uuid.UUID, reads roleReads) error {
	if reads.role != models.RouterRoleBRAS {
		return nil
	}
	var ifaces []models.RouterInterface
	if err := tx.Where("router_id = ?", routerID).Find(&ifaces).Error; err != nil {
		return err
	}
	if reads.defaultsErr == nil {
		err := tx.Model(&models.MikrotikRouter{}).Where("id = ?", routerID).
			Update("uplink_interface_id", uplinkFor(reads.defaults, ifaces)).Error
		if err != nil {
			return err
		}
	}
	if reads.ifNamesErr != nil {
		return nil
	}
	for _, iface := range ifaces {
		index, ok := reads.ifNames[iface.Name]
		if !ok {
			continue
		}
		if err := tx.Model(&models.RouterInterface{}).Where("id = ?", iface.ID).Update("if_index", index).Error; err != nil {
			return err
		}
	}
	return nil
}
