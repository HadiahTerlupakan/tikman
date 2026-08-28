package connectivity

import (
	"bufio"
	"io"
)

// Telnet protocol bytes, RFC 854.
const (
	telnetIAC  = 255
	telnetSE   = 240
	telnetSB   = 250
	telnetWILL = 251
	telnetWONT = 252
	telnetDO   = 253
	telnetDONT = 254
)

// Options this client agrees to, RFC 857 and RFC 858.
const (
	telnetOptionEcho            = 1
	telnetOptionSuppressGoAhead = 3
)

// negotiateTelnetOption answers one IAC sequence whose leading IAC byte the
// caller has already read. A ZTE C300 opens with WILL ECHO and three DO
// requests and prints nothing further until they are answered; leaving them
// unanswered made the login read negotiation bytes as though they were the
// username prompt and send credentials into a session that was not ready.
//
// It returns a data byte to keep when the sequence was an escaped 0xFF.
func negotiateTelnetOption(reader *bufio.Reader, w io.Writer) (byte, bool, error) {
	command, err := reader.ReadByte()
	if err != nil {
		return 0, false, err
	}

	switch command {
	case telnetIAC:
		return telnetIAC, true, nil
	case telnetSB:
		return 0, false, skipTelnetSubnegotiation(reader)
	case telnetWILL, telnetWONT, telnetDO, telnetDONT:
		option, err := reader.ReadByte()
		if err != nil {
			return 0, false, err
		}
		_, err = w.Write(telnetReply(command, option))
		return 0, false, err
	default:
		// Every other command is two bytes and needs no answer.
		return 0, false, nil
	}
}

// telnetReply refuses every option except the two the OLT needs agreement on
// before it will talk: it may echo, and go-ahead may be suppressed. Refusing
// the rest keeps this a line-mode client with no terminal to describe.
func telnetReply(command, option byte) []byte {
	switch command {
	case telnetWILL:
		if option == telnetOptionEcho || option == telnetOptionSuppressGoAhead {
			return []byte{telnetIAC, telnetDO, option}
		}
		return []byte{telnetIAC, telnetDONT, option}
	case telnetDO:
		if option == telnetOptionSuppressGoAhead {
			return []byte{telnetIAC, telnetWILL, option}
		}
		return []byte{telnetIAC, telnetWONT, option}
	case telnetWONT:
		return []byte{telnetIAC, telnetDONT, option}
	default: // telnetDONT
		return []byte{telnetIAC, telnetWONT, option}
	}
}

func skipTelnetSubnegotiation(reader *bufio.Reader) error {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if b != telnetIAC {
			continue
		}
		next, err := reader.ReadByte()
		if err != nil {
			return err
		}
		if next == telnetSE {
			return nil
		}
	}
}
