package mikrotik

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxWordLength bounds the bytes of one word and of one whole sentence. A
// sentence is capped as well, since a word-count bound alone would let a peer
// grow memory without limit within the read deadline; anything this large means
// the stream is not the API.
const maxWordLength = 1 << 20

// errBadReply means the bytes read cannot be a RouterOS API sentence.
var errBadReply = errors.New("malformed RouterOS API reply")

// encodeLength writes a word length the way the RouterOS API prefixes it: the
// high bits of the first byte say how many bytes follow.
func encodeLength(n int) []byte {
	switch {
	case n < 0x80:
		return []byte{byte(n)}
	case n < 0x4000:
		v := n | 0x8000
		return []byte{byte(v >> 8), byte(v)}
	case n < 0x200000:
		v := n | 0xC00000
		return []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case n < 0x10000000:
		v := uint32(n) | 0xE0000000
		return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	default:
		return []byte{0xF0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
}

func readLength(r *bufio.Reader) (int, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	var extra, n int
	switch {
	case first&0x80 == 0x00:
		return int(first), nil
	case first&0xC0 == 0x80:
		extra, n = 1, int(first&0x3F)
	case first&0xE0 == 0xC0:
		extra, n = 2, int(first&0x1F)
	case first&0xF0 == 0xE0:
		extra, n = 3, int(first&0x0F)
	case first == 0xF0:
		extra = 4
	default:
		return 0, fmt.Errorf("%w: control byte %#x", errBadReply, first)
	}
	for i := 0; i < extra; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n = n<<8 | int(b)
	}
	return n, nil
}

// writeSentence builds the sentence, ended by an empty word, in one buffer and
// sends it with a single Write call.
func writeSentence(w io.Writer, words ...string) error {
	var buf bytes.Buffer
	for _, word := range words {
		buf.Write(encodeLength(len(word)))
		buf.WriteString(word)
	}
	buf.WriteByte(0)
	_, err := w.Write(buf.Bytes())
	return err
}

func readSentence(r *bufio.Reader) ([]string, error) {
	var words []string
	total := 0
	for {
		n, err := readLength(r)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return words, nil
		}
		if n > maxWordLength-total {
			return nil, fmt.Errorf("%w: sentence over %d bytes", errBadReply, maxWordLength)
		}
		total += n
		word := make([]byte, n)
		if _, err := io.ReadFull(r, word); err != nil {
			return nil, err
		}
		words = append(words, string(word))
	}
}

// reply is one sentence from the router: its type ("!re", "!done", "!trap",
// "!fatal", "!empty") and its "=key=value" attributes.
type reply struct {
	kind  string
	attrs map[string]string
}

// parseSentence splits attributes on the second "=" only, since values such
// as comments may contain "=" themselves. Words that are not attributes (the
// ".tag" of a tagged command) carry nothing a read needs.
func parseSentence(words []string) (reply, error) {
	if len(words) == 0 {
		return reply{}, fmt.Errorf("%w: empty sentence", errBadReply)
	}
	r := reply{kind: words[0], attrs: map[string]string{}}
	for _, word := range words[1:] {
		if !strings.HasPrefix(word, "=") {
			continue
		}
		key, value, _ := strings.Cut(word[1:], "=")
		r.attrs[key] = value
	}
	return r, nil
}
