package connectivity

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A T-CONT profile lists its bandwidths on the line after its name:
//
//	Profile name :1G
//	 Type           FBW(kbps)   ABW(kbps)   MBW(kbps)   PRIORITY   WEIGHT
//	 3              0           512         1024000     N/A         N/A
var zteTcontBandwidth = regexp.MustCompile(`(?m)^\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s`)

// ZTETcontProfile is one T-CONT profile with the bandwidths it grants. The OLT
// reports kbps; the speed shown to an operator is derived from these rather
// than from the profile's name, which is only a label and can lie.
type ZTETcontProfile struct {
	Name      string `json:"name"`
	Type      int    `json:"type"`
	FixedBW   int    `json:"fixed_bw_kbps"`
	AssuredBW int    `json:"assured_bw_kbps"`
	MaxBW     int    `json:"max_bw_kbps"`
}

// ReadZTETcontProfileDetails returns the T-CONT profiles with their bandwidths.
func ReadZTETcontProfileDetails(ctx context.Context, commander OLTCommander) ([]ZTETcontProfile, error) {
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

	return parseZTETcontProfiles(result.Output), nil
}

// parseZTETcontProfiles splits the listing on its profile headers so a
// bandwidth row is attributed to the profile it appeared under.
func parseZTETcontProfiles(output string) []ZTETcontProfile {
	profiles := make([]ZTETcontProfile, 0)

	headers := zteProfileName.FindAllStringSubmatchIndex(unwrapZTEOutput(output), -1)
	unwrapped := unwrapZTEOutput(output)
	for i, header := range headers {
		name := strings.TrimSpace(unwrapped[header[2]:header[3]])
		end := len(unwrapped)
		if i+1 < len(headers) {
			end = headers[i+1][0]
		}

		profile := ZTETcontProfile{Name: name}
		if row := zteTcontBandwidth.FindStringSubmatch(unwrapped[header[1]:end]); row != nil {
			profile.Type, _ = strconv.Atoi(row[1])
			profile.FixedBW, _ = strconv.Atoi(row[2])
			profile.AssuredBW, _ = strconv.Atoi(row[3])
			profile.MaxBW, _ = strconv.Atoi(row[4])
		}
		profiles = append(profiles, profile)
	}

	return profiles
}
