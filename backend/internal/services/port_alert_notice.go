package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

// The heads of the notices, by the status a port moved to.
const (
	noticeDownHead = "🔴 DOWN"
	noticeUpHead   = "🟢 UP"
)

// noticeTimeLayout is when a port went down, "08:15 WIB".
const noticeTimeLayout = "15:04 WIB"

// noticePort is one changed port as one report names it.
type noticePort struct {
	customer string
	label    string
	device   string
	// notify is whether the report's customer is told of this port.
	notify     bool
	transition portTransition
}

// where names the port as the admin and the network know it,
// "Uplink (core ether1)".
func (p noticePort) where() string {
	return fmt.Sprintf("%s (%s)", p.label, strings.TrimSpace(p.device+" "+p.transition.ifName))
}

// customerNotice is the one message a report's destinations get for its
// ports that moved to status in one run. Ports going down in a run went down
// together, so the time is said once; recoveries each have their own outage.
func customerNotice(customer, status string, ports []noticePort) string {
	lines := []string{noticeHead(status) + " — " + customer}
	for _, port := range ports {
		lines = append(lines, "Port: "+port.where())
		if status == models.PortLinkUp {
			lines = append(lines, "Down selama "+outageDuration(port.transition))
		}
	}
	if status == models.PortLinkDown {
		lines = append(lines, "Sejak: "+ports[0].transition.at.In(wib).Format(noticeTimeLayout))
	}
	return strings.Join(lines, "\n")
}

// nocNotice is the NOC's one message for every port that moved to status in
// one run, each under every customer whose report names it.
func nocNotice(status string, ports []noticePort) string {
	lines := []string{noticeHead(status)}
	for _, port := range ports {
		line := port.customer + " — " + port.where()
		if status == models.PortLinkUp {
			line += ", down selama " + outageDuration(port.transition)
		}
		lines = append(lines, line)
	}
	if status == models.PortLinkDown {
		lines = append(lines, "Sejak: "+ports[0].transition.at.In(wib).Format(noticeTimeLayout))
	}
	return strings.Join(lines, "\n")
}

func noticeHead(status string) string {
	if status == models.PortLinkUp {
		return noticeUpHead
	}
	return noticeDownHead
}

// outageDuration says how long a recovered port was down, "1 jam 5 menit",
// "12 menit", or "< 1 menit".
func outageDuration(t portTransition) string {
	minutes := int(t.at.Sub(t.since) / time.Minute)
	if minutes < 1 {
		return "< 1 menit"
	}
	units := []struct {
		size int
		name string
	}{{24 * 60, "hari"}, {60, "jam"}, {1, "menit"}}
	var parts []string
	for _, unit := range units {
		if n := minutes / unit.size; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, unit.name))
			minutes -= n * unit.size
		}
	}
	return strings.Join(parts, " ")
}
