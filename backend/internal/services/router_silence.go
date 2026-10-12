package services

import (
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

// silentAfterPolls is how many samples in a row with no inbound traffic mark
// an interface silent. A live POP always sends something (ARP, routing), so
// exactly zero three minutes running is a stop rather than a quiet moment.
const silentAfterPolls = 3

// trackSilence adds to updates what one inbound rate means for the
// interface's silence. An interface that has never carried traffic is spare,
// not down, and is never marked.
func trackSilence(iface models.RouterInterface, inBps float64, now time.Time, updates map[string]interface{}) {
	if inBps > 0 {
		updates["last_traffic_at"] = now
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
