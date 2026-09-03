package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignReplyPutsTheMarkOnItsOwnLine(t *testing.T) {
	assert.Equal(t, "Sudah kami cek ya.\n\n~BS",
		signReply("Sudah kami cek ya.", "BS"))
}

// "Coba lagi" re-sends the stored body, which already carries a signature.
// Signing it again would stack them, and every retry would grow the message.
func TestSignReplyDoesNotStackOnARetry(t *testing.T) {
	once := signReply("Sudah kami cek ya.", "BS")
	assert.Equal(t, once, signReply(once, "BS"))
}

// A different CS taking over does sign their own reply, even when the body they
// resend was signed by someone else — the customer should see who sent it now.
func TestSignReplyStillSignsWhenAnotherCSSignedBefore(t *testing.T) {
	fromBudi := signReply("Sudah kami cek ya.", "BS")
	assert.Equal(t, fromBudi+"\n\n~RA", signReply(fromBudi, "RA"))
}

// A user row seeded before initials existed carries "" rather than a mark;
// appending a bare "~" for it would read as a typo at the end of every reply.
func TestSignReplyLeavesTheBodyAloneWithNoMark(t *testing.T) {
	assert.Equal(t, "Sudah kami cek ya.", signReply("Sudah kami cek ya.", ""))
}
