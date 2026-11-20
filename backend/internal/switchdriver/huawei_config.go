package switchdriver

import (
	"strconv"
	"strings"
)

// ceStanza collects one `interface` block of the VRP running config. VRP
// prints only what differs from the default, so the defaults live here: an
// unconfigured switched port is an access port in VLAN 1, and trunk and
// hybrid ports start out carrying VLAN 1 until `undo … vlan 1` removes it.
type ceStanza struct {
	name, alias string
	linkType    string
	pvid        map[string]int
	trunk       vlanSet
	hybrid      vlanSet
	stpDisabled bool
	edge        *bool
	shutdown    bool
	member      bool // eth-trunk N: the Eth-Trunk carries the VLANs
	routed      bool // undo portswitch: a layer-3 port
	fiber       bool
}

func newStanza(name string) *ceStanza {
	return &ceStanza{
		name:   name,
		pvid:   map[string]int{},
		trunk:  vlanSet{defaultVLAN: true},
		hybrid: vlanSet{defaultVLAN: true},
	}
}

type stanzaRule struct {
	prefix string
	apply  func(s *ceStanza, rest string)
}

func setPVID(mode string) func(*ceStanza, string) {
	return func(s *ceStanza, rest string) {
		if vid, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
			s.pvid[mode] = vid
		}
	}
}

func editTrunk(add bool) func(*ceStanza, string) {
	return func(s *ceStanza, rest string) { s.trunk.apply(strings.Fields(rest), add) }
}

func editHybrid(add bool) func(*ceStanza, string) {
	return func(s *ceStanza, rest string) { s.hybrid.apply(strings.Fields(rest), add) }
}

func setEdge(edge bool) func(*ceStanza, string) {
	return func(s *ceStanza, _ string) { s.edge = &edge }
}

// stanzaRules are matched as prefixes of the trimmed line; anything else in a
// stanza (ip address, lldp disable, mode lacp-static, …) does not affect a
// port's VLAN or STP settings.
var stanzaRules = []stanzaRule{
	{"description ", func(s *ceStanza, rest string) { s.alias = rest }},
	{"port link-type ", func(s *ceStanza, rest string) { s.linkType = strings.TrimSpace(rest) }},
	{"port default vlan ", setPVID(ModeAccess)},
	{"port trunk pvid vlan ", setPVID(ModeTrunk)},
	{"port hybrid pvid vlan ", setPVID(ModeHybrid)},
	{"port trunk allow-pass vlan ", editTrunk(true)},
	{"undo port trunk allow-pass vlan ", editTrunk(false)},
	{"port hybrid tagged vlan ", editHybrid(true)},
	{"port hybrid untagged vlan ", editHybrid(true)},
	{"undo port hybrid tagged vlan ", editHybrid(false)},
	{"undo port hybrid untagged vlan ", editHybrid(false)},
	{"undo port hybrid vlan ", editHybrid(false)},
	{"stp disable", func(s *ceStanza, _ string) { s.stpDisabled = true }},
	{"stp edged-port enable", setEdge(true)},
	{"stp edged-port disable", setEdge(false)},
	{"shutdown", func(s *ceStanza, _ string) { s.shutdown = true }},
	{"eth-trunk ", func(s *ceStanza, _ string) { s.member = true }},
	{"undo portswitch", func(s *ceStanza, _ string) { s.routed = true }},
	{"device transceiver ", func(s *ceStanza, rest string) { s.fiber = strings.Contains(rest, "FIBER") }},
}

func (s *ceStanza) apply(text string) {
	for _, rule := range stanzaRules {
		if rest, ok := strings.CutPrefix(text, rule.prefix); ok {
			rule.apply(s, rest)
			return
		}
	}
}

// switched reports whether the port carries VLANs itself: a front-panel port
// or an Eth-Trunk that is neither a trunk member nor layer 3.
func (s *ceStanza) switched(kind string) bool {
	if s.member || s.routed {
		return false
	}
	isEthTrunk := strings.HasPrefix(s.name, "Eth-Trunk")
	isPanel := kind != KindVirtual && !strings.HasPrefix(s.name, "MEth")
	return isEthTrunk || isPanel
}

func (s *ceStanza) port() Port {
	kind := portKind(s.name, s.fiber)
	port := Port{Name: s.name, Alias: s.alias, Kind: kind, AdminUp: !s.shutdown, STPEdge: s.edge}
	if !s.switched(kind) {
		return port
	}
	disabled := s.stpDisabled
	port.STPDisabled = &disabled
	port.Mode = s.mode()
	if port.Mode == "" {
		return port
	}
	port.PVID = defaultVLAN
	if vid, ok := s.pvid[port.Mode]; ok {
		port.PVID = vid
	}
	switch port.Mode {
	case ModeTrunk:
		port.AllowedVLANs = s.trunk.String()
	case ModeHybrid:
		port.AllowedVLANs = s.hybrid.String()
	}
	return port
}

// mode is the port's link type, access when none is written. A link type
// outside the three modes (dot1q-tunnel for QinQ) gives no mode: its VLANs
// do not read the same way, and guessing would show the wrong ones.
func (s *ceStanza) mode() string {
	switch s.linkType {
	case "":
		return ModeAccess
	case ModeAccess, ModeTrunk, ModeHybrid:
		return s.linkType
	}
	return ""
}

// parseConfigPorts reads every `interface` stanza of `display
// current-configuration`, in config order. A stanza ends at "#" or at the
// next unindented line.
func parseConfigPorts(config string) []Port {
	var ports []Port
	var current *ceStanza
	flush := func() {
		if current != nil {
			ports = append(ports, current.port())
			current = nil
		}
	}
	for _, line := range lines(config) {
		if name, ok := strings.CutPrefix(line, "interface "); ok {
			flush()
			current = newStanza(strings.TrimSpace(name))
			continue
		}
		if current == nil {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			flush()
			continue
		}
		current.apply(strings.TrimSpace(line))
	}
	flush()
	return ports
}
