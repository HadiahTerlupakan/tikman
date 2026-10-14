package services

import (
	"net/netip"
	"sort"

	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// routerPolicy is what the router announces and the filter rules shaping
// it, as one read: the rules hold the prepend each peer sees per prefix.
type routerPolicy struct {
	rules     []mikrotik.FilterRule
	announced []netip.Prefix
}

// readPolicy reads the filter rules and every address list a BGP
// connection names as output.network.
func readPolicy(api RouterAPI) (routerPolicy, error) {
	var policy routerPolicy
	conns, err := api.BGPConnections()
	if err != nil {
		return policy, err
	}
	if policy.rules, err = api.FilterRules(); err != nil {
		return policy, err
	}
	seen := map[netip.Prefix]bool{}
	for _, list := range announcedLists(conns) {
		entries, err := api.AddressList(list)
		if err != nil {
			return policy, err
		}
		for _, e := range entries {
			prefix, ok := announcedPrefix(e)
			if ok && !seen[prefix] {
				seen[prefix] = true
				policy.announced = append(policy.announced, prefix)
			}
		}
	}
	sort.Slice(policy.announced, func(i, j int) bool { return policy.announced[i].String() < policy.announced[j].String() })
	return policy, nil
}

func announcedLists(conns []mikrotik.BGPConnection) []string {
	seen := map[string]bool{}
	var lists []string
	for _, c := range conns {
		if c.OutputNetwork != "" && !mikrotik.ParseBool(c.Disabled) && !seen[c.OutputNetwork] {
			seen[c.OutputNetwork] = true
			lists = append(lists, c.OutputNetwork)
		}
	}
	return lists
}

// announcedPrefix reads an address-list entry as an IPv4 prefix; a bare
// address is its /32.
func announcedPrefix(e mikrotik.AddressListEntry) (netip.Prefix, bool) {
	if mikrotik.ParseBool(e.Disabled) {
		return netip.Prefix{}, false
	}
	if prefix, err := netip.ParsePrefix(e.Address); err == nil && prefix.Addr().Is4() {
		return prefix.Masked(), true
	}
	if addr, err := netip.ParseAddr(e.Address); err == nil && addr.Is4() {
		return netip.PrefixFrom(addr, addr.BitLen()), true
	}
	return netip.Prefix{}, false
}

// savePolicy replaces the stored rules and announced prefixes with a read.
func (s *RouterPollService) savePolicy(router *models.MikrotikRouter, policy routerPolicy) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("router_id = ?", router.ID).Delete(&models.RouterFilterRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("router_id = ?", router.ID).Delete(&models.RouterAnnouncedPrefix{}).Error; err != nil {
			return err
		}
		for i, r := range policy.rules {
			row := models.RouterFilterRule{RouterID: router.ID, Position: i, Chain: r.Chain, Rule: r.Rule,
				Disabled: mikrotik.ParseBool(r.Disabled), Comment: r.Comment}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		for _, p := range policy.announced {
			if err := tx.Create(&models.RouterAnnouncedPrefix{RouterID: router.ID, Prefix: p.String()}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
