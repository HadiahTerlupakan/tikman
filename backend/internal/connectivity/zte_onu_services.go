package connectivity

import (
	"regexp"
	"strconv"
	"strings"
)

// ZTEONUService is one ONU's provisioned service, as the OLT has it now. It
// backs the pre-fill on the configure form, so it carries what that form asks
// for and nothing else.
//
// PPPoEPassword is read because reconfiguring a service requires resending it,
// and an operator who does not have it to hand would otherwise break the
// subscriber's session. It is tagged out of the JSON on purpose: the service
// is stored as JSON, and this one field is encrypted into its own column
// instead of riding along in clear text.
type ZTEONUService struct {
	// ONUType is the name the OLT was registered with, which is not the model
	// the ONU announces over OMCI: a Huawei HG8245H5 announces itself as HWTC.
	// Only the former is a name the OLT accepts back.
	ONUType      string `json:"onu_type"`
	VLANID       int    `json:"vlan_id"`
	VLANMode     string `json:"vlan_mode"`
	ServiceType  string `json:"service_type"`
	TCONTProfile string `json:"tcont_profile"`
	WANMode      string `json:"wan_mode"`
	WANIPMode    string `json:"wan_ip_mode"`
	VLANProfile  string `json:"vlan_profile"`
	// UseVEIP records whether the ONU's traffic is bound to its virtual
	// Ethernet interface. Without reading it back, the configure form reopened
	// with the toggle off however the ONU was actually set up, and saving
	// silently dropped it.
	UseVEIP       bool   `json:"use_veip"`
	PPPoEUsername string `json:"pppoe_username"`
	PPPoEPassword string `json:"-"`
}

var (
	zteInterfaceHeader = regexp.MustCompile(`^interface gpon-onu_1/(\d+)/(\d+):(\d+)`)
	ztePortHeader      = regexp.MustCompile(`^interface gpon-olt_1/(\d+)/(\d+)`)
	zteONURegistration = regexp.MustCompile(`^onu (\d+) type (\S+)`)
	zteMgmtHeader      = regexp.MustCompile(`^pon-onu-mng gpon-onu_1/(\d+)/(\d+):(\d+)`)
	zteTcontLine       = regexp.MustCompile(`^tcont (\d+).*\bprofile (\S+)`)
	zteGemportLine     = regexp.MustCompile(`^gemport (\d+).*\btcont (\d+)`)
	zteServicePortLine = regexp.MustCompile(`^service-port \d+ vport \d+ user-vlan (\S+)(?:.*\bvlan (\d+))?`)
	zteServiceLine     = regexp.MustCompile(`^service \S+ gemport (\d+)(?: (untag)| .*\bvlan (\d+))?`)
	zteWanIPLine       = regexp.MustCompile(`^wan-ip \d+ mode (\S+)`)
	zteWanIPUser       = regexp.MustCompile(`\busername (\S+)`)
	zteWanIPPassword   = regexp.MustCompile(`\bpassword (\S+)`)
	zteWanIPProfile    = regexp.MustCompile(`\bvlan-profile (\S+)`)
	// The VEIP line the provisioning form writes when "Use VEIP" is on.
	zteVEIPLine = regexp.MustCompile(`^vlan port veip_1\b`)
)

