package mikrotik

import (
	"bufio"
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reader(b []byte) *bufio.Reader { return bufio.NewReader(bytes.NewReader(b)) }

// Each width change of the length prefix is where an off-by-one would corrupt
// every following word, so both sides of every boundary are pinned.
func TestLengthEncodingAtEveryBoundary(t *testing.T) {
	cases := []struct {
		n    int
		want []byte
	}{
		{0, []byte{0x00}},
		{0x7F, []byte{0x7F}},
		{0x80, []byte{0x80, 0x80}},
		{0x3FFF, []byte{0xBF, 0xFF}},
		{0x4000, []byte{0xC0, 0x40, 0x00}},
		{0x1FFFFF, []byte{0xDF, 0xFF, 0xFF}},
		{0x200000, []byte{0xE0, 0x20, 0x00, 0x00}},
		{0xFFFFFFF, []byte{0xEF, 0xFF, 0xFF, 0xFF}},
		{0x10000000, []byte{0xF0, 0x10, 0x00, 0x00, 0x00}},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, encodeLength(c.n), "encode %#x", c.n)
		got, err := readLength(reader(c.want))
		require.NoError(t, err, "decode %#x", c.n)
		assert.Equal(t, c.n, got, "decode %#x", c.n)
	}
}

func TestSentenceRoundTrip(t *testing.T) {
	long := string(bytes.Repeat([]byte("x"), 200))
	var buf bytes.Buffer
	require.NoError(t, writeSentence(&buf, "/login", "=name=tikman", "=password="+long))

	words, err := readSentence(reader(buf.Bytes()))

	require.NoError(t, err)
	assert.Equal(t, []string{"/login", "=name=tikman", "=password=" + long}, words)
}

// A control byte (0xF8-0xFF) or an absurd length means the stream is not the
// RouterOS API — for instance a TLS port answering a plain connection.
func TestReadSentenceRejectsWhatIsNotTheAPI(t *testing.T) {
	_, err := readSentence(reader([]byte{0xF8}))
	assert.True(t, errors.Is(err, errBadReply), "control byte: %v", err)

	_, err = readSentence(reader(encodeLength(maxWordLength + 1)))
	assert.True(t, errors.Is(err, errBadReply), "oversized word: %v", err)
}

func TestParseSentenceKeepsEqualsInsideAValue(t *testing.T) {
	r, err := parseSentence([]string{"!re", "=comment=uplink=transit", "=name=vlan-100", ".tag=7"})

	require.NoError(t, err)
	assert.Equal(t, "!re", r.kind)
	assert.Equal(t, "uplink=transit", r.attrs["comment"])
	assert.Equal(t, "vlan-100", r.attrs["name"])
	assert.NotContains(t, r.attrs, ".tag")
}

func TestParseSentenceRejectsAnEmptySentence(t *testing.T) {
	_, err := parseSentence(nil)
	assert.True(t, errors.Is(err, errBadReply))
}

// Each word is under maxWordLength, so only the sentence total can reject
// this; a peer without a pinned certificate could otherwise grow a sentence
// without bound inside the read deadline.
func TestReadSentenceRejectsAWholeSentenceOverTheLimit(t *testing.T) {
	word := string(bytes.Repeat([]byte("x"), 300<<10))
	var buf bytes.Buffer
	require.NoError(t, writeSentence(&buf, word, word, word, word))

	_, err := readSentence(reader(buf.Bytes()))

	assert.True(t, errors.Is(err, errBadReply), "sentence total: %v", err)
}
