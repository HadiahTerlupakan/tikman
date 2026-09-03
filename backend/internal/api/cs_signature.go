package api

import "strings"

// signatureSeparator is the blank line WhatsApp Business puts between a reply
// and the name of whoever wrote it. Keeping the same shape matters: customers
// already read "~Name" on a trailing line as a person, not as part of the
// sentence above it.
const signatureSeparator = "\n\n~"

// signReply puts the sender's stored initials on the end of a reply, the way
// a CS signs a shared number so the customer knows who answered.
//
// It runs on the way into the outbox rather than in the composer, so what a CS
// sees in the thread is exactly what the customer received. It is also
// idempotent: "Coba lagi" re-sends the stored body, which already carries the
// signature, and signing that again would stack them.
func signReply(body, mark string) string {
	if mark == "" {
		return body
	}
	signature := signatureSeparator + mark
	if strings.HasSuffix(body, signature) {
		return body
	}
	return body + signature
}
