package shell

import (
	"regexp"
	"strings"
)

// Dialect selects the prompt shapes and the paging command of a switch CLI.
type Dialect string

// The CLI dialects the shell understands.
const (
	DialectVRP       Dialect = "vrp"
	DialectFiberHome Dialect = "fiberhome"
)

// The prompt right after login, before the name is known. VRP lands in user
// view; FiberHome in its exec or privileged mode.
var loginPrompt = map[Dialect]*regexp.Regexp{
	DialectVRP:       regexp.MustCompile(`^<([^<>\s]+)>$`),
	DialectFiberHome: regexp.MustCompile(`^([^\s()<>#\[\]]+)(\([^)]*\))?[>#]$`),
}

// Anchored on the learned name, because the same characters ('>', '#', ']')
// turn up at line ends inside command output.
var promptPatterns = map[Dialect][]string{
	DialectVRP:       {`^<NAME>$`, `^\[[~*]?NAME(-[^\]]+)?\]$`},
	DialectFiberHome: {`^NAME(\([^)]*\))?[>#]$`},
}

var pagingOff = map[Dialect]string{
	DialectVRP:       "screen-length 0 temporary",
	DialectFiberHome: "terminal length 0",
}

// learnHostname returns the switch name from a login prompt line, or "" when
// the line is not one.
func learnHostname(dialect Dialect, line string) string {
	pattern, ok := loginPrompt[dialect]
	if !ok {
		return ""
	}
	match := pattern.FindStringSubmatch(strings.TrimSpace(line))
	if match == nil {
		return ""
	}
	return match[1]
}

type promptMatcher []*regexp.Regexp

func newPromptMatcher(dialect Dialect, hostname string) promptMatcher {
	var matcher promptMatcher
	for _, pattern := range promptPatterns[dialect] {
		expr := strings.ReplaceAll(pattern, "NAME", regexp.QuoteMeta(hostname))
		matcher = append(matcher, regexp.MustCompile(expr))
	}
	return matcher
}

func (m promptMatcher) matches(line string) bool {
	line = strings.TrimSpace(line)
	for _, pattern := range m {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
