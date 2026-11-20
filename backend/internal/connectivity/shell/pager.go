package shell

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// terminalWidth is the column count asked for on the PTY and in Telnet NAWS.
// Wide enough that a switch rarely wraps; when it does, a line of exactly this
// many columns is a wrapped one.
const terminalWidth = 512

// terminalHeight only has to be plausible: paging is switched off by command.
const terminalHeight = 24

// pagerMarkers cover VRP ("  ---- More ----") and FiberHome ("--More--").
var pagerMarkers = []string{"---- More ----", "--More--"}

// A [Y/N] question during a read is answered N: nothing this shell runs may
// confirm a change, and a password-expiry prompt at login is declined.
var yesNoQuestion = regexp.MustCompile(`(?i)\[y/n\]\s*:?\s*$`)

var cursorBack = regexp.MustCompile(`^\x1b\[(\d*)D`)
var otherEscape = regexp.MustCompile(`^\x1b\[[0-9;?]*[A-Za-z]`)

// pendingReply returns what the device is waiting for at the end of text: a
// space for a pager, "N" for a yes/no question, or "" when it waits for
// nothing.
func pendingReply(text string) string {
	for _, marker := range pagerMarkers {
		if strings.Contains(text, marker) {
			return " "
		}
	}
	if yesNoQuestion.MatchString(lastLine(text)) {
		return "N"
	}
	return ""
}

// renderLine applies what a terminal would to one line: a move left erases
// from there on, which is how the pager line is wiped after it is answered.
// Truncating rather than overwriting also drops a marker that was only
// returned over with '\r'. A trailing '\r' is not a return over anything:
// "text\r\r\n" (the CLI's CRLF plus the tty's ONLCR) would otherwise erase
// every line it ends.
func renderLine(raw string) string {
	raw = strings.TrimRight(raw, "\r")
	var line []rune
	for i := 0; i < len(raw); {
		rest := raw[i:]
		switch {
		case rest[0] == '\r':
			line = line[:0]
			i++
		case rest[0] == '\b':
			line = line[:max(len(line)-1, 0)]
			i++
		case cursorBack.MatchString(rest):
			match := cursorBack.FindStringSubmatch(rest)
			line = line[:max(len(line)-columns(match[1]), 0)]
			i += len(match[0])
		case otherEscape.MatchString(rest):
			i += len(otherEscape.FindString(rest))
		default:
			r, size := utf8.DecodeRuneInString(rest)
			line = append(line, r)
			i += size
		}
	}
	return string(line)
}

// columns reads the count of a cursor move; a bare ESC[D moves one column.
func columns(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 1
	}
	return n
}

// cleanLines renders raw output into lines with wrapping undone and pager
// lines removed.
func cleanLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\n\r", "\n")
	var lines []string
	pending := ""
	for _, rawLine := range strings.Split(raw, "\n") {
		line := pending + renderLine(rawLine)
		pending = ""
		if line != "" && utf8.RuneCountInString(line)%terminalWidth == 0 {
			pending = line
			continue
		}
		lines = append(lines, line)
	}
	if pending != "" {
		lines = append(lines, pending)
	}
	return dropPagerLines(lines)
}

func dropPagerLines(lines []string) []string {
	kept := lines[:0]
	for _, line := range lines {
		stripped := line
		for _, marker := range pagerMarkers {
			stripped = strings.ReplaceAll(stripped, marker, "")
		}
		if stripped != line && strings.TrimSpace(stripped) == "" {
			continue
		}
		kept = append(kept, strings.TrimRight(stripped, " \t"))
	}
	return kept
}

// commandOutput is what a command printed: the echoed command line before it
// and the prompt after it are dropped.
func commandOutput(raw, cmd string) string {
	lines := cleanLines(raw)
	if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > 0 && strings.HasSuffix(strings.TrimSpace(lines[0]), cmd) {
		lines = lines[1:]
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func lastLine(s string) string {
	if index := strings.LastIndexByte(s, '\n'); index >= 0 {
		return s[index+1:]
	}
	return s
}
