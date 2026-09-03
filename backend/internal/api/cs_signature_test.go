package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialsTakeTheFirstLetterOfEachPart(t *testing.T) {
	cases := map[string]string{
		"Budi Santoso": "BS",
		"budi.santoso": "BS",
		"budi_santoso": "BS",
		"admin":        "A",
		"":             "",
	}
	for username, want := range cases {
		assert.Equal(t, want, initials(username), username)
	}
}

func TestSignReplyPutsTheInitialsOnTheirOwnLine(t *testing.T) {
	assert.Equal(t, "Sudah kami cek ya.\n\n~BS",
		signReply("Sudah kami cek ya.", "Budi Santoso"))
}

// "Coba lagi" re-sends the stored body, which already carries a signature.
// Signing it again would stack them, and every retry would grow the message.
func TestSignReplyDoesNotStackOnARetry(t *testing.T) {
	once := signReply("Sudah kami cek ya.", "Budi Santoso")
	assert.Equal(t, once, signReply(once, "Budi Santoso"))
}

// A different CS taking over does sign their own reply, even when the body they
// resend was signed by someone else — the customer should see who sent it now.
func TestSignReplyStillSignsWhenAnotherCSSignedBefore(t *testing.T) {
	fromBudi := signReply("Sudah kami cek ya.", "Budi Santoso")
	assert.Equal(t, fromBudi+"\n\n~RA", signReply(fromBudi, "Rina Astuti"))
}

// A username with nothing to take an initial from must not append a bare "~",
// which would read as a typo at the end of every reply.
func TestSignReplyLeavesTheBodyAloneWhenThereIsNoInitial(t *testing.T) {
	assert.Equal(t, "Sudah kami cek ya.", signReply("Sudah kami cek ya.", "  "))
}
