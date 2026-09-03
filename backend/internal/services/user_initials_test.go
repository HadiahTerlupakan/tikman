package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A username is validated alphanum on the way in, so one word is what this
// almost always gets, and one letter is thin for a mark a customer reads at
// the end of every reply. The separator cases still matter: rows seeded before
// that validation, and initials an admin retypes by hand.
func TestDeriveInitialsGivesTwoLettersWhereverItCan(t *testing.T) {
	cases := map[string]string{
		"admin":               "AD",
		"Budi Santoso":        "BS",
		"budi.santoso":        "BS",
		"budi_santoso":        "BS",
		"budi-santoso":        "BS",
		"budi santoso wijaya": "BSW",
		"a":                   "A",
		"":                    "",
		"  ":                  "",
	}
	for username, want := range cases {
		assert.Equal(t, want, deriveInitials(username), username)
	}
}

// What an admin types is a display mark, not an identifier: "bs" and "BS" are
// the same intent, and storing them differently would show one CS under two
// signatures depending on how they were typed.
func TestNormalizeInitialsUppercasesAndTrims(t *testing.T) {
	cases := map[string]string{
		"bs":    "BS",
		" bs ":  "BS",
		"BS":    "BS",
		"":      "",
		"   ":   "",
		"budi2": "BUDI2",
	}
	for typed, want := range cases {
		assert.Equal(t, want, normalizeInitials(typed), typed)
	}
}
