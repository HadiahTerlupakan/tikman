package utils

import (
	"fmt"
	"strings"
)

// minPhoneDigits is the shortest Indonesian mobile number worth accepting:
// country code plus a subscriber number. Anything shorter is a typo, and
// storing it would attach a chat to the wrong subscriber.
const minPhoneDigits = 10

// NormalizePhone rewrites the three ways an Indonesian number gets typed —
// 0812…, +62812…, 62812… — into the single 62812… form WhatsApp itself uses,
// so a chat can be matched to an ONT whichever way the number was recorded.
func NormalizePhone(raw string) (string, error) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	n := digits.String()
	switch {
	case strings.HasPrefix(n, "0"):
		n = "62" + strings.TrimPrefix(n, "0")
	case strings.HasPrefix(n, "62"):
	default:
		return "", fmt.Errorf("nomor %q tidak dikenali sebagai nomor Indonesia", raw)
	}

	if len(n) < minPhoneDigits {
		return "", fmt.Errorf("nomor %q terlalu pendek", raw)
	}
	return n, nil
}
