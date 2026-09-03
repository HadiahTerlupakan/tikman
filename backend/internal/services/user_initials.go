package services

import (
	"strings"
	"unicode"
)

// deriveInitials reduces a username to a display mark, run when a user is
// created and whenever an admin clears the initials field to fall back on it.
// Two letters wherever the username can give them: one per word for a
// multi-word name, otherwise the first two letters of the one word it has.
// Usernames are validated alphanum on the way in, so the separator-splitting
// here mainly serves rows seeded before that validation existed.
func deriveInitials(username string) string {
	fields := strings.FieldsFunc(username, func(r rune) bool {
		return unicode.IsSpace(r) || r == '.' || r == '_' || r == '-'
	})

	if len(fields) > 1 {
		var out strings.Builder
		for _, field := range fields {
			for _, r := range field {
				out.WriteRune(unicode.ToUpper(r))
				break
			}
		}
		return out.String()
	}

	if len(fields) == 1 {
		return normalizeInitials(firstNRunes(fields[0], 2))
	}

	return ""
}

func firstNRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}

// normalizeInitials uppercases and trims what an admin types, so "bs" and
// "BS" store as the same mark instead of showing one CS under two signatures
// depending on how it was typed.
func normalizeInitials(typed string) string {
	return strings.ToUpper(strings.TrimSpace(typed))
}
