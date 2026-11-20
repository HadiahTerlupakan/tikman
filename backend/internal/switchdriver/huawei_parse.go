package switchdriver

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultVLAN = 1
	maxVLAN     = 4094
)

var (
	ceModelLine   = regexp.MustCompile(`(?m)^HUAWEI (\S+) uptime is`)
	ceVersionLine = regexp.MustCompile(`VRP \(R\) software, Version \S+ \((?:\S+ )?(\S+)\)`)
	ifPrefix      = regexp.MustCompile(`^[0-9]*[A-Za-z-]+`)
	ceMACRow      = regexp.MustCompile(`^([0-9a-fA-F]{4})-([0-9a-fA-F]{4})-([0-9a-fA-F]{4})\s+(\d+)\S*\s+(\S+)`)
	leadingNumber = regexp.MustCompile(`^\s*(\d+)\s`)
)

func lines(out string) []string {
	split := strings.Split(out, "\n")
	for i, line := range split {
		split[i] = strings.TrimRight(line, "\r")
	}
	return split
}

// parseVersion reads `display version`: the model from "HUAWEI <model>
// uptime is", the release (V100R006C00SPC600) from the VRP line.
func parseVersion(out string) (Identity, error) {
	model := ceModelLine.FindStringSubmatch(out)
	if model == nil {
		return Identity{}, errors.New("switchdriver: display version did not name a Huawei model")
	}
	identity := Identity{Model: model[1]}
	if version := ceVersionLine.FindStringSubmatch(out); version != nil {
		identity.Version = version[1]
	}
	return identity, nil
}

// portKind maps a VRP interface name to how the faceplate draws it. GE can be
// copper or an SFP cage; only the transceiver tells.
func portKind(name string, fiber bool) string {
	switch ifPrefix.FindString(name) {
	case "GE":
		if fiber {
			return KindSFP
		}
		return KindRJ
	case "10GE", "25GE":
		return KindSFP
	case "40GE", "100GE":
		return KindQSFP
	case "MEth":
		return KindRJ
	default:
		return KindVirtual
	}
}

// vlanSet is a set of VIDs read from VRP lists such as "10 20 3359 to 3361".
type vlanSet map[int]bool

func (s vlanSet) apply(tokens []string, add bool) {
	for i := 0; i < len(tokens); i++ {
		lo, hi, used, ok := vlanRange(tokens[i:])
		if !ok {
			continue
		}
		i += used - 1
		for vid := max(lo, 1); vid <= min(hi, maxVLAN); vid++ {
			if add {
				s[vid] = true
			} else {
				delete(s, vid)
			}
		}
	}
}

// vlanRange reads "all", "N" or "N to M" from the start of tokens and says
// how many tokens it used.
func vlanRange(tokens []string) (lo, hi, used int, ok bool) {
	if tokens[0] == "all" {
		return 1, maxVLAN, 1, true
	}
	lo, err := strconv.Atoi(tokens[0])
	if err != nil {
		return 0, 0, 1, false
	}
	if len(tokens) >= 3 && tokens[1] == "to" {
		if hi, err := strconv.Atoi(tokens[2]); err == nil {
			return lo, hi, 3, true
		}
	}
	return lo, lo, 1, true
}

// String renders the set as "10,20,3359-3361".
func (s vlanSet) String() string {
	vids := make([]int, 0, len(s))
	for vid := range s {
		vids = append(vids, vid)
	}
	sort.Ints(vids)
	var parts []string
	for i := 0; i < len(vids); {
		j := i
		for j+1 < len(vids) && vids[j+1] == vids[j]+1 {
			j++
		}
		part := strconv.Itoa(vids[i])
		if j > i {
			part += "-" + strconv.Itoa(vids[j])
		}
		parts = append(parts, part)
		i = j + 1
	}
	return strings.Join(parts, ",")
}

// parseInterfaceBrief reads admin state from `display interface brief`:
// "*down" in the PHY column is administratively down. Trunk members are
// indented under their Eth-Trunk.
func parseInterfaceBrief(out string) map[string]bool {
	admin := map[string]bool{}
	inTable := false
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] == "Interface" && fields[1] == "PHY" {
			inTable = true
			continue
		}
		if inTable {
			admin[fields[0]] = !strings.HasPrefix(fields[1], "*down")
		}
	}
	return admin
}

