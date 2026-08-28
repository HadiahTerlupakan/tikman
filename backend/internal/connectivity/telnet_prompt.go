package connectivity

import "strings"

// The CLI prints its prompt without spaces and ends it with one of these.
const promptTerminators = "#>"

// deviceHostname pulls the fixed part of the prompt out of a login response.
// A C300 prompt changes with the configuration context —
// "OLT#", "OLT(config)#", "OLT(config-if)#" — but the name in front does not,
// so that is what later reads anchor on.
func deviceHostname(loginResponse string) string {
	line := lastNonEmptyLine(loginResponse)
	if line == "" {
		return ""
	}

	name := strings.TrimRight(line, " \t"+promptTerminators)
	if index := strings.IndexAny(name, "("); index >= 0 {
		name = name[:index]
	}

	name = strings.TrimSpace(name)
	if strings.ContainsAny(name, " \t") {
		return ""
	}
	return name
}

// endsWithDevicePrompt reports whether output has stopped at the CLI prompt.
//
// The previous read returned on any '>' , '#' or '$' anywhere in the stream.
// The C300 uses '$' as its line-wrap marker, so a long echoed command ended
// the read halfway and the next command went out while the OLT was still
// writing. That is how a registration reached the device as
// "onu 15 type universalOnuType id".
func endsWithDevicePrompt(output, hostname string) bool {
	line := strings.TrimRight(lastLine(output), " \t")
	if line == "" || !strings.ContainsAny(line[len(line)-1:], promptTerminators) {
		return false
	}

	// The name anchors it. Some contexts put a space inside the brackets —
	// "OLT(gpon-onu-mng 1/3/1:1)#" — so the prompt itself is not space-free.
	if hostname != "" {
		return strings.HasPrefix(line, hostname)
	}

	// With no name to anchor on, only a prompt-shaped token will do, or output
	// that happens to end in '#' would cut the read short.
	return !strings.ContainsAny(line, " \t")
}

func lastLine(s string) string {
	if index := strings.LastIndexByte(s, '\n'); index >= 0 {
		return s[index+1:]
	}
	return s
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
