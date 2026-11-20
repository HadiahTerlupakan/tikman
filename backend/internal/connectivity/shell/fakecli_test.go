package shell

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	fakePrompt = "<SW-TEST>"
	fakePager  = "  ---- More ----"
	iacByte    = 255
	seByte     = 240
	sbByte     = 250
	nawsOption = 31
)

// fakeErase is how VRP wipes the pager line once it has been answered: move
// back over it, blank it, move back again.
var fakeErase = "\x1b[42D" + strings.Repeat(" ", 42) + "\x1b[42D"

// fakeCLI plays a VRP user view. Each response is a list of pages; between
// two pages it prints the pager line and waits for the key that answers it.
type fakeCLI struct {
	responses map[string][]string
	hangOn    string // a command answered without a prompt afterwards
	askYesNo  bool   // ask the password-change question before the prompt

	mu           sync.Mutex
	received     []string
	pagerAnswers []byte
	nawsWidth    int
}

func (f *fakeCLI) record(line string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = append(f.received, line)
}

func (f *fakeCLI) lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.received...)
}

func (f *fakeCLI) serve(w io.Writer, r *fakeReader) {
	if f.askYesNo {
		_, _ = io.WriteString(w, "\r\nThe password needs to be changed. Change now? [Y/N]:")
		answer, err := r.line()
		if err != nil {
			return
		}
		f.record(answer)
	}
	_, _ = io.WriteString(w, "\r\n"+fakePrompt)
	for {
		line, err := r.line()
		if err != nil {
			return
		}
		f.record(line)
		_, _ = io.WriteString(w, line+"\r\n")
		if !f.answer(w, r, line) {
			continue
		}
		_, _ = io.WriteString(w, "\r\n"+fakePrompt)
	}
}

// answer prints the response to line and reports whether the prompt follows.
func (f *fakeCLI) answer(w io.Writer, r *fakeReader, line string) bool {
	pages, ok := f.responses[line]
	if !ok {
		_, _ = io.WriteString(w, "Error: Unrecognized command found at '^' position.")
		return true
	}
	for i, page := range pages {
		_, _ = io.WriteString(w, page)
		if i == len(pages)-1 {
			break
		}
		_, _ = io.WriteString(w, "\r\n"+fakePager)
		key, err := r.byte()
		if err != nil {
			return false
		}
		f.mu.Lock()
		f.pagerAnswers = append(f.pagerAnswers, key)
		f.mu.Unlock()
		_, _ = io.WriteString(w, fakeErase)
	}
	return line != f.hangOn
}

// fakeReader reads what the client typed, skipping Telnet negotiation and
// noting the window size a NAWS subnegotiation carries.
type fakeReader struct {
	r   *bufio.Reader
	cli *fakeCLI
}

func (fr *fakeReader) byte() (byte, error) {
	for {
		b, err := fr.r.ReadByte()
		if err != nil || b != iacByte {
			return b, err
		}
		command, err := fr.r.ReadByte()
		if err != nil {
			return 0, err
		}
		if command == sbByte {
			if err := fr.subnegotiation(); err != nil {
				return 0, err
			}
			continue
		}
		if _, err := fr.r.ReadByte(); err != nil {
			return 0, err
		}
	}
}

func (fr *fakeReader) subnegotiation() error {
	var body []byte
	for {
		b, err := fr.r.ReadByte()
		if err != nil {
			return err
		}
		if b == seByte && len(body) > 0 && body[len(body)-1] == iacByte {
			break
		}
		body = append(body, b)
	}
	if len(body) >= 5 && body[0] == nawsOption {
		fr.cli.mu.Lock()
		fr.cli.nawsWidth = int(body[1])<<8 | int(body[2])
		fr.cli.mu.Unlock()
	}
	return nil
}

// line reads up to Enter, which arrives as "\r" over SSH and "\r\n" over Telnet.
func (fr *fakeReader) line() (string, error) {
	var line []byte
	for {
		b, err := fr.byte()
		if err != nil {
			return "", err
		}
		switch b {
		case '\r':
			return string(line), nil
		case '\n':
			if len(line) > 0 {
				return string(line), nil
			}
		default:
			line = append(line, b)
		}
	}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func versionCLI() *fakeCLI {
	return &fakeCLI{responses: map[string][]string{
		"screen-length 0 temporary": {"Info: The configuration takes effect on the current user terminal interface only."},
		"display version":           {"Huawei Versatile Routing Platform Software\r\nVRP (R) software, Version 8.80 (CE6850EI V100R006C00)"},
		"display current-configuration": {
			"sysname SW-TEST\r\n#\r\nvlan batch 10 20 30",
			"#\r\ninterface 10GE1/0/1\r\n port link-type trunk",
			"#\r\nreturn",
		},
		"display hang": {"partial output"},
	}, hangOn: "display hang"}
}

const (
	wantVersion = "Huawei Versatile Routing Platform Software\nVRP (R) software, Version 8.80 (CE6850EI V100R006C00)"
	wantConfig  = "sysname SW-TEST\n#\nvlan batch 10 20 30\n#\ninterface 10GE1/0/1\n port link-type trunk\n#\nreturn"
)
