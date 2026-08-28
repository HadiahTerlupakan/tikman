package connectivity

import (
	"context"
	"fmt"
	"regexp"
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
