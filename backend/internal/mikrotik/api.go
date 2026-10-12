package mikrotik

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// apiCallTimeout bounds dialing, login and each command, as requestTimeout
// does for REST. A variable only so tests need not wait five seconds.
var apiCallTimeout = requestTimeout

// maxReplyBytes caps one command's whole reply. A reply larger than this is
// not one of the read menus TikMan uses; the bound keeps a misbehaving or
// impersonated peer (plain API has no certificate pin) from streaming rows for
// the whole deadline and growing worker memory.
const maxReplyBytes = 16 << 20

// apiTransport is one logged-in RouterOS API connection. It lives for one poll.
type apiTransport struct {
	conn   net.Conn
	reader *bufio.Reader
}

// trapError is a command the router refused; its message is the router's.
type trapError struct{ message string }

func (e *trapError) Error() string { return "router refused: " + e.message }

// dialAPI connects and logs in, so a refused login surfaces here as
// ErrUnauthorized. Over api-ssl the certificate is checked before the
// password is written.
func dialAPI(cfg Config) (*apiTransport, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := dialConn(cfg, address)
	if err != nil {
		if errors.Is(err, ErrCertificateChanged) {
			return nil, ErrCertificateChanged
		}
		return nil, fmt.Errorf("connect %s: %w", address, err)
	}
	t := &apiTransport{conn: conn, reader: bufio.NewReader(conn)}
	if err := t.login(cfg.Username, cfg.Password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return t, nil
}

// dialConn picks the wire explicitly. Anything but the two API protocols is
// refused before a socket opens, so a mislabelled router can never fall
// through to cleartext and send its password unencrypted.
func dialConn(cfg Config, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: apiCallTimeout}
	switch cfg.Protocol {
	case ProtocolAPISSL:
		// DialWithDialer applies the dialer's Timeout to the handshake too.
		conn, err := tls.DialWithDialer(dialer, "tcp", address, pinnedTLS(cfg.Fingerprint))
		if err != nil {
			return nil, err
		}
		return conn, nil
	case ProtocolAPI:
		return dialer.Dial("tcp", address)
	default:
		return nil, fmt.Errorf("unknown RouterOS API protocol %q", cfg.Protocol)
	}
}

func (t *apiTransport) login(username, password string) error {
	_, err := t.run("/login", "=name="+username, "=password="+password)
	var trap *trapError
	if errors.As(err, &trap) {
		return ErrUnauthorized
	}
	return err
}

// run sends one command and collects its !re rows until !done. A !trap is
// followed by !done, so it is remembered and returned once the reply ends.
func (t *apiTransport) run(words ...string) ([]map[string]string, error) {
	if err := t.conn.SetDeadline(time.Now().Add(apiCallTimeout)); err != nil {
		return nil, err
	}
	if err := writeSentence(t.conn, words...); err != nil {
		return nil, fmt.Errorf("send %s: %w", words[0], err)
	}
	var rows []map[string]string
	var trap error
	received := 0
	for {
		sentence, err := readSentence(t.reader)
		if err != nil {
			return nil, fmt.Errorf("read %s reply: %w", words[0], err)
		}
		if received += sentenceBytes(sentence); received > maxReplyBytes {
			return nil, fmt.Errorf("%w: %s reply over %d bytes", errBadReply, words[0], maxReplyBytes)
		}
		r, err := parseSentence(sentence)
		if err != nil {
			return nil, err
		}
		switch r.kind {
		case "!re":
			rows = append(rows, r.attrs)
		case "!trap":
			trap = &trapError{message: r.attrs["message"]}
		case "!fatal":
			return nil, fmt.Errorf("router closed the API session: %s", strings.Join(sentence[1:], " "))
		case "!done":
			if trap != nil {
				return nil, trap
			}
			return rows, nil
		}
	}
}

func sentenceBytes(words []string) int {
	total := 0
	for _, w := range words {
		total += len(w)
	}
	return total
}

// List reads a menu with "<path>/print". Its rows carry the same property
// names REST uses, so they decode into the same structs.
func (t *apiTransport) List(path string, out interface{}) error {
	rows, err := t.run(path + "/print")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return decodeRows(rows, out)
}

// Close ends the connection; the router drops the session with it.
func (t *apiTransport) Close() error { return t.conn.Close() }

// decodeRows fills out from rows through the structs' json tags. A single-
// object menu such as /system/resource arrives as one row and fills a struct.
func decodeRows(rows []map[string]string, out interface{}) error {
	var value interface{} = rows
	if reflect.TypeOf(out).Elem().Kind() != reflect.Slice {
		value = map[string]string{}
		if len(rows) > 0 {
			value = rows[0]
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
