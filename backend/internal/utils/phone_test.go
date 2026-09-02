package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhoneAcceptsTheThreeWaysIndonesiansWriteANumber(t *testing.T) {
	for _, raw := range []string{"081234567890", "+6281234567890", "6281234567890", "0812-3456-7890", "0812 3456 7890"} {
		got, err := NormalizePhone(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, "6281234567890", got, raw)
	}
}

func TestNormalizePhoneRejectsWhatCannotBeANumber(t *testing.T) {
	for _, raw := range []string{"", "  ", "abcdefg", "0812", "0", "62"} {
		_, err := NormalizePhone(raw)
		assert.Error(t, err, raw)
	}
}
