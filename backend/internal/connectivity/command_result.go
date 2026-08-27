package connectivity

import "strings"

var commandFailureMarkers = []string{"error", "failed", "failure", "invalid", "unknown command", "denied", "rejected", "incomplete"}

func commandOutputError(output string) string {
	lower := strings.ToLower(output)
	for _, marker := range commandFailureMarkers {
		if strings.Contains(lower, marker) {
			return strings.TrimSpace(output)
		}
	}
	return ""
}
