package services

import (
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

// silentAfterPolls is how many samples in a row with no inbound traffic mark
// an interface silent. A live POP always sends something (ARP, routing), so
// exactly zero three minutes running is a stop rather than a quiet moment.
const silentAfterPolls = 3

// silentBusyBps is the inbound rate that counts as carrying traffic. A POP
// serving customers is far above it; a management VLAN or the loopback sees
// a few packets now and then and is mostly at zero, and would otherwise flap
// between silent and not. Must match migration 60.
const silentBusyBps = 64_000

// trackSilence adds to updates what one inbound rate means for the
// interface's silence. Only an interface that has carried real traffic is
// ever marked; any traffic at all clears the mark. One the router reports
// down already says so plainly; marking it silent too would hide that and
// count every VLAN above a failed port.
func trackSilence(iface models.RouterInterface, inBps float64, operUp bool, now time.Time, updates map[string]interface{}) {
	if inBps > 0 || !operUp {
		if inBps >= silentBusyBps {
			updates["last_traffic_at"] = now
		}
		updates["zero_in_polls"] = 0
		updates["zero_in_since"] = nil
		updates["silent_since"] = nil
		return
	}
	since := now
	if iface.ZeroInSince != nil {
		since = *iface.ZeroInSince
	}
	polls := iface.ZeroInPolls + 1
	updates["zero_in_polls"] = polls
	updates["zero_in_since"] = since
	if polls >= silentAfterPolls && iface.LastTrafficAt != nil {
		updates["silent_since"] = since
	}
}