// parseSTPBrief reads the effective edge state per port from the Edged
// column of `display stp brief`. A port listed once per MSTI has one edge
// state, so the first row wins. Ports with STP disabled are not listed.
func parseSTPBrief(out string) map[string]bool {
	edges := map[string]bool{}
	hasEdged := false
	for _, line := range lines(out) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "MSTID" {
			hasEdged = fields[len(fields)-1] == "Edged"
			continue
		}
		if !hasEdged || len(fields) < 3 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		if _, seen := edges[fields[1]]; !seen {
			edges[fields[1]] = fields[len(fields)-1] == "enable"
		}
	}
	return edges
}

// mergePorts lays the interface table's admin state and STP's edge state over
// the ports read from the config.
func mergePorts(ports []Port, admin, edges map[string]bool) []Port {
	merged := make([]Port, len(ports))
	for i, port := range ports {
		if up, ok := admin[port.Name]; ok {
			port.AdminUp = up
		}
		if edge, ok := edges[port.Name]; ok {
			port.STPEdge = &edge
		}
		merged[i] = port
	}
	return merged
}

// parseVLANs reads VIDs and names from the second table of `display vlan`,
// whose header ends in Description.
func parseVLANs(out string) []VLAN {
	var vlans []VLAN
	descCol := -1
	for _, line := range lines(out) {
		if strings.HasPrefix(line, "VID") && strings.Contains(line, "Description") {
			descCol = strings.Index(line, "Description")
			continue
		}
		match := leadingNumber.FindStringSubmatch(line)
		if descCol < 0 || match == nil {
			continue
		}
		vid, _ := strconv.Atoi(match[1])
		vlans = append(vlans, VLAN{VID: vid, Name: cell(line, descCol, len(line))})
	}
	return vlans
}

// cell is the trimmed text of line between two columns, empty past its end.
func cell(line string, start, end int) string {
	if start >= len(line) {
		return ""
	}
	return strings.TrimSpace(line[start:min(end, len(line))])
}

// parseLLDPBrief reads `display lldp neighbor brief`. The columns are found
// from the header because a remote port name can hold a space
// ("TFGigabitEthernet 0/25") and releases order the columns differently.
func parseLLDPBrief(out string) []Neighbor {
	var neighbors []Neighbor
	var cols lldpColumns
	for _, line := range lines(out) {
		if header, ok := parseLLDPHeader(line); ok {
			cols = header
			continue
		}
		fields := strings.Fields(line)
		if !cols.found || len(fields) == 0 || strings.HasPrefix(fields[0], "---") {
			continue
		}
		neighbors = append(neighbors, Neighbor{
			Port:       fields[0],
			Kind:       NeighborLLDP,
			RemoteName: cell(line, cols.device[0], cols.device[1]),
			RemotePort: cell(line, cols.port[0], cols.port[1]),
		})
	}
	return neighbors
}

type lldpColumns struct {
	found        bool
	port, device [2]int
}

func parseLLDPHeader(line string) (lldpColumns, bool) {
	if !strings.HasPrefix(line, "Local Int") {
		return lldpColumns{}, false
	}
	port := indexAny(line, "Neighbor Interface", "Neighbor Intf")
	device := indexAny(line, "Neighbor Device", "Neighbor Dev")
	if port < 0 || device < 0 {
		return lldpColumns{}, false
	}
	starts := []int{port, device}
	if exptime := strings.Index(line, "Exptime"); exptime >= 0 {
		starts = append(starts, exptime)
	}
	end := func(start int) int {
		next := math.MaxInt
		for _, s := range starts {
			if s > start && s < next {
				next = s
			}
		}
		return next
	}
	return lldpColumns{found: true, port: [2]int{port, end(port)}, device: [2]int{device, end(device)}}, true
}

func indexAny(line string, labels ...string) int {
	for _, label := range labels {
		if i := strings.Index(line, label); i >= 0 {
			return i
		}
	}
	return -1
}

// parseMACTable reads `display mac-address` rows such as
// "0200-5e00-0a01 10/-/-  10GE1/0/1  dynamic  200".
func parseMACTable(out string) []Neighbor {
	var neighbors []Neighbor
	for _, line := range lines(out) {
		match := ceMACRow.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		hex := strings.ToUpper(match[1] + match[2] + match[3])
		octets := make([]string, 0, 6)
		for i := 0; i < len(hex); i += 2 {
			octets = append(octets, hex[i:i+2])
		}
		vid, _ := strconv.Atoi(match[4])
		neighbors = append(neighbors, Neighbor{
			Port: match[5], Kind: NeighborMAC, MAC: strings.Join(octets, ":"), VID: vid,
		})
	}
	return neighbors
}
