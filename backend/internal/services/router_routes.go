package services

import (
	"net/netip"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// routedPrefixBatch is rows per INSERT; a border router has tens of static
// routes, not thousands.
const routedPrefixBatch = 200

// replaceRoutedPrefixes refills the router's routed blocks from this
// discovery's routes: a route removed on the router disappears here.
func replaceRoutedPrefixes(tx *gorm.DB, routerID uuid.UUID, routes []mikrotik.Route) error {
	var ifaces []models.RouterInterface
	if err := tx.Where("router_id = ?", routerID).Find(&ifaces).Error; err != nil {
		return err
	}
	if err := tx.Where("router_id = ?", routerID).Delete(&models.RouterRoutedPrefix{}).Error; err != nil {
		return err
	}
	rows := buildRoutedPrefixes(routerID, routes, ifaces)
	if len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, routedPrefixBatch).Error
}

// buildRoutedPrefixes keeps the static routes that hand a block to a device
// behind one of the router's interfaces, keyed to that interface.
func buildRoutedPrefixes(routerID uuid.UUID, routes []mikrotik.Route, ifaces []models.RouterInterface) []models.RouterRoutedPrefix {
	byName := make(map[string]uuid.UUID, len(ifaces))
	for _, i := range ifaces {
		byName[i.Name] = i.ID
	}
	seen := map[string]bool{}
	var rows []models.RouterRoutedPrefix
	for _, r := range routes {
		prefix, ok := routedPrefix(r)
		if !ok || seen[prefix] {
			continue
		}
		interfaceID := routeInterface(r, byName, ifaces)
		if interfaceID == nil {
			continue
		}
		seen[prefix] = true
		rows = append(rows, models.RouterRoutedPrefix{RouterID: routerID, Prefix: prefix, InterfaceID: *interfaceID})
	}
	return rows
}

// routedPrefix returns the masked block of a route that can be a POP's. The
// router already filtered on static and active; the flags are checked again
// because a BGP route slipping through would hand an upstream's whole table
// to a POP.
func routedPrefix(r mikrotik.Route) (string, bool) {
	if !mikrotik.ParseBool(r.Static) || !mikrotik.ParseBool(r.Active) || mikrotik.ParseBool(r.BGP) ||
		mikrotik.ParseBool(r.Connect) || mikrotik.ParseBool(r.Disabled) {
		return "", false
	}
	p, err := netip.ParsePrefix(r.DstAddress)
	// A default route points at an upstream, never at a POP's block.
	if err != nil || !p.Addr().Is4() || p.Bits() == 0 {
		return "", false
	}
	return p.Masked().String(), true
}

// routeInterface finds the interface a route leaves through: the one
// immediate-gw names ("192.0.2.34%vlan-pop-a"), else the one whose subnet
// holds the gateway, else a gateway that is itself an interface name.
func routeInterface(r mikrotik.Route, byName map[string]uuid.UUID, ifaces []models.RouterInterface) *uuid.UUID {
	if _, name, ok := strings.Cut(r.ImmediateGW, "%"); ok {
		if id, known := byName[name]; known {
			return &id
		}
	}
	if id := MatchInterface(r.Gateway, ifaces); id != nil {
		return id
	}
	if id, known := byName[r.Gateway]; known {
		return &id
	}
	return nil
}
