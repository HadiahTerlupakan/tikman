package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	password := "test-password-123"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
}

func TestComparePassword_Valid(t *testing.T) {
	password := "test-password-123"
	hash, err := HashPassword(password)
	require.NoError(t, err)

	err = ComparePassword(hash, password)
	assert.NoError(t, err)
}

func TestComparePassword_Invalid(t *testing.T) {
	password := "test-password-123"
	hash, err := HashPassword(password)
	require.NoError(t, err)

	err = ComparePassword(hash, "wrong-password")
	assert.Error(t, err)
}

func TestEncryptDecrypt(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef" // 32 bytes
	plaintext := "my-secret-password"

	encrypted, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := Decrypt(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	key := "short-key"
	plaintext := "secret"

	_, err := Encrypt(plaintext, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")
}

// A dummy hash cheaper than bcryptCost would make a login for an unknown
// username return measurably sooner than a wrong password for a real one.
func TestDummyPasswordHashCostsTheSameAsARealHash(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	require.NoError(t, err)
	assert.Equal(t, bcryptCost, cost)
}