// ParseZTEONUServices reads every ONU's current service out of a running
// config. Locations that appear in neither section are simply absent.
func ParseZTEONUServices(config string) map[ONTLocation]ZTEONUService {
	services := make(map[ONTLocation]ZTEONUService)

	var location ONTLocation
	var port ONTLocation
	var inPort bool
	var inSection bool
	// Keyed by location, not reset per section: an ONU's T-CONTs and GEM ports
	// are declared under interface, while the service that picks one of them
	// sits in the separate pon-onu-mng section.
	gemportTcont := make(map[ONTLocation]map[int]int)
	tcontProfile := make(map[ONTLocation]map[int]string)

	for _, line := range strings.Split(unwrapZTEOutput(config), "\n") {
		trimmed := strings.TrimSpace(line)

		// The registered type lives on the port's own section, one line per ONU,
		// not inside the ONU's section.
		if match := ztePortHeader.FindStringSubmatch(trimmed); match != nil {
			slot, _ := strconv.Atoi(match[1])
			portID, _ := strconv.Atoi(match[2])
			port, inPort, inSection = ONTLocation{Slot: slot, Port: portID}, true, false
			continue
		}
		if inPort {
			if match := zteONURegistration.FindStringSubmatch(trimmed); match != nil {
				ontID, _ := strconv.Atoi(match[1])
				at := ONTLocation{Slot: port.Slot, Port: port.Port, ONTID: ontID}
				service := services[at]
				service.ONUType = match[2]
				services[at] = service
				continue
			}
			if trimmed == "!" || trimmed == "end" {
				inPort = false
			}
		}

		if next, ok := zteSectionLocation(trimmed); ok {
			inPort = false
			location, inSection = next, true
			if _, seen := services[location]; !seen {
				services[location] = ZTEONUService{ServiceType: "bridge", WANMode: "setup_via_ont"}
			}
			if gemportTcont[location] == nil {
				gemportTcont[location] = make(map[int]int)
				tcontProfile[location] = make(map[int]string)
			}
			continue
		}
		if trimmed == "!" || trimmed == "end" {
			inSection = false
			continue
		}
		if !inSection {
			continue
		}

		service := services[location]
		applyZTEServiceLine(&service, trimmed, gemportTcont[location], tcontProfile[location])
		services[location] = service
	}

	return services
}

func zteSectionLocation(line string) (ONTLocation, bool) {
	for _, pattern := range []*regexp.Regexp{zteInterfaceHeader, zteMgmtHeader} {
		if match := pattern.FindStringSubmatch(line); match != nil {
			slot, _ := strconv.Atoi(match[1])
			port, _ := strconv.Atoi(match[2])
			ontID, _ := strconv.Atoi(match[3])
			return ONTLocation{Slot: slot, Port: port, ONTID: ontID}, true
		}
	}
	return ONTLocation{}, false
}

// applyZTEServiceLine folds one config line into the service being built. The
// T-CONT profile is resolved through the GEM port the service names, because an
// ONU commonly carries several T-CONTs and only one of them is this service's.
func applyZTEServiceLine(service *ZTEONUService, line string, gemportTcont map[int]int, tcontProfile map[int]string) {
	switch {
	case zteTcontLine.MatchString(line):
		match := zteTcontLine.FindStringSubmatch(line)
		index, _ := strconv.Atoi(match[1])
		tcontProfile[index] = match[2]

	case zteGemportLine.MatchString(line):
		match := zteGemportLine.FindStringSubmatch(line)
		gemport, _ := strconv.Atoi(match[1])
		tcont, _ := strconv.Atoi(match[2])
		gemportTcont[gemport] = tcont

	case zteServicePortLine.MatchString(line):
		match := zteServicePortLine.FindStringSubmatch(line)
		if match[1] == "untagged" {
			service.VLANMode = "untag"
		} else {
			service.VLANMode = "tag"
		}
		if vlan, err := strconv.Atoi(match[2]); err == nil {
			service.VLANID = vlan
		} else if vlan, err := strconv.Atoi(match[1]); err == nil {
			service.VLANID = vlan
		}

	case zteVEIPLine.MatchString(line):
		service.UseVEIP = true

	case zteServiceLine.MatchString(line):
		match := zteServiceLine.FindStringSubmatch(line)
		gemport, _ := strconv.Atoi(match[1])
		if profile, ok := tcontProfile[gemportTcont[gemport]]; ok {
			service.TCONTProfile = profile
		}
		if match[2] == "untag" {
			service.VLANMode = "untag"
		}
		if vlan, err := strconv.Atoi(match[3]); err == nil {
			service.VLANID = vlan
		}

	case zteWanIPLine.MatchString(line):
		service.ServiceType = "internet"
		service.WANMode = "wan_ip"
		service.WANIPMode = zteWanIPLine.FindStringSubmatch(line)[1]
		if match := zteWanIPUser.FindStringSubmatch(line); match != nil {
			service.PPPoEUsername = match[1]
		}
		if match := zteWanIPPassword.FindStringSubmatch(line); match != nil {
			service.PPPoEPassword = match[1]
		}
		if match := zteWanIPProfile.FindStringSubmatch(line); match != nil {
			service.VLANProfile = match[1]
		}
	}
}
