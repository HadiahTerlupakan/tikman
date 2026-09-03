package api

import (
	"strings"
	"unicode"
)

// signatureSeparator is the blank line WhatsApp Business puts between a reply
// and the name of whoever wrote it. Keeping the same shape matters: customers
// already read "~Name" on a trailing line as a person, not as part of the
// sentence above it.
const signatureSeparator = "\n\n~"

// initials reduces a username to what goes on the end of a reply. A single-word
// name yields a single letter, which is thin — but it is what a one-word
// username has to give, and inventing more would be inventing.
func initials(username string) string {
	var out strings.Builder
	for _, field := range strings.FieldsFunc(username, func(r rune) bool {
		return unicode.IsSpace(r) || r == '.' || r == '_' || r == '-'
	}) {
		for _, r := range field {
			out.WriteRune(unicode.ToUpper(r))
			break
		}
	}
	return out.String()
}

// signReply puts the sender's initials on the end of a reply, the way a CS
// signs a shared number so the customer knows who answered.
//
// It runs on the way into the outbox rather than in the composer, so what a CS
// sees in the thread is exactly what the customer received. It is also
// idempotent: "Coba lagi" re-sends the stored body, which already carries the
// signature, and signing that again would stack them.
func signReply(body, username string) string {
	mark := initials(username)
	if mark == "" {
		return body
	}
	signature := signatureSeparator + mark
	if strings.HasSuffix(body, signature) {
		return body
	}
	return body + signature
}
