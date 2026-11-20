package connectivity

import (
	"bufio"
	"io"
)

// TelnetIAC is the byte that opens a Telnet command sequence.
const TelnetIAC = telnetIAC

// NegotiateTelnetOption answers one IAC sequence whose leading IAC the caller
// has read, as the OLT Telnet client does. It returns a data byte to keep when
// the sequence was an escaped 0xFF. Exported for connectivity/shell.
func NegotiateTelnetOption(reader *bufio.Reader, w io.Writer) (byte, bool, error) {
	return negotiateTelnetOption(reader, w)
}
