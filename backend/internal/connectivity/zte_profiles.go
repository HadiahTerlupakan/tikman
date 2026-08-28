package connectivity

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// zteProfileName matches the "Profile name :default" heading that opens each
// entry of "show gpon profile tcont".
var zteProfileName = regexp.MustCompile(`(?mi)^\s*Profile name\s*:\s*(\S+)`)

// zteTcontProfileCommand lists the T-CONT profiles. These are the names the
// provisioning command references as "tcont 1 name internet profile-name X".
// A C300 V2.1.0 exposes no equivalent for vlan-profile: "show gpon profile ?"
// offers only tcont and traffic.
const zteTcontProfileCommand = "show gpon profile tcont"

// disablePagingCommand stops the CLI paginating a long listing, which would
// otherwise stall the read waiting for a keypress instead of a prompt.
const disablePagingCommand = "terminal length 0"

// zteVLANProfileName captures the profile named by a wan-ip line. The C300 has
// no listing command for VLAN profiles — "show gpon profile ?" offers only
// tcont and traffic, and vlan-profile is a keyword of wan-ip rather than a
// command — so the names in use are recovered from the running config.
var zteVLANProfileName = regexp.MustCompile(`vlan-profile\s+(\S+)`)

const zteRunningConfigCommand = "show running-config"

// zteTerminalWidth is the column the CLI wraps at for a session that declined
// NAWS, which this client does. A wrap splits a value mid-token — the name
// PPPOE-214 arrives as "PPPOE-21" then "4 host 2" on the next line — so the
// output has to be rejoined before anything is read out of it.
const zteTerminalWidth = 80

// bulkCommander reads output the ordinary prompt-bounded read cannot handle.
type bulkCommander interface {
	ExecuteBulk(ctx context.Context, cmd string, quiet, max time.Duration) (string, error)
}

// zteONUTypeName captures the profile names an "onu-type" line declares. The
// registration command takes one of these, not the model string an ONU reports
// over OMCI: an F609 announces itself as F609V9, which the OLT rejects.
var zteONUTypeName = regexp.MustCompile(`(?m)^\s*onu-type\s+(\S+)`)

// ZTEConfigSnapshot is everything one pass over the running config yields: the
// ONU types the OLT will accept, the VLAN profile names in use, and each ONU's
// current service.
type ZTEConfigSnapshot struct {
	Cards        []ZTECard
	ONUTypes     []string
	VLANProfiles []string
	ONUServices  map[ONTLocation]ZTEONUService
}

// parseZTEONUTypes lists the registered ONU types in alphabetical order.
func parseZTEONUTypes(config string) []string {
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, match := range zteONUTypeName.FindAllStringSubmatch(unwrapZTEOutput(config), -1) {
		if name := strings.TrimSpace(match[1]); name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ReadZTEConfigSnapshot reads the OLT's running config once and returns only
// what the provisioning form needs from it.
//
// The config carries PPPoE passwords in clear text. Nothing but parsed values
// leaves this function: the body is never returned, logged or stored, and the
// parsed service deliberately has no password field.
func ReadZTEConfigSnapshot(ctx context.Context, commander OLTCommander) (ZTEConfigSnapshot, error) {
	bulk, ok := commander.(bulkCommander)
	if !ok {
		return ZTEConfigSnapshot{}, ErrUnsupported
	}

	if _, err := commander.ExecuteCommand(ctx, disablePagingCommand); err != nil {
		return ZTEConfigSnapshot{}, fmt.Errorf("disable paging: %w", err)
	}

	output, err := bulk.ExecuteBulk(ctx, zteRunningConfigCommand, 3*time.Second, 90*time.Second)
	if err != nil {
		return ZTEConfigSnapshot{}, fmt.Errorf("read running config: %w", err)
	}

	return ZTEConfigSnapshot{
		Cards:        ParseZTECards(output),
		ONUTypes:     parseZTEONUTypes(output),
		VLANProfiles: rankZTEVLANProfiles(output),
		ONUServices:  ParseZTEONUServices(output),
	}, nil
}

// rankZTEVLANProfiles orders names by how many ONUs use them, so the profile
// the OLT is actually standardised on leads and one-off typos sort last.
func rankZTEVLANProfiles(config string) []string {
	counts := make(map[string]int)
	for _, match := range zteVLANProfileName.FindAllStringSubmatch(unwrapZTEOutput(config), -1) {
		if name := strings.TrimSpace(match[1]); name != "" {
			counts[name]++
		}
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})

	return names
}

// unwrapZTEOutput rejoins lines the CLI broke at its terminal width. A line of
// exactly that width continues on the next one; anything shorter ended there.
func unwrapZTEOutput(output string) string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")

	var joined strings.Builder
	for i, line := range lines {
		joined.WriteString(line)
		if i == len(lines)-1 {
			continue
		}
		if len(line) != zteTerminalWidth {
			joined.WriteString("\n")
		}
	}

	return joined.String()
}

// ReadZTETcontProfiles returns the T-CONT profile names configured on the OLT,
// in the order the CLI lists them.
func ReadZTETcontProfiles(ctx context.Context, commander OLTCommander) ([]string, error) {
	if _, err := commander.ExecuteCommand(ctx, disablePagingCommand); err != nil {
		return nil, fmt.Errorf("disable paging: %w", err)
	}

	result, err := commander.ExecuteCommand(ctx, zteTcontProfileCommand)
	if err != nil {
		return nil, fmt.Errorf("list T-CONT profiles: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("list T-CONT profiles: no output")
	}
	if result.Error != "" {
		return nil, fmt.Errorf("list T-CONT profiles: %s", result.Error)
	}

	return parseZTEProfileNames(result.Output), nil
}

func parseZTEProfileNames(output string) []string {
	matches := zteProfileName.FindAllStringSubmatch(output, -1)
	names := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
