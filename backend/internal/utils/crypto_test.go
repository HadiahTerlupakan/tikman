package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
