package firebaseapp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A checkout with no Firebase project must still start. Every caller reads a
// nil app as "Firebase is not configured", so this may not be an error.
func TestNewReturnsNothingWhenUnconfigured(t *testing.T) {
	app, err := New(context.Background(), "")

	require.NoError(t, err)
	assert.Nil(t, app)
}

// A malformed key is a deployment mistake and has to be loud, not silently
// equivalent to having no Firebase at all.
func TestNewRejectsAKeyThatIsNotBase64(t *testing.T) {
	_, err := New(context.Background(), "not base64 at all!!")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode FIREBASE_SERVICE_ACCOUNT_JSON_B64")
}
