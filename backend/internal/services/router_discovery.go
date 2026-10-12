package services

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// errNoInterfaces refuses a discovery that would empty the router: every
// interface and the history hanging off it would go with that one reply.
var errNoInterfaces = errors.New("router reported no interfaces; keeping the ones already known")

type routerInventory struct {
	interfaces []mikrotik.Interface
	vlans      []mikrotik.VLAN
	addresses  []mikrotik.Address
	routes     []mikrotik.Route
}

// Discover brings the stored interfaces in line with the router's, then
// re-matches BGP sessions to them.
func (s *RouterPollService) Discover(router *models.MikrotikRouter) error {
	api, err := s.apiFor(router)
	if err != nil {
		return err
	}
	defer func() { _ = api.Close() }()
	inventory, err := readInventory(api)
	if err != nil {
		return s.recordAPIError(router, err)
	}
	rows := buildInterfaceRows(router.ID, inventory)
	if len(rows) == 0 {
		return errNoInterfaces
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertInterfaces(tx, router.ID, rows); err != nil {
			return err
		}
		if err := rematchSessions(tx, router.ID); err != nil {
			return err
		}
		return replaceRoutedPrefixes(tx, router.ID, inventory.routes)
	})
}

func readInventory(api RouterAPI) (routerInventory, error) {
	var inventory routerInventory
	var err error
	if inventory.interfaces, err = api.Interfaces(); err != nil {
		return inventory, err
	}
	if inventory.vlans, err = api.VLANs(); err != nil {
		return inventory, err
	}
	if inventory.addresses, err = api.Addresses(); err != nil {
		return inventory, err
	}
	inventory.routes, err = api.StaticRoutes()
	return inventory, err
}

func buildInterfaceRows(routerID uuid.UUID, inventory routerInventory) []models.RouterInterface {
	vlans := make(map[string]mikrotik.VLAN, len(inventory.vlans))
	for _, v := range inventory.vlans {
		vlans[v.Name] = v
	}
	addresses := map[string][]string{}
	for _, a := range inventory.addresses {
		if !mikrotik.ParseBool(a.Disabled) && !mikrotik.ParseBool(a.Invalid) {
			addresses[a.Interface] = append(addresses[a.Interface], a.Address)
		}
	}

	rows := make([]models.RouterInterface, 0, len(inventory.interfaces))
	for _, i := range inventory.interfaces {
		// A dynamic interface (one pppoe-in per connection) gets a new name
		// each time it comes up, so keyed on its name it would be a new
		// interface every time.
		if mikrotik.ParseBool(i.Dynamic) {
			continue
		}
		row := models.RouterInterface{
			RouterID: routerID, Name: i.Name, Comment: i.Comment, Type: i.Type, OperStatus: "unknown",
			Disabled: mikrotik.ParseBool(i.Disabled), Addresses: strings.Join(addresses[i.Name], ","),
		}
		if vlan, ok := vlans[i.Name]; ok {
			applyVLAN(&row, vlan)
		}
		rows = append(rows, row)
	}
	return rows
}

func applyVLAN(row *models.RouterInterface, vlan mikrotik.VLAN) {
	parent := vlan.Interface
	row.ParentName = &parent
	if id, err := mikrotik.ParseInt(vlan.VLANID); err == nil {
		vlanID := int(id)
		row.VLANID = &vlanID
	}
}

func upsertInterfaces(tx *gorm.DB, routerID uuid.UUID, rows []models.RouterInterface) error {
	names := make([]string, 0, len(rows))
	for i := range rows {
		names = append(names, rows[i].Name)
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "router_id"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"comment", "type", "vlan_id", "parent_name",
				"disabled", "addresses", "updated_at"}),
		}).Create(&rows[i]).Error
		if err != nil {
			return err
		}
	}
	return tx.Where("router_id = ? AND name NOT IN ?", routerID, names).Delete(&models.RouterInterface{}).Error
}

func rematchSessions(tx *gorm.DB, routerID uuid.UUID) error {
	var ifaces []models.RouterInterface
	if err := tx.Where("router_id = ?", routerID).Find(&ifaces).Error; err != nil {
		return err
	}
	var sessions []models.RouterBGPSession
	if err := tx.Where("router_id = ?", routerID).Find(&sessions).Error; err != nil {
		return err
	}
	for _, session := range sessions {
		err := tx.Model(&models.RouterBGPSession{}).Where("id = ?", session.ID).
			Update("interface_id", MatchInterface(session.RemoteAddress, ifaces)).Error
		if err != nil {
			return err
		}
	}
	return nil
}
